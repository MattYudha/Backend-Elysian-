package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Elysian-Rebirth/backend-go/internal/config"
	"github.com/Elysian-Rebirth/backend-go/internal/domain"
)

// NewPostgresDB creates a new PostgreSQL database connection using GORM
func NewPostgresDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := cfg.GetDatabaseDSN()

	var gormLogger logger.Interface
	if cfg.IsDevelopment() {
		gormLogger = logger.Default.LogMode(logger.Info)
	} else {
		gormLogger = logger.Default.LogMode(logger.Error)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 gormLogger,
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.Database.ConnMaxIdleTime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connection established")

	return db, nil
}

// NOTE: Only use this in development! Use goose migrations in production
func AutoMigrate(db *gorm.DB) error {
	log.Println("Running auto-migration...")

	err := db.AutoMigrate(
		&domain.User{},
		&domain.Role{},
		&domain.UserRole{},
	)

	if err != nil {
		return fmt.Errorf("auto-migration failed: %w", err)
	}

	log.Println("Auto-migration completed")
	return nil
}

func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
