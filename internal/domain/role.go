package domain

import (
	"encoding/json"
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

func (r *Role) GetPermissions() []string {
	var perms []string

	if err := json.Unmarshal(r.Permissions, &perms); err != nil {
		return []string{}
	}

	return perms
}

func (r *Role) HasPermission(permission string) bool {
	perms := r.GetPermissions()

	for _, perm := range perms {
		if perm == "*" || perm == permission {
			return true
		}
	}

	return false
}

func (r *Role) HasAllPermissions(permissions ...string) bool {
	perms := r.GetPermissions()
	permMap := make(map[string]bool)

	for _, perm := range perms {
		permMap[perm] = true
	}

	if permMap["*"] {
		return true
	}

	for _, required := range permissions {
		if !permMap[required] {
			return false
		}
	}

	return true
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
