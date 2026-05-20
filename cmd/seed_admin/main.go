package main

import (
	"log"
	"time"

	"github.com/Elysian-Rebirth/backend-go/internal/config"
	"github.com/Elysian-Rebirth/backend-go/internal/domain"
	"github.com/Elysian-Rebirth/backend-go/internal/infrastructure/database"
	"github.com/Elysian-Rebirth/backend-go/internal/usecase/auth"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func main() {
	// 1. Load Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Connect to Database
	db, err := database.NewPostgresDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	// Defer Close logic if implemented manually, generally GORM manages pool

	// 3. Define Admin Users
	adminEmails := []string{"admin@gmail.com", "adin@gmail.com"}
	const adminPasswordRaw = "password"

	// 4. Hash Password
	passwordService := auth.NewPasswordService()
	hashedPassword, err := passwordService.HashPassword(adminPasswordRaw)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// 5. Transaction to Seed
	err = db.Transaction(func(tx *gorm.DB) error {
		// Cache admin user IDs
		adminUserIDs := make(map[string]uuid.UUID)

		for _, email := range adminEmails {
			var existingUser domain.User
			result := tx.Where("email = ?", email).First(&existingUser)

			var userID uuid.UUID

			if result.Error == nil {
				// Update existing
				log.Printf("User %s exists. Updating password...", email)
				userID = existingUser.ID
				existingUser.PasswordHash = hashedPassword
				if err := tx.Save(&existingUser).Error; err != nil {
					return err
				}
			} else if result.Error == gorm.ErrRecordNotFound {
				// Create new
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

		// B. Ensure Role Exists (System level)
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

		// C. Check/Create Tenants and Assign Role
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

			// D. Assign Role to User via TenantUser
			for email, uID := range adminUserIDs {
				var tenantUser domain.TenantUser
				if err := tx.Where("tenant_id = ? AND user_id = ? AND role_id = ?", tenantID, uID, adminRole.ID).First(&tenantUser).Error; err != nil {
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
				}
			}
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Seeding failed: %v", err)
	}

	log.Println("Seeding completed successfully!")
}
