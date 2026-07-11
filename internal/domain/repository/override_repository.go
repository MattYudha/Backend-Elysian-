package repository

import (
	"context"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
)

type OverrideRepository interface {
	Save(ctx context.Context, override *domain.AuditorOverride) error
	GetByItemName(ctx context.Context, tenantID, itemName string) (*domain.AuditorOverride, error)
	List(ctx context.Context, tenantID string) ([]*domain.AuditorOverride, error)
}
