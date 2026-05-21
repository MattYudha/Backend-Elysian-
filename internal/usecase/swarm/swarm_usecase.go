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
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/mq"
	"github.com/Elysian-Rebirth/backend-go/internal/repository/postgres"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type SwarmUsecase struct {
	swarmRepo         *postgres.SwarmRepository
	redis             cache.Cache
	blockchainService *blockchain.AuditTrailService
	mqClient          mq.TaskQueue
}

func NewSwarmUsecase(swarmRepo *postgres.SwarmRepository, redis cache.Cache, bcService *blockchain.AuditTrailService, mqClient mq.TaskQueue) *SwarmUsecase {
	return &SwarmUsecase{
		swarmRepo:         swarmRepo,
		redis:             redis,
		blockchainService: bcService,
		mqClient:          mqClient,
	}
}

func (u *SwarmUsecase) TriggerSwarm(ctx context.Context, documentID string, items []map[string]interface{}, tenantIDStr string, userIDStr string) (*domain.SwarmTask, error) {
	var finalDocID string = documentID
	if _, err := uuid.Parse(documentID); err != nil {
		// Document ID is not a valid UUID (e.g. "draft-1")
		tenantUUID, err := uuid.Parse(tenantIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid tenant id parameter: %w", err)
		}
		userUUID, err := uuid.Parse(userIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid user id parameter: %w", err)
		}

		// Generate deterministic draft UUID based on the tenant ID and the original identifier
		draftUUID := uuid.NewSHA1(uuid.NameSpaceDNS, []byte("draft-"+tenantIDStr+"-"+documentID))
		finalDocID = draftUUID.String()

		// Verify or create the draft document in the database to satisfy the foreign key constraint
		db := u.swarmRepo.GetDB()
		var count int64
		err = db.WithContext(ctx).Table("documents").Where("id = ?", draftUUID).Count(&count).Error
		if err != nil {
			return nil, fmt.Errorf("failed to check existing draft document: %w", err)
		}

		if count == 0 {
			draftDoc := &domain.Document{
				ID:            draftUUID,
				TenantID:      tenantUUID,
				UserID:        userUUID,
				Title:         "Draft Document (" + documentID + ")",
				Category:      "general",
				Status:        "draft",
				CreatedAt:     time.Now(),
				LastUpdatedAt: time.Now(),
			}
			err = db.WithContext(ctx).Table("documents").Create(draftDoc).Error
			if err != nil {
				return nil, fmt.Errorf("failed to auto-create draft document record: %w", err)
			}
		}
	}

	// 1. Create Task in DB
	task := &domain.SwarmTask{
		DocumentID: finalDocID,
		Status:     "PENDING",
	}

	if err := u.swarmRepo.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create swarm task: %w", err)
	}

	// 2. Prepare Payload
	payload := domain.SwarmPayload{
		TaskID:       task.ID,
		DocumentID:   finalDocID,
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

	// Step 5 — Push hash to blockchain asynchronously via Asynq queue
	if u.blockchainService != nil && task.RationaleHash != "" && task.ConsensusHash != "" {
		asynqTask, err := NewCommitSwarmToBlockchainTask(task.ID, task.RationaleHash, task.ConsensusHash)
		if err != nil {
			log.Printf("[Swarm] Failed to create blockchain commit task for task %s: %v", task.ID, err)
		} else {
			if _, err := u.mqClient.EnqueueTask(asynqTask); err != nil {
				log.Printf("[Swarm] Failed to enqueue blockchain commit task for task %s: %v. Falling back to local state.", task.ID, err)
				u.updateBlockchainStatus(ctx, task.ID, "", "PENDING_COMMIT")
			} else {
				log.Printf("[Swarm] Successfully enqueued blockchain commit task for task %s", task.ID)
			}
		}
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

func (u *SwarmUsecase) ListSwarmTasks(ctx context.Context, tenantID string, limit, offset int) ([]*domain.SwarmTask, int64, error) {
	return u.swarmRepo.ListByTenant(ctx, tenantID, limit, offset)
}

