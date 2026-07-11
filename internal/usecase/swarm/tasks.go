package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/blockchain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/cache"
	"github.com/Elysian-Rebirth/backend-go/internal/repository/postgres"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

const (
	TypeCommitSwarmToBlockchain = "swarm:commit_blockchain"
)

type CommitBlockchainPayload struct {
	TaskID        string `json:"task_id"`
	RationaleHash string `json:"rationale_hash"`
	ConsensusHash string `json:"consensus_hash"`
}

func NewCommitSwarmToBlockchainTask(taskID, rationaleHash, consensusHash string) (*asynq.Task, error) {
	payload, err := json.Marshal(CommitBlockchainPayload{
		TaskID:        taskID,
		RationaleHash: rationaleHash,
		ConsensusHash: consensusHash,
	})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(
		TypeCommitSwarmToBlockchain,
		payload,
		asynq.MaxRetry(5),
		asynq.Queue("default"),
	), nil
}

type SwarmTaskHandler struct {
	swarmRepo         *postgres.SwarmRepository
	blockchainService *blockchain.AuditTrailService
	redis             cache.Cache
}

func NewSwarmTaskHandler(swarmRepo *postgres.SwarmRepository, bcService *blockchain.AuditTrailService, redis cache.Cache) *SwarmTaskHandler {
	handler := &SwarmTaskHandler{
		swarmRepo:         swarmRepo,
		blockchainService: bcService,
		redis:             redis,
	}
	if bcService != nil && redis != nil {
		go handler.startBatchCommitter()
	}
	return handler
}

func (h *SwarmTaskHandler) HandleCommitSwarmToBlockchain(ctx context.Context, t *asynq.Task) error {
	var payload CommitBlockchainPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	redisCache, ok := h.redis.(*cache.RedisCache)
	if !ok {
		return fmt.Errorf("redis cache not initialized")
	}

	payloadBytes, _ := json.Marshal(payload)
	err := redisCache.GetClient().RPush(ctx, "blockchain:pending_commits", payloadBytes).Err()
	if err != nil {
		return fmt.Errorf("failed to queue for batch commit: %w", err)
	}

	log.Printf("[Swarm-Worker] Queued task %s for batch blockchain commit", payload.TaskID)
	return nil
}

func (h *SwarmTaskHandler) startBatchCommitter() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	ctx := context.Background()
	redisCache, ok := h.redis.(*cache.RedisCache)
	if !ok {
		return
	}
	rClient := redisCache.GetClient()

	for range ticker.C {
		lenCmd := rClient.LLen(ctx, "blockchain:pending_commits")
		pendingLen, err := lenCmd.Result()
		if err != nil || pendingLen == 0 {
			continue
		}

		var batchPayloads []CommitBlockchainPayload
		var poppedJSONs []string
		for i := int64(0); i < pendingLen && i < 10; i++ {
			val, err := rClient.LPop(ctx, "blockchain:pending_commits").Result()
			if err != nil {
				if err == redis.Nil {
					break
				}
				log.Printf("[Swarm-Batch-Committer] LPop error: %v", err)
				break
			}
			var p CommitBlockchainPayload
			if err := json.Unmarshal([]byte(val), &p); err == nil {
				batchPayloads = append(batchPayloads, p)
				poppedJSONs = append(poppedJSONs, val)
			}
		}

		if len(batchPayloads) == 0 {
			continue
		}

		log.Printf("[Swarm-Batch-Committer] Found %d pending commits. Processing batch...", len(batchPayloads))

		var lastTxHash string
		var commitErr error
		for i, p := range batchPayloads {
			txHash, err := h.blockchainService.InsertLog(ctx, p.TaskID, p.RationaleHash, p.ConsensusHash)
			if err != nil {
				commitErr = err
				log.Printf("[Swarm-Batch-Committer] ❌ Individual commit failed for task %s: %v. Re-queueing task to avoid loss.", p.TaskID, err)
				_ = rClient.RPush(ctx, "blockchain:pending_commits", poppedJSONs[i]).Err()
			} else {
				lastTxHash = txHash
				log.Printf("[Swarm-Batch-Committer] 📨 Submitted tx %s for task %s. Waiting for confirmation...", txHash, p.TaskID)
				h.updateBlockchainStatus(ctx, p.TaskID, txHash, "PENDING_CONFIRMATION")
				
				go func(tx string, payload CommitBlockchainPayload) {
					bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
					defer cancel()
					
					receipt, err := h.blockchainService.WaitForConfirmation(bgCtx, tx, 90*time.Second)
					if err != nil {
						log.Printf("[Swarm-Batch-Worker] ⚠️ Wait for confirmation timed out for task %s, tx: %s", payload.TaskID, tx)
						h.updateBlockchainStatus(bgCtx, payload.TaskID, tx, "FAILED")
						return
					}
					
					if receipt.Status == 1 {
						log.Printf("[Swarm-Batch-Worker] ✅ Tx confirmed for task %s, tx: %s in block %d", payload.TaskID, tx, receipt.BlockNumber)
						h.updateBlockchainStatus(bgCtx, payload.TaskID, tx, "VERIFIED")
					} else {
						log.Printf("[Swarm-Batch-Worker] ❌ Tx execution failed on-chain for task %s, tx: %s", payload.TaskID, tx)
						h.updateBlockchainStatus(bgCtx, payload.TaskID, tx, "FAILED")
					}
				}(txHash, p)
			}
		}
		if commitErr != nil && lastTxHash == "" {
			continue
		}
	}
}

func (h *SwarmTaskHandler) updateBlockchainStatus(ctx context.Context, taskID, txHash, status string) {
	task, err := h.swarmRepo.GetByID(ctx, taskID)
	if err != nil {
		log.Printf("[Swarm-Worker] Failed to find task %s: %v", taskID, err)
		return
	}

	if txHash != "" {
		task.BlockchainTx = txHash
	}
	task.BlockchainStat = status
	task.UpdatedAt = time.Now()

	if err := h.swarmRepo.Update(ctx, task); err != nil {
		log.Printf("[Swarm-Worker] Failed to update task status in PostgreSQL: %v", err)
	}

	h.publishBlockchainUpdate(ctx, taskID, txHash, status)
}

func (h *SwarmTaskHandler) publishBlockchainUpdate(ctx context.Context, taskID, txHash, status string) {
	task, err := h.swarmRepo.GetByID(ctx, taskID)
	if err != nil {
		return
	}

	if h.redis == nil {
		return
	}

	if redisCache, ok := h.redis.(*cache.RedisCache); ok {
		var results interface{}
		if len(task.Results) > 0 {
			_ = json.Unmarshal(task.Results, &results)
		}
		ssePayload := map[string]interface{}{
			"task_id":    task.ID,
			"status":     task.Status,
			"results":    results,
			"blockchain": map[string]interface{}{
				"tx_hash": txHash,
				"network": task.BlockchainNet,
				"status":  status,
			},
			"nft_token_id":  task.NFTTokenID,
			"ipfs_cid":      task.IPFSCID,
			"nft_tx_hash":   task.NFTTxHash,
			"timestamp":     time.Now().UnixNano() / int64(time.Millisecond),
		}
		sseBytes, _ := json.Marshal(ssePayload)
		redisCache.GetClient().Publish(ctx, "swarm:events", sseBytes)
	}
}
