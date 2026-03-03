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

	// 3. Define Admin User
	const adminEmail = "admin@gmail.com"
	const adminPasswordRaw = "admin12345"

	// 4. Hash Password
	passwordService := auth.NewPasswordService()
	hashedPassword, err := passwordService.HashPassword(adminPasswordRaw)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// 5. Transaction to Seed
	err = db.Transaction(func(tx *gorm.DB) error {
		// A. Check if user exists
		var existingUser domain.User
		result := tx.Where("email = ?", adminEmail).First(&existingUser)

		var userID uuid.UUID

		if result.Error == nil {
			// Update existing
			log.Printf("User %s exists. Updating password...", adminEmail)
			userID = existingUser.ID
			existingUser.PasswordHash = hashedPassword
			if err := tx.Save(&existingUser).Error; err != nil {
				return err
			}
		} else if result.Error == gorm.ErrRecordNotFound {
			// Create new
			log.Printf("Creating new admin user %s...", adminEmail)
			userID = uuid.New()
			newUser := domain.User{
				ID:           userID,
				FullName:     "Super Admin",
				Email:        adminEmail,
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

		// B. Check/Create Default Tenant
		var systemTenantID uuid.UUID
		if err := tx.Raw("SELECT id FROM tenants WHERE name = 'System'").Scan(&systemTenantID).Error; err != nil && err != gorm.ErrRecordNotFound {
			if systemTenantID == uuid.Nil {
				systemTenantID = uuid.New()
				if err := tx.Exec("INSERT INTO tenants (id, name, plan_tier) VALUES (?, 'System', 'enterprise')", systemTenantID).Error; err != nil {
					return err
				}
			}
		} else if systemTenantID == uuid.Nil {
			systemTenantID = uuid.New()
			if err := tx.Exec("INSERT INTO tenants (id, name, plan_tier) VALUES (?, 'System', 'enterprise')", systemTenantID).Error; err != nil {
				return err
			}
		}

		// C. Ensure Role Exists (System level)
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

		// D. Assign Role to User via TenantUser
		var tenantUser domain.TenantUser
		if err := tx.Where("tenant_id = ? AND user_id = ? AND role_id = ?", systemTenantID, userID, adminRole.ID).First(&tenantUser).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				log.Println("Assigning 'admin' role to user in System tenant...")
				tenantUser = domain.TenantUser{
					TenantID: systemTenantID,
					UserID:   userID,
					RoleID:   &adminRole.ID,
				}
				if err := tx.Create(&tenantUser).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Seeding failed: %v", err)
	}

	log.Println("Seeding completed successfully!")
}
