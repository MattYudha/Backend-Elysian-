package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/blockchain"
	"github.com/Elysian-Rebirth/backend-go/internal/repository/postgres"
	"github.com/hibiken/asynq"
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
}

func NewSwarmTaskHandler(swarmRepo *postgres.SwarmRepository, bcService *blockchain.AuditTrailService) *SwarmTaskHandler {
	return &SwarmTaskHandler{
		swarmRepo:         swarmRepo,
		blockchainService: bcService,
	}
}

func (h *SwarmTaskHandler) HandleCommitSwarmToBlockchain(ctx context.Context, t *asynq.Task) error {
	var payload CommitBlockchainPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	if h.blockchainService == nil {
		log.Printf("[Swarm-Worker] Blockchain service not initialized. Skipping commit for task %s", payload.TaskID)
		return nil
	}

	log.Printf("[Swarm-Worker] ▶ Committing hashes to Sepolia for Swarm Task %s (Rationale: %s, Consensus: %s)",
		payload.TaskID, payload.RationaleHash, payload.ConsensusHash)

	// Emit block transaction
	txHash, err := h.blockchainService.InsertLog(ctx, payload.TaskID, payload.RationaleHash, payload.ConsensusHash)
	if err != nil {
		log.Printf("[Swarm-Worker] ❌ insertLog failed for task %s: %v", payload.TaskID, err)
		h.updateBlockchainStatus(ctx, payload.TaskID, "", "FAILED")
		return fmt.Errorf("insertLog failed: %w", err)
	}

	log.Printf("[Swarm-Worker] 📨 insertLog submitted tx %s for task %s. Waiting for confirmation...", txHash, payload.TaskID)
	h.updateBlockchainStatus(ctx, payload.TaskID, txHash, "PENDING_CONFIRMATION")

	// Wait for blockchain confirmation
	receipt, err := h.blockchainService.WaitForConfirmation(ctx, txHash, 90*time.Second)
	if err != nil {
		log.Printf("[Swarm-Worker] ⚠️ Wait for confirmation timed out for task %s, tx: %s", payload.TaskID, txHash)
		return fmt.Errorf("wait for confirmation timed out: %w", err)
	}

	if receipt.Status == 1 {
		log.Printf("[Swarm-Worker] ✅ Tx confirmed for task %s in block %d", payload.TaskID, receipt.BlockNumber)
		h.updateBlockchainStatus(ctx, payload.TaskID, txHash, "VERIFIED")
	} else {
		log.Printf("[Swarm-Worker] ❌ Tx execution failed on-chain for task %s", payload.TaskID)
		h.updateBlockchainStatus(ctx, payload.TaskID, txHash, "FAILED")
		return fmt.Errorf("transaction execution failed on chain")
	}

	return nil
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
}
