package domain

import (
	"time"

	"github.com/google/uuid"
)

type Pipeline struct {
	ID                uuid.UUID  `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	TenantID          uuid.UUID  `gorm:"type:uuid;not null" json:"tenant_id"`
	WorkflowVersionID uuid.UUID  `gorm:"type:uuid;not null" json:"workflow_version_id"`
	Name              string     `gorm:"type:varchar(255);not null" json:"name"`
	Status            string     `gorm:"type:varchar(50);default:'running'" json:"status"`
	ExecutionTimeMs   int        `gorm:"type:int" json:"execution_time_ms"`
	StartedAt         time.Time  `gorm:"autoCreateTime" json:"started_at"`
	CompletedAt       *time.Time `json:"completed_at"`
}

type Workstream struct {
	ID         uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	PipelineID uuid.UUID `gorm:"type:uuid;not null" json:"pipeline_id"`
	Name       string    `gorm:"type:varchar(255);not null" json:"name"`
	Type       string    `gorm:"type:varchar(100);not null" json:"type"`
}
