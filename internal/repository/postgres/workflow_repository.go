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

func (r *workflowRepository) List(ctx context.Context, tenantID string, limit, offset int) ([]*domain.Workflow, int64, error) {
	var workflows []*domain.Workflow
	var total int64

	db := r.db.WithContext(ctx).Model(&domain.Workflow{}).Where("tenant_id = ?", tenantID)

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

func (r *workflowRepository) UpdateGraph(ctx context.Context, workflowID string, configuration []byte) error {
	var wf domain.Workflow
	if err := r.db.WithContext(ctx).Where("id = ?", workflowID).First(&wf).Error; err != nil {
		return fmt.Errorf("workflow not found: %w", err)
	}

	var latestVersion domain.WorkflowVersion
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("version_number DESC").
		First(&latestVersion).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Create first version (version 1)
		v := &domain.WorkflowVersion{
			WorkflowID:    wf.ID,
			VersionNumber: 1,
			Configuration: configuration,
		}
		return r.db.WithContext(ctx).Create(v).Error
	} else if err != nil {
		return fmt.Errorf("failed to check latest version: %w", err)
	}

	// If workflow is active/published/running, create a new version incremented
	if wf.Status == "active" || wf.Status == "published" || wf.Status == "running" || wf.Status == "completed" {
		v := &domain.WorkflowVersion{
			WorkflowID:    wf.ID,
			VersionNumber: latestVersion.VersionNumber + 1,
			Configuration: configuration,
		}
		return r.db.WithContext(ctx).Create(v).Error
	}

	// Otherwise, overwrite the latest version (which is a draft)
	latestVersion.Configuration = configuration
	return r.db.WithContext(ctx).Save(&latestVersion).Error
}

func (r *workflowRepository) GetLatestVersion(ctx context.Context, workflowID string) (*domain.WorkflowVersion, error) {
	var version domain.WorkflowVersion
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("version_number DESC").
		First(&version).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // No version exists yet
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest version: %w", err)
	}
	return &version, nil
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
