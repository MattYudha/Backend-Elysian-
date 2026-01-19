package domain

import (
	"time"

	"gorm.io/datatypes"
)

type Role struct {
	ID          string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name        string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"name"`
	Description *string        `gorm:"type:text" json:"description,omitempty"`
	Permissions datatypes.JSON `gorm:"type:jsonb;default:'[]';not null" json:"permissions"`
	CreatedAt   time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Role) TableName() string {
	return "roles"
}

type UserRole struct {
	ID        string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    string    `gorm:"type:uuid;not null;index" json:"user_id"`
	RoleID    string    `gorm:"type:uuid;not null;index" json:"role_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	User User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"user,omitempty"`
	Role Role `gorm:"foreignKey:RoleID;references:ID;constraint:OnDelete:CASCADE" json:"role,omitempty"`
}

func (UserRole) TableName() string {
	return "user_roles"
}
