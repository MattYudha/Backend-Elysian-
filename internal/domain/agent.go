package domain

import (
	"context"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type Agent struct {
	ID          uuid.UUID      `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	TenantID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"tenant_id"`
	Name        string         `gorm:"type:varchar(255);not null" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	ModelUsed   string         `gorm:"type:varchar(100);not null" json:"model_used"`
	Status      string         `gorm:"type:varchar(50);default:'active'" json:"status"`
	Skills      []Skill        `gorm:"foreignKey:AgentID;constraint:OnDelete:CASCADE" json:"skills,omitempty"`
}

type Skill struct {
	ID                uuid.UUID      `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	AgentID           uuid.UUID      `gorm:"type:uuid;not null;index" json:"agent_id"`
	Name              string         `gorm:"type:varchar(255);not null" json:"name"`
	ConfigurationJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"configuration_json"`
}

type AgentRepository interface {
	Create(ctx context.Context, agent *Agent) error
	FindByID(ctx context.Context, tenantID, id string) (*Agent, error)
	List(ctx context.Context, tenantID string) ([]*Agent, error)
	Update(ctx context.Context, agent *Agent) error
	Delete(ctx context.Context, tenantID, id string) error

	CreateSkill(ctx context.Context, skill *Skill) error
	DeleteSkill(ctx context.Context, agentID, skillID string) error
}
