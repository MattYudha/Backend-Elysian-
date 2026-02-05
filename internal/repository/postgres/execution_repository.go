package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type executionRepository struct {
	db *gorm.DB
}

func NewExecutionRepository(db *gorm.DB) *executionRepository {
	return &executionRepository{db: db}
}

func (r *executionRepository) Create(ctx context.Context, execution *domain.Execution) error {
	if err := r.db.WithContext(ctx).Create(execution).Error; err != nil {
		return fmt.Errorf("failed to create execution: %w", err)
	}
	return nil
}

func (r *executionRepository) FindByID(ctx context.Context, id string) (*domain.Execution, error) {
	var execution domain.Execution
	err := r.db.WithContext(ctx).
		Preload("Logs", func(db *gorm.DB) *gorm.DB {
			return db.Order("execution_logs.created_at ASC")
		}).
		Where("id = ?", id).
		First(&execution).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("execution not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find execution: %w", err)
	}

	return &execution, nil
}

func (r *executionRepository) List(ctx context.Context, workflowID string, limit, offset int) ([]*domain.Execution, int64, error) {
	var executions []*domain.Execution
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.Execution{}).Where("workflow_id = ?", workflowID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count executions: %w", err)
	}

	err := db.Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&executions).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to list executions: %w", err)
	}

	return executions, total, nil
}

func (r *executionRepository) UpdateStatus(ctx context.Context, id string, status domain.ExecutionStatus, output map[string]interface{}) error {
	updateData := map[string]interface{}{
		"status": status,
	}

	if status == domain.ExecutionStatusRunning {
		updateData["started_at"] = time.Now()
	}

	if status == domain.ExecutionStatusCompleted || status == domain.ExecutionStatusFailed || status == domain.ExecutionStatusCancelled {
		updateData["finished_at"] = time.Now()
		// Calculate duration if started_at exists
		var execution domain.Execution
		if err := r.db.Select("started_at").First(&execution, "id = ?", id).Error; err == nil && execution.StartedAt != nil {
			updateData["duration"] = time.Since(*execution.StartedAt).Seconds()
		}
	}

	if output != nil {
		jsonOutput, err := json.Marshal(output)
		if err == nil {
			updateData["output"] = datatypes.JSON(jsonOutput)
		}
	}

	if err := r.db.WithContext(ctx).Model(&domain.Execution{}).Where("id = ?", id).Updates(updateData).Error; err != nil {
		return fmt.Errorf("failed to update execution status: %w", err)
	}
	return nil
}

func (r *executionRepository) AddLog(ctx context.Context, log *domain.ExecutionLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("failed to create execution log: %w", err)
	}
	return nil
}
