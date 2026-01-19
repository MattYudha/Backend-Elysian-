package repository

import (
	"context"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, limit, offset int) ([]*domain.User, int64, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
}
