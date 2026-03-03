package domain

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`
	Email        string    `gorm:"type:varchar(255);unique;not null" json:"email"`
	FullName     string    `gorm:"type:varchar(255);not null" json:"full_name"`
	AvatarURL    string    `gorm:"type:text" json:"avatar_url"`
	PasswordHash string    `gorm:"type:varchar(255)" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
