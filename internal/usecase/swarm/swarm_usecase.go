package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/blockchain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/cache"
	"github.com/Elysian-Rebirth/backend-go/internal/repository/postgres"
	"gorm.io/datatypes"
)

type SwarmUsecase struct {
	swarmRepo         *postgres.SwarmRepository
	redis             cache.Cache
	blockchainService *blockchain.AuditTrailService
}

func NewSwarmUsecase(swarmRepo *postgres.SwarmRepository, redis cache.Cache, bcService *blockchain.AuditTrailService) *SwarmUsecase {
	return &SwarmUsecase{
		swarmRepo:         swarmRepo,
		redis:             redis,
		blockchainService: bcService,
	}
}

func (u *SwarmUsecase) TriggerSwarm(ctx context.Context, documentID string, items []map[string]interface{}) (*domain.SwarmTask, error) {
	// 1. Create Task in DB
	task := &domain.SwarmTask{
		DocumentID: documentID,
		Status:     "PENDING",
	}

	if err := u.swarmRepo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create swarm task: %w", err)
	}

	// 2. Prepare Payload
	payload := domain.SwarmPayload{
		TaskID:       task.ID,
		DocumentID:   documentID,
		DocumentType: "RAPBD",
		Items:        items,
		WebhookURL:   "http://localhost:7777/api/v1/swarm/callback",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// 3. LPUSH to Redis
	if redisCache, ok := u.redis.(*cache.RedisCache); ok {
		err = redisCache.GetClient().LPush(ctx, "swarm:tasks", payloadBytes).Err()
		if err != nil {
			return nil, fmt.Errorf("failed to publish to redis: %w", err)
		}
	} else {
		return nil, fmt.Errorf("cache is not redis")
	}

	return task, nil
}

func (u *SwarmUsecase) HandleCallback(ctx context.Context, callback domain.SwarmCallback) error {
	task, err := u.swarmRepo.GetByID(ctx, callback.TaskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	task.Status = callback.Status
	task.Summary = callback.Summary
	task.RationaleHash = callback.Hashes.RationaleHash
	task.ConsensusHash = callback.Hashes.ConsensusHash
	task.BlockchainNet = callback.Blockchain.Network
	task.BlockchainStat = callback.Blockchain.Status

	resultsBytes, _ := json.Marshal(callback.Results)
	task.Results = datatypes.JSON(resultsBytes)
	task.UpdatedAt = time.Now()

	if err := u.swarmRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	// Publish to Redis PubSub for SSE streaming
	if redisCache, ok := u.redis.(*cache.RedisCache); ok {
		redisCache.GetClient().Publish(ctx, "swarm:events", resultsBytes)
	}

	// Step 5 — Push hash to blockchain (if configured)
	if u.blockchainService != nil && task.RationaleHash != "" && task.ConsensusHash != "" {
		go func() {
			// Use background context with timeout
			bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()

			txHash, err := u.blockchainService.InsertLog(bgCtx, task.ID, task.RationaleHash, task.ConsensusHash)
			if err != nil {
				log.Printf("[Blockchain] insertLog failed for task %s: %v", task.ID, err)
				// Update status to FAILED
				u.updateBlockchainStatus(bgCtx, task.ID, "", "FAILED")
				return
			}

			log.Printf("[Blockchain] insertLog tx submitted: %s for task %s", txHash, task.ID)

			// Wait for confirmation
			receipt, err := u.blockchainService.WaitForConfirmation(bgCtx, txHash, 90*time.Second)
			if err != nil {
				log.Printf("[Blockchain] confirmation timeout for task %s: %v", task.ID, err)
				u.updateBlockchainStatus(bgCtx, task.ID, txHash, "PENDING_CONFIRMATION")
				return
			}

			if receipt.Status == 1 {
				log.Printf("[Blockchain] tx confirmed for task %s, block: %d", task.ID, receipt.BlockNumber)
				u.updateBlockchainStatus(bgCtx, task.ID, txHash, "VERIFIED")
			} else {
				log.Printf("[Blockchain] tx failed for task %s", task.ID)
				u.updateBlockchainStatus(bgCtx, task.ID, txHash, "FAILED")
			}
		}()
	}

	return nil
}

func (u *SwarmUsecase) updateBlockchainStatus(ctx context.Context, taskID, txHash, status string) {
	task, err := u.swarmRepo.GetByID(ctx, taskID)
	if err != nil {
		log.Printf("[Blockchain] failed to get task %s for status update: %v", taskID, err)
		return
	}

	if txHash != "" {
		task.BlockchainTx = txHash
	}
	task.BlockchainStat = status
	task.UpdatedAt = time.Now()

	if err := u.swarmRepo.Update(ctx, task); err != nil {
		log.Printf("[Blockchain] failed to update task %s status: %v", taskID, err)
	}
}

func (u *SwarmUsecase) GetSwarmTask(ctx context.Context, id string) (*domain.SwarmTask, error) {
	return u.swarmRepo.GetByID(ctx, id)
}

