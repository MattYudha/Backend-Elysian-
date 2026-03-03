package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"gorm.io/gorm"
)

type workflowRepository struct {
	db *gorm.DB
}

func NewWorkflowRepository(db *gorm.DB) *workflowRepository {
	return &workflowRepository{db: db}
}

func (r *workflowRepository) Create(ctx context.Context, workflow *domain.Workflow) error {
	if err := r.db.WithContext(ctx).Create(workflow).Error; err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}
	return nil
}

func (r *workflowRepository) FindByID(ctx context.Context, id string) (*domain.Workflow, error) {
	var workflow domain.Workflow
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&workflow).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("workflow not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow: %w", err)
	}

	return &workflow, nil
}

func (r *workflowRepository) List(ctx context.Context, userID string, limit, offset int) ([]*domain.Workflow, int64, error) {
	var workflows []*domain.Workflow
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.Workflow{}).Where("user_id = ?", userID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count workflows: %w", err)
	}

	err := db.Limit(limit).
		Offset(offset).
		Order("updated_at DESC").
		Find(&workflows).Error

	if err != nil {
		return nil, 0, fmt.Errorf("failed to list workflows: %w", err)
	}

	return workflows, total, nil
}

func (r *workflowRepository) Update(ctx context.Context, workflow *domain.Workflow) error {
	result := r.db.WithContext(ctx).Save(workflow)
	if result.Error != nil {
		return fmt.Errorf("failed to update workflow: %w", result.Error)
	}
	return nil
}

func (r *workflowRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&domain.Workflow{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete workflow: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("workflow not found")
	}
	return nil
}

// UpdateGraph is deprecated in the new versioning system. Replaced by WorkflowVersion.
// This is a stub to satisfy interfaces until the DAG engine is rebuilt.
func (r *workflowRepository) UpdateGraph(ctx context.Context, workflowID string, configuration []byte) error {
	return fmt.Errorf("Not Implemented: DAG Engine requires WorkflowVersion")
}

func (r *workflowRepository) GetVersionByID(ctx context.Context, versionID string) (*domain.WorkflowVersion, error) {
	var version domain.WorkflowVersion
	err := r.db.WithContext(ctx).Where("id = ?", versionID).First(&version).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("workflow version not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find workflow version: %w", err)
	}
	return &version, nil
}

func (r *workflowRepository) CreatePipeline(ctx context.Context, pipeline *domain.Pipeline) error {
	if err := r.db.WithContext(ctx).Create(pipeline).Error; err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
	}
	return nil
}

func (r *workflowRepository) UpdatePipeline(ctx context.Context, pipeline *domain.Pipeline) error {
	if err := r.db.WithContext(ctx).Save(pipeline).Error; err != nil {
		return fmt.Errorf("failed to update pipeline: %w", err)
	}
	return nil
}
