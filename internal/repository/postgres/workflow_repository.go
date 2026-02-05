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
		Preload("Nodes").
		Preload("Edges").
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

// UpdateGraph implements the "Nuke & Pave" strategy transactionally
func (r *workflowRepository) UpdateGraph(ctx context.Context, workflowID string, nodes []domain.WorkflowNode, edges []domain.WorkflowEdge) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 1. Delete existing Edges (Edges depend on Nodes, so delete first)
		if err := tx.Where("workflow_id = ?", workflowID).Delete(&domain.WorkflowEdge{}).Error; err != nil {
			return fmt.Errorf("failed to delete edges: %w", err)
		}

		// 2. Delete existing Nodes
		if err := tx.Where("workflow_id = ?", workflowID).Delete(&domain.WorkflowNode{}).Error; err != nil {
			return fmt.Errorf("failed to delete nodes: %w", err)
		}

		// 3. Bulk Insert Nodes
		if len(nodes) > 0 {
			if err := tx.Create(&nodes).Error; err != nil {
				return fmt.Errorf("failed to insert nodes: %w", err)
			}
		}

		// 4. Bulk Insert Edges
		if len(edges) > 0 {
			if err := tx.Create(&edges).Error; err != nil {
				return fmt.Errorf("failed to insert edges: %w", err)
			}
		}

		// 5. Explicitly update workflow updated_at
		if err := tx.Model(&domain.Workflow{}).Where("id = ?", workflowID).Update("updated_at", gorm.Expr("current_timestamp")).Error; err != nil {
			return err
		}

		return nil
	})
}
