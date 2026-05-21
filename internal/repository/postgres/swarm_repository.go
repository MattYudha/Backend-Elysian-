package postgres

import (
	"context"
	"fmt"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"gorm.io/gorm"
)

type SwarmRepository struct {
	db *gorm.DB
}

func NewSwarmRepository(db *gorm.DB) *SwarmRepository {
	return &SwarmRepository{db: db}
}

func (r *SwarmRepository) GetDB() *gorm.DB {
	return r.db
}

func (r *SwarmRepository) Create(ctx context.Context, task *domain.SwarmTask) error {
	if err := r.db.WithContext(ctx).Create(task).Error; err != nil {
		return fmt.Errorf("failed to create swarm task: %w", err)
	}
	return nil
}

func (r *SwarmRepository) GetByID(ctx context.Context, id string) (*domain.SwarmTask, error) {
	var task domain.SwarmTask
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&task).Error; err != nil {
		return nil, fmt.Errorf("failed to get swarm task: %w", err)
	}
	return &task, nil
}

func (r *SwarmRepository) Update(ctx context.Context, task *domain.SwarmTask) error {
	if err := r.db.WithContext(ctx).Save(task).Error; err != nil {
		return fmt.Errorf("failed to update swarm task: %w", err)
	}
	return nil
}

func (r *SwarmRepository) ListByTenant(ctx context.Context, tenantID string, limit, offset int) ([]*domain.SwarmTask, int64, error) {
	var tasks []*domain.SwarmTask
	var total int64

	// 1. Separate Count Query to avoid GORM v2 statement mutation bugs
	if err := r.db.WithContext(ctx).
		Model(&domain.SwarmTask{}).
		Joins("JOIN documents ON documents.id = swarm_tasks.document_id").
		Where("documents.tenant_id = ?", tenantID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 2. Separate Find Query to fetch the actual tasks cleanly
	err := r.db.WithContext(ctx).
		Table("swarm_tasks").
		Select("swarm_tasks.*").
		Joins("JOIN documents ON documents.id = swarm_tasks.document_id").
		Where("documents.tenant_id = ?", tenantID).
		Limit(limit).
		Offset(offset).
		Order("swarm_tasks.created_at DESC").
		Find(&tasks).
		Error

	return tasks, total, err
}
