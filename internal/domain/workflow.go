package domain

import (
	"context"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type WorkflowStatus string

const (
	WorkflowStatusDraft     WorkflowStatus = "draft"
	WorkflowStatusPublished WorkflowStatus = "published"
	WorkflowStatusArchived  WorkflowStatus = "archived"
)

type Workflow struct {
	ID            string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID        string         `gorm:"type:uuid;not null;index" json:"user_id"`
	Name          string         `gorm:"type:varchar(255);not null" json:"name"`
	Description   *string        `gorm:"type:text" json:"description,omitempty"`
	Version       int            `gorm:"default:1;not null" json:"version"`
	Status        WorkflowStatus `gorm:"type:varchar(50);default:'draft';not null" json:"status"`
	IsTemplate    bool           `gorm:"default:false;not null" json:"is_template"`
	IsPublic      bool           `gorm:"default:false;not null" json:"is_public"`
	Tags          datatypes.JSON `gorm:"type:jsonb" json:"tags,omitempty"`
	Configuration datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"configuration"`
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string" format:"date-time"`

	// Relationships
	Nodes []WorkflowNode `gorm:"foreignKey:WorkflowID;constraint:OnDelete:CASCADE" json:"nodes,omitempty"`
	Edges []WorkflowEdge `gorm:"foreignKey:WorkflowID;constraint:OnDelete:CASCADE" json:"edges,omitempty"`
}

type WorkflowNode struct {
	ID            string         `gorm:"type:uuid;primaryKey" json:"id"` // ID from ReactFlow (frontend)
	WorkflowID    string         `gorm:"type:uuid;not null;index" json:"workflow_id"`
	NodeKey       string         `gorm:"type:varchar(100);not null" json:"node_key"`
	NodeType      string         `gorm:"type:varchar(100);not null" json:"type"`
	Label         *string        `gorm:"type:varchar(255)" json:"label,omitempty"`
	PositionX     float64        `gorm:"type:real" json:"position_x"`
	PositionY     float64        `gorm:"type:real" json:"position_y"`
	Configuration datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"data"`
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
}

type WorkflowEdge struct {
	ID            string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WorkflowID    string         `gorm:"type:uuid;not null;index" json:"workflow_id"`
	EdgeKey       string         `gorm:"type:varchar(100);not null" json:"edge_key"` // reactflow edge id
	SourceNodeID  string         `gorm:"type:uuid;not null;index" json:"source"`
	TargetNodeID  string         `gorm:"type:uuid;not null;index" json:"target"`
	SourceHandle  *string        `gorm:"type:varchar(100)" json:"sourceHandle"` // Critical for handles
	TargetHandle  *string        `gorm:"type:varchar(100)" json:"targetHandle"` // Critical for handles
	Type          string         `gorm:"type:varchar(50)" json:"type"`
	Animated      bool           `gorm:"default:false" json:"animated"`
	Configuration datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"data,omitempty"`
	CreatedAt     time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Workflow) TableName() string {
	return "workflows"
}

func (WorkflowNode) TableName() string {
	return "workflow_nodes"
}

func (WorkflowEdge) TableName() string {
	return "workflow_edges"
}

// Repository Interface
type WorkflowRepository interface {
	Create(ctx context.Context, workflow *Workflow) error
	FindByID(ctx context.Context, id string) (*Workflow, error)
	List(ctx context.Context, userID string, limit, offset int) ([]*Workflow, int64, error)
	Update(ctx context.Context, workflow *Workflow) error
	Delete(ctx context.Context, id string) error
	UpdateGraph(ctx context.Context, workflowID string, nodes []WorkflowNode, edges []WorkflowEdge) error
}
