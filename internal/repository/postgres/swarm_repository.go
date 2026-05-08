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
