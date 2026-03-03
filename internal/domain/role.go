package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Role struct {
	ID          uuid.UUID      `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	TenantID    *uuid.UUID     `gorm:"type:uuid" json:"tenant_id"` // Nullable untuk role sistem
	Name        string         `gorm:"type:varchar(100);not null" json:"name"`
	Permissions datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"permissions"`
}

type TenantUser struct {
	TenantID uuid.UUID  `gorm:"type:uuid;primaryKey" json:"tenant_id"`
	UserID   uuid.UUID  `gorm:"type:uuid;primaryKey" json:"user_id"`
	RoleID   *uuid.UUID `gorm:"type:uuid" json:"role_id"`
	JoinedAt time.Time  `json:"joined_at"`
}
