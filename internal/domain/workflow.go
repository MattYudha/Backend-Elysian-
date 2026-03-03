package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Workflow struct {
	ID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	TenantID  uuid.UUID `gorm:"type:uuid;not null" json:"tenant_id"`
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	Status    string    `gorm:"type:varchar(50);default:'draft'" json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type WorkflowVersion struct {
	ID            uuid.UUID      `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	WorkflowID    uuid.UUID      `gorm:"type:uuid;not null" json:"workflow_id"`
	VersionNumber int            `gorm:"not null" json:"version_number"`
	Configuration datatypes.JSON `gorm:"type:jsonb;not null" json:"configuration"`
	CreatedAt     time.Time      `json:"created_at"`
}
