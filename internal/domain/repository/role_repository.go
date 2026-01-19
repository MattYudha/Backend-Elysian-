package repository

import (
	"context"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
)

type RoleRepository interface {
	Create(ctx context.Context, role *domain.Role) error
	FindByID(ctx context.Context, id string) (*domain.Role, error)
	FindByName(ctx context.Context, name string) (*domain.Role, error)
	Update(ctx context.Context, role *domain.Role) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]*domain.Role, error)
	AssignToUser(ctx context.Context, userID, roleID string) error
	RemoveFromUser(ctx context.Context, userID, roleID string) error
	GetUserRoles(ctx context.Context, userID string) ([]*domain.Role, error)
}
