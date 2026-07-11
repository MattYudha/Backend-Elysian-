package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"gorm.io/gorm"
)

type agentRepository struct {
	db *gorm.DB
}

func NewAgentRepository(db *gorm.DB) domain.AgentRepository {
	return &agentRepository{db: db}
}

func (r *agentRepository) Create(ctx context.Context, agent *domain.Agent) error {
	if err := r.db.WithContext(ctx).Create(agent).Error; err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}
	return nil
}

func (r *agentRepository) FindByID(ctx context.Context, tenantID, id string) (*domain.Agent, error) {
	var agent domain.Agent
	err := r.db.WithContext(ctx).
		Preload("Skills").
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&agent).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("agent not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find agent: %w", err)
	}

	return &agent, nil
}

func (r *agentRepository) List(ctx context.Context, tenantID string) ([]*domain.Agent, error) {
	var agents []*domain.Agent
	err := r.db.WithContext(ctx).
		Preload("Skills").
		Where("tenant_id = ?", tenantID).
		Find(&agents).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	return agents, nil
}

func (r *agentRepository) Update(ctx context.Context, agent *domain.Agent) error {
	result := r.db.WithContext(ctx).Save(agent)
	if result.Error != nil {
		return fmt.Errorf("failed to update agent: %w", result.Error)
	}
	return nil
}

func (r *agentRepository) Delete(ctx context.Context, tenantID, id string) error {
	result := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&domain.Agent{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete agent: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("agent not found")
	}
	return nil
}

func (r *agentRepository) CreateSkill(ctx context.Context, skill *domain.Skill) error {
	if err := r.db.WithContext(ctx).Create(skill).Error; err != nil {
		return fmt.Errorf("failed to create skill: %w", err)
	}
	return nil
}

func (r *agentRepository) DeleteSkill(ctx context.Context, agentID, skillID string) error {
	result := r.db.WithContext(ctx).
		Where("agent_id = ? AND id = ?", agentID, skillID).
		Delete(&domain.Skill{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete skill: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("skill not found")
	}
	return nil
}
