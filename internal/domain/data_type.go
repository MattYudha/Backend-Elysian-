package domain

import (
	"time"

	"github.com/google/uuid"
)

type DataType struct {
	ID          uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	TenantID    uuid.UUID `gorm:"type:uuid;not null" json:"tenant_id"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	FieldsCount int       `gorm:"default:0" json:"fields_count"`
	IsSystem    bool      `gorm:"default:false" json:"is_system"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
