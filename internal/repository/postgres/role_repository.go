package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/domain/repository"
	"gorm.io/gorm"
)

type RoleRepository struct {
	db *gorm.DB
}

func NewRoleRepository(db *gorm.DB) repository.RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) Create(ctx context.Context, role *domain.Role) error {
	if err := r.db.WithContext(ctx).Create(role).Error; err != nil {
		return fmt.Errorf("failed to create role: %w", err)
	}
	return nil
}

func (r *RoleRepository) FindByID(ctx context.Context, id string) (*domain.Role, error) {
	var role domain.Role
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&role).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("role not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find role: %w", err)
	}

	return &role, nil
}

func (r *RoleRepository) FindByName(ctx context.Context, name string) (*domain.Role, error) {
	var role domain.Role
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&role).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("role not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find role: %w", err)
	}

	return &role, nil
}

func (r *RoleRepository) Update(ctx context.Context, role *domain.Role) error {
	result := r.db.WithContext(ctx).Save(role)
	if result.Error != nil {
		return fmt.Errorf("failed to update role: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("role not found")
	}
	return nil
}

func (r *RoleRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&domain.Role{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete role: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("role not found")
	}
	return nil
}

func (r *RoleRepository) List(ctx context.Context) ([]*domain.Role, error) {
	var roles []*domain.Role
	err := r.db.WithContext(ctx).Order("name ASC").Find(&roles).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}
	return roles, nil
}

func (r *RoleRepository) AssignToUser(ctx context.Context, userID, roleID string) error {
	userRole := &domain.UserRole{
		UserID: userID,
		RoleID: roleID,
	}

	if err := r.db.WithContext(ctx).Create(userRole).Error; err != nil {
		return fmt.Errorf("failed to assign role to user: %w", err)
	}

	return nil
}

func (r *RoleRepository) RemoveFromUser(ctx context.Context, userID, roleID string) error {
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND role_id = ?", userID, roleID).
		Delete(&domain.UserRole{})

	if result.Error != nil {
		return fmt.Errorf("failed to remove role from user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("user role assignment not found")
	}

	return nil
}

func (r *RoleRepository) GetUserRoles(ctx context.Context, userID string) ([]*domain.Role, error) {
	var roles []*domain.Role

	err := r.db.WithContext(ctx).
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Find(&roles).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}

	return roles, nil
}
