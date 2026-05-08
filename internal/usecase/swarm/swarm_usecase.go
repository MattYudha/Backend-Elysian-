package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/cache"
	"github.com/Elysian-Rebirth/backend-go/internal/repository/postgres"
	"gorm.io/datatypes"
)

type SwarmUsecase struct {
	swarmRepo *postgres.SwarmRepository
	redis     cache.Cache
}

func NewSwarmUsecase(swarmRepo *postgres.SwarmRepository, redis cache.Cache) *SwarmUsecase {
	return &SwarmUsecase{
		swarmRepo: swarmRepo,
		redis:     redis,
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
		// Assuming the server is running on localhost:7777 for hackathon
		WebhookURL: "http://host.docker.internal:7777/api/internal/swarm/callback",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// 3. LPUSH to Redis
	// We need to access the underlying redis client. The cache.Cache interface might not have LPUSH.
	// But we can use the Set method if LPUSH is not available, or we might need to cast.
	// Wait, the cache interface in Elysian is basic. I'll use standard cache methods or cast it.
	// Let's assume there's a way to publish or we just cast to RedisCache.
	
	// For now, let's use the underlying redis client.
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
	
	resultsBytes, _ := json.Marshal(callback.Results)
	task.Results = datatypes.JSON(resultsBytes)
	task.UpdatedAt = time.Now()

	if err := u.swarmRepo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	// In a real app, we'd also publish an event to Redis PubSub here so SSE handlers on any node can pick it up.
	// For this hackathon, we'll use a local Go channel or Redis PubSub in the handler.
	if redisCache, ok := u.redis.(*cache.RedisCache); ok {
		redisCache.GetClient().Publish(ctx, "swarm:events", resultsBytes)
	}

	return nil
}
