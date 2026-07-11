package action_center

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/mq"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

type ActionCenterUseCase interface {
	ListActionItems(ctx context.Context, tenantID string, status string) ([]*domain.ActionItem, error)
	ResolveActionItem(ctx context.Context, itemID string, resolvedBy string, justification string) error
}

type actionCenterUseCase struct {
	actionRepo  domain.ActionItemRepository
	asynqClient mq.TaskQueue
}

func NewActionCenterUseCase(actionRepo domain.ActionItemRepository, asynqClient mq.TaskQueue) ActionCenterUseCase {
	return &actionCenterUseCase{
		actionRepo:  actionRepo,
		asynqClient: asynqClient,
	}
}

func (u *actionCenterUseCase) ListActionItems(ctx context.Context, tenantID string, status string) ([]*domain.ActionItem, error) {
	tID, err := uuid.Parse(tenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid tenant id: %w", err)
	}
	return u.actionRepo.List(ctx, tID, status)
}

func (u *actionCenterUseCase) ResolveActionItem(ctx context.Context, itemID string, resolvedBy string, justification string) error {
	id, err := uuid.Parse(itemID)
	if err != nil {
		return fmt.Errorf("invalid action item id: %w", err)
	}

	item, err := u.actionRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("action item not found: %w", err)
	}

	userUUID, err := uuid.Parse(resolvedBy)
	if err != nil {
		return fmt.Errorf("invalid user id: %w", err)
	}

	now := time.Now()
	item.Status = "resolved"
	item.ResolvedBy = &userUUID
	item.ResolvedAt = &now

	var metadata map[string]interface{}
	if item.MetadataJSON != "" && item.MetadataJSON != "{}" {
		_ = json.Unmarshal([]byte(item.MetadataJSON), &metadata)
	}
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadata["justification"] = justification
	metadataBytes, _ := json.Marshal(metadata)
	item.MetadataJSON = string(metadataBytes)

	if err := u.actionRepo.Update(ctx, item); err != nil {
		return fmt.Errorf("failed to update action item: %w", err)
	}

	// Trigger asynchronous feedback loop task for Python ML
	taskPayload, err := json.Marshal(map[string]interface{}{
		"item_id":       itemID,
		"justification": justification,
		"action":        "LEARN_OVERRIDE",
	})
	if err != nil {
		return fmt.Errorf("failed to marshal task payload: %w", err)
	}

	asynqTask := asynq.NewTask("ml:update_memory_pack", taskPayload, asynq.Queue("default"))
	if _, err := u.asynqClient.EnqueueTask(asynqTask); err != nil {
		return fmt.Errorf("failed to enqueue ML feedback task: %w", err)
	}

	return nil
}
