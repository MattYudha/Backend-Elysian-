package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/domain/repository"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type RoleRepository struct {
	db          *gorm.DB
	redisClient *redis.Client
}

func NewRoleRepository(db *gorm.DB, redisClient *redis.Client) repository.RoleRepository {
	return &RoleRepository{db: db, redisClient: redisClient}
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

	// Invalidasi Cache secara SINKRON untuk seluruh tenant_user yang mengemban role ini
	var tenantUsers []domain.TenantUser
	r.db.WithContext(ctx).Select("tenant_id", "user_id").Where("role_id = ?", role.ID).Find(&tenantUsers)

	if len(tenantUsers) > 0 {
		pipe := r.redisClient.Pipeline()
		for _, tu := range tenantUsers {
			pipe.Del(ctx, "auth:rbac:"+tu.TenantID.String()+":"+tu.UserID.String())
		}
		_, _ = pipe.Exec(ctx)
	}

	return nil
}

func (r *RoleRepository) Delete(ctx context.Context, id string) error {
	// Kumpulkan affected users sebelum di-Delete di Pg
	var tenantUsers []domain.TenantUser
	r.db.WithContext(ctx).Select("tenant_id", "user_id").Where("role_id = ?", id).Find(&tenantUsers)

	result := r.db.WithContext(ctx).Delete(&domain.Role{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete role: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("role not found")
	}

	// Invalidasi Cache secara SINKRON setelah Pg sukses
	if len(tenantUsers) > 0 {
		pipe := r.redisClient.Pipeline()
		for _, tu := range tenantUsers {
			pipe.Del(ctx, "auth:rbac:"+tu.TenantID.String()+":"+tu.UserID.String())
		}
		_, _ = pipe.Exec(ctx)
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

func (r *RoleRepository) AssignToUser(ctx context.Context, tenantID, userID, roleID string) error {
	tID, err := uuid.Parse(tenantID)
	if err != nil {
		return err
	}
	uID, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	rID, err := uuid.Parse(roleID)
	if err != nil {
		return err
	}

	tenantUser := &domain.TenantUser{
		TenantID: tID,
		UserID:   uID,
		RoleID:   &rID,
	}

	if err := r.db.WithContext(ctx).Create(tenantUser).Error; err != nil {
		return fmt.Errorf("failed to assign role to user in tenant: %w", err)
	}

	// SINKRON Invalidasi
	r.redisClient.Del(ctx, "auth:rbac:"+tenantID+":"+userID)

	return nil
}

func (r *RoleRepository) RemoveFromUser(ctx context.Context, tenantID, userID, roleID string) error {
	result := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ? AND role_id = ?", tenantID, userID, roleID).
		Delete(&domain.TenantUser{})

	if result.Error != nil {
		return fmt.Errorf("failed to remove role from user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("tenant user assignment not found")
	}

	// SINKRON Invalidasi
	r.redisClient.Del(ctx, "auth:rbac:"+tenantID+":"+userID)

	return nil
}

func (r *RoleRepository) GetUserRoles(ctx context.Context, tenantID, userID string) ([]*domain.Role, error) {
	var roles []*domain.Role

	err := r.db.WithContext(ctx).
		Joins("JOIN tenant_users ON tenant_users.role_id = roles.id").
		Where("tenant_users.tenant_id = ? AND tenant_users.user_id = ?", tenantID, userID).
		Find(&roles).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get user roles for tenant: %w", err)
	}

	return roles, nil
}
