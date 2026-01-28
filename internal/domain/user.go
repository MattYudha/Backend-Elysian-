package domain

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID              string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email           string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	PasswordHash    string         `gorm:"type:varchar(255);not null" json:"-"`
	Name            string         `gorm:"type:varchar(255);not null" json:"name"`
	AvatarURL       *string        `gorm:"type:varchar(500)" json:"avatar_url,omitempty"`
	IsActive        bool           `gorm:"default:true;not null" json:"is_active"`
	EmailVerifiedAt *time.Time     `json:"email_verified_at,omitempty"`
	LastLoginAt     *time.Time     `json:"last_login_at,omitempty"`
	CreatedAt       time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty" swaggertype:"string" format:"date-time"`
}

func (User) TableName() string {
	return "users"
}
