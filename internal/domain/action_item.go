package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type ActionItem struct {
	ID           uuid.UUID  `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	TenantID     uuid.UUID  `json:"tenant_id" gorm:"type:uuid;not null;index"`
	Type         string     `json:"type" gorm:"type:varchar(50);not null"`
	Status       string     `json:"status" gorm:"type:varchar(50);default:'pending'"` // pending, resolved, deleted
	Description  string     `json:"description" gorm:"type:text;not null"`
	MetadataJSON string     `json:"metadata_json" gorm:"column:metadata_json;type:jsonb;default:'{}'"`
	CreatedAt    time.Time  `json:"created_at" gorm:"autoCreateTime"`
	ResolvedBy   *uuid.UUID `json:"resolved_by,omitempty" gorm:"type:uuid"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

type ActionItemRepository interface {
	Create(ctx context.Context, item *ActionItem) error
	GetByID(ctx context.Context, id uuid.UUID) (*ActionItem, error)
	List(ctx context.Context, tenantID uuid.UUID, status string) ([]*ActionItem, error)
	Update(ctx context.Context, item *ActionItem) error
}
