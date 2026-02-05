package domain

import (
	"context"
	"time"

	"gorm.io/datatypes"
)

type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "PENDING"
	ExecutionStatusRunning   ExecutionStatus = "RUNNING"
	ExecutionStatusCompleted ExecutionStatus = "COMPLETED"
	ExecutionStatusFailed    ExecutionStatus = "FAILED"
	ExecutionStatusCancelled ExecutionStatus = "CANCELLED"
)

type Execution struct {
	ID         string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WorkflowID string          `gorm:"type:uuid;not null;index" json:"workflow_id"`
	UserID     string          `gorm:"type:uuid;not null;index" json:"user_id"`
	Status     ExecutionStatus `gorm:"type:varchar(50);default:'PENDING';not null" json:"status"`
	Input      datatypes.JSON  `gorm:"type:jsonb" json:"input,omitempty"`
	Output     datatypes.JSON  `gorm:"type:jsonb" json:"output,omitempty"`
	StartedAt  *time.Time      `json:"started_at,omitempty"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
	Duration   float64         `gorm:"type:real" json:"duration,omitempty"` // in seconds
	CreatedAt  time.Time       `gorm:"autoCreateTime" json:"created_at"`

	// Relations
	Workflow Workflow       `gorm:"foreignKey:WorkflowID" json:"-"`
	Logs     []ExecutionLog `gorm:"foreignKey:ExecutionID;constraint:OnDelete:CASCADE" json:"logs,omitempty"`
}

type ExecutionLog struct {
	ID          string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ExecutionID string         `gorm:"type:uuid;not null;index" json:"execution_id"`
	NodeID      *string        `gorm:"type:uuid" json:"node_id,omitempty"` // Optional: global log vs node log
	Level       string         `gorm:"type:varchar(20);default:'INFO'" json:"level"`
	Message     string         `gorm:"type:text;not null" json:"message"`
	Details     datatypes.JSON `gorm:"type:jsonb" json:"details,omitempty"`
	Timestamp   time.Time      `gorm:"autoCreateTime" json:"timestamp"`
}

func (Execution) TableName() string {
	return "executions"
}

func (ExecutionLog) TableName() string {
	return "execution_logs"
}

type ExecutionRepository interface {
	Create(ctx context.Context, execution *Execution) error
	FindByID(ctx context.Context, id string) (*Execution, error)
	List(ctx context.Context, workflowID string, limit, offset int) ([]*Execution, int64, error)
	UpdateStatus(ctx context.Context, id string, status ExecutionStatus, output map[string]interface{}) error
	AddLog(ctx context.Context, log *ExecutionLog) error
}
