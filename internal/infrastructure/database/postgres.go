package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/Elysian-Rebirth/backend-go/internal/config"
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

	maxOpen := cfg.Database.MaxOpenConns
	if maxOpen <= 0 {
		maxOpen = 50
	}
	maxIdle := cfg.Database.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 10
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)

	maxLifetime := cfg.Database.ConnMaxLifetime
	if maxLifetime <= 0 {
		maxLifetime = 15 * time.Minute
	}
	maxIdleTime := cfg.Database.ConnMaxIdleTime
	if maxIdleTime <= 0 {
		maxIdleTime = 5 * time.Minute
	}
	sqlDB.SetConnMaxLifetime(maxLifetime)
	sqlDB.SetConnMaxIdleTime(maxIdleTime)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Database connection established")

	return db, nil
}

// NOTE: Only use this in development! Use goose migrations in production
func AutoMigrate(db *gorm.DB) error {
	log.Println("Auto-migration disabled. Using pure Goose DDL migrations instead.")
	return nil
}

func Close(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// EnsurePartitions automatically creates partition tables for partitioned tables
// for the current month and the next month to avoid GORM write failures on range constraints.
func EnsurePartitions(db *gorm.DB) error {
	now := time.Now().UTC()
	
	// Create partitions for current month and next month
	months := []time.Time{
		time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC),
		time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0),
	}

	tables := []string{"token_usage_ledgers", "chat_messages", "enterprise_audit_logs"}

	for _, t := range months {
		year := t.Year()
		month := int(t.Month())
		
		startDate := t.Format("2006-01-02")
		endDate := t.AddDate(0, 1, 0).Format("2006-01-02")

		for _, table := range tables {
			partitionName := fmt.Sprintf("%s_y%04dm%02d", table, year, month)
			query := fmt.Sprintf(
				"CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')",
				partitionName, table, startDate, endDate,
			)
			log.Printf("Ensuring partition: %s", partitionName)
			if err := db.Exec(query).Error; err != nil {
				return fmt.Errorf("failed to create partition %s: %w", partitionName, err)
			}
		}
	}
	return nil
}
