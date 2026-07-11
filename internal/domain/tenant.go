package domain

import (
	"time"

	"github.com/google/uuid"
)

type Tenant struct {
	ID           uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Name         string    `gorm:"type:varchar(255);not null" json:"name"`
	PlanTier     string    `gorm:"type:varchar(50);default:'free'" json:"plan_tier"`
	Status       string    `gorm:"type:varchar(50);default:'active'" json:"status"`
	HealthScore  int       `gorm:"default:100" json:"health_score"`
	BillingCycle string    `gorm:"type:varchar(50);default:'monthly'" json:"billing_cycle"`
	CreatedAt    time.Time `json:"created_at"`
}
