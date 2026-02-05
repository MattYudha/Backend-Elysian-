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
	const adminPasswordRaw = "admin"

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

		var userID string

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
			userID = "usr_" + uuid.New().String() // Generating ID similar to known prefix if any, or UUID
			newUser := domain.User{
				ID:           userID,
				Name:         "Super Admin",
				Email:        adminEmail,
				PasswordHash: hashedPassword,
				IsActive:     true,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}
			if err := tx.Create(&newUser).Error; err != nil {
				return err
			}
		} else {
			return result.Error
		}

		// B. Ensure Role Exists
		var adminRole domain.Role
		if err := tx.Where("name = ?", "admin").First(&adminRole).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				log.Println("Creating 'admin' role...")
				desc := "Administrator with full access"
				adminRole = domain.Role{
					ID:          "role_" + uuid.New().String(),
					Name:        "admin",
					Description: &desc,
				}
				if err := tx.Create(&adminRole).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}

		// C. Assign Role to User
		var userRole domain.UserRole
		if err := tx.Where("user_id = ? AND role_id = ?", userID, adminRole.ID).First(&userRole).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				log.Println("Assigning 'admin' role to user...")
				userRole = domain.UserRole{
					UserID: userID,
					RoleID: adminRole.ID,
				}
				if err := tx.Create(&userRole).Error; err != nil {
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
