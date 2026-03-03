package domain

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AuditLog represents a forensic trail record in the enterprise_audit_logs partitioned table.
type AuditLog struct {
	ID           uuid.UUID       `gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	TenantID     uuid.UUID       `gorm:"type:uuid;index"`
	ActorID      uuid.UUID       `gorm:"type:uuid;index"`
	Action       string          `gorm:"type:varchar(255)"`
	ResourceType string          `gorm:"type:varchar(100)"`
	ResourceID   uuid.UUID       `gorm:"type:uuid"`
	ContextIP    string          `gorm:"type:inet"`
	Evidence     json.RawMessage `gorm:"type:jsonb"`
	CreatedAt    time.Time       `gorm:"primaryKey"` // Required for table partitioning
}

// AuditRepository defines the interface for persisting audit logs cleanly.
type AuditRepository interface {
	Create(ctx context.Context, audit *AuditLog) error
}
