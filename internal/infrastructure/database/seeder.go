package database

import (
	"log"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/auth"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SeedAdmin seeds the default administrator accounts and system roles.
func SeedAdmin(db *gorm.DB) error {
	adminEmails := []string{"admin@gmail.com", "adin@gmail.com", "dewarahmat7234@gmail.com"}
	const adminPasswordRaw = "82471129Qwe!"

	passwordService := auth.NewPasswordService()
	hashedPassword, err := passwordService.HashPassword(adminPasswordRaw)
	if err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		adminUserIDs := make(map[string]uuid.UUID)

		for _, email := range adminEmails {
			var existingUser domain.User
			result := tx.Where("email = ?", email).First(&existingUser)

			var userID uuid.UUID

			if result.Error == nil {
				log.Printf("User %s exists. Ensuring admin password hash...", email)
				userID = existingUser.ID
				existingUser.PasswordHash = hashedPassword
				if err := tx.Save(&existingUser).Error; err != nil {
					return err
				}
			} else if result.Error == gorm.ErrRecordNotFound {
				log.Printf("Creating new admin user %s...", email)
				userID = uuid.New()
				newUser := domain.User{
					ID:           userID,
					FullName:     "Super Admin",
					Email:        email,
					PasswordHash: hashedPassword,
					CreatedAt:    time.Now(),
					UpdatedAt:    time.Now(),
				}
				if err := tx.Create(&newUser).Error; err != nil {
					return err
				}
			} else {
				return result.Error
			}
			adminUserIDs[email] = userID
		}

		// Ensure Role Exists
		var adminRole domain.Role
		if err := tx.Where("name = ?", "admin").First(&adminRole).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				log.Println("Creating 'admin' role...")
				adminRole = domain.Role{
					ID:   uuid.New(),
					Name: "admin",
				}
				if err := tx.Create(&adminRole).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}

		// Ensure default tenants exist and assign admin role
		tenantsToSeed := []string{"System", "Workspace A", "Workspace B"}
		for _, name := range tenantsToSeed {
			type TenantTemp struct {
				ID uuid.UUID
			}
			var existingTenant TenantTemp
			if err := tx.Table("tenants").Select("id").Where("name = ?", name).Limit(1).Scan(&existingTenant).Error; err != nil {
				return err
			}

			var tenantID uuid.UUID
			if existingTenant.ID == uuid.Nil {
				tenantID = uuid.New()
				log.Printf("Creating tenant %s...", name)
				if err := tx.Exec("INSERT INTO tenants (id, name, plan_tier) VALUES (?, ?, 'enterprise')", tenantID, name).Error; err != nil {
					return err
				}
			} else {
				tenantID = existingTenant.ID
			}

			// Assign role to each user in each tenant
			for email, uID := range adminUserIDs {
				var tenantUser domain.TenantUser
				err := tx.Where("tenant_id = ? AND user_id = ?", tenantID, uID).First(&tenantUser).Error
				if err != nil {
					if err == gorm.ErrRecordNotFound {
						log.Printf("Assigning 'admin' role to user %s in tenant %s...", email, name)
						tenantUser = domain.TenantUser{
							TenantID: tenantID,
							UserID:   uID,
							RoleID:   adminRole.ID,
							JoinedAt: time.Now(),
						}
						if err := tx.Create(&tenantUser).Error; err != nil {
							return err
						}
					} else {
						return err
					}
				} else if tenantUser.RoleID != adminRole.ID {
					log.Printf("Updating user %s role to 'admin' in tenant %s...", email, name)
					tenantUser.RoleID = adminRole.ID
					if err := tx.Save(&tenantUser).Error; err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
}
