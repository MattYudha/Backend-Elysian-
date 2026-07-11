package postgres

import (
	"context"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type actionItemRepository struct {
	db *gorm.DB
}

func NewActionItemRepository(db *gorm.DB) domain.ActionItemRepository {
	return &actionItemRepository{db: db}
}

func (r *actionItemRepository) Create(ctx context.Context, item *domain.ActionItem) error {
	return r.db.WithContext(ctx).Table("action_items").Create(item).Error
}

func (r *actionItemRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.ActionItem, error) {
	var item domain.ActionItem
	err := r.db.WithContext(ctx).Table("action_items").Where("id = ?", id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *actionItemRepository) List(ctx context.Context, tenantID uuid.UUID, status string) ([]*domain.ActionItem, error) {
	var items []*domain.ActionItem
	query := r.db.WithContext(ctx).Table("action_items").Where("tenant_id = ?", tenantID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Order("created_at desc").Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

func (r *actionItemRepository) Update(ctx context.Context, item *domain.ActionItem) error {
	return r.db.WithContext(ctx).Table("action_items").Save(item).Error
}
