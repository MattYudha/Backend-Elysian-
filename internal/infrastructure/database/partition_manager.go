package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-co-op/gocron/v2"
	"gorm.io/gorm"
)

// StartPartitionManager initializes a cron job to create partitions for the next month on the 25th.
func StartPartitionManager(db *gorm.DB) (gocron.Scheduler, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	// Schedule job to run on the 25th of every month at 00:00.
	_, err = s.NewJob(
		gocron.CronJob(
			"0 0 25 * *", // Minute, Hour, Day of month, Month, Day of week
			false,
		),
		gocron.NewTask(
			func() {
				log.Println("Starting scheduled monthly partition creation...")
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()

				if err := CreateNextMonthPartitions(ctx, db); err != nil {
					log.Printf("ERROR: Failed to create partitions: %v\n", err)
				} else {
					log.Println("Successfully created partitions for the next month.")
				}
			},
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule partition job: %w", err)
	}

	log.Println("Partition manager scheduled for the 25th of every month.")
	s.Start()
	return s, nil
}

// CreateNextMonthPartitions runs raw DDL to create table partitions for the following month.
func CreateNextMonthPartitions(ctx context.Context, db *gorm.DB) error {
	// Calculate the next month
	now := time.Now()
	// Next month target based on current time (even if run on the 25th, adding 1 month targets the next month correctly)
	nextMonth := now.AddDate(0, 1, 0)

	year := nextMonth.Year()
	month := nextMonth.Month()

	// e.g., "2026-04-01"
	startDateStr := fmt.Sprintf("%04d-%02d-01", year, month)
	
	// The start of the month after next
	monthAfterNext := nextMonth.AddDate(0, 1, 0)
	endDateStr := fmt.Sprintf("%04d-%02d-01", monthAfterNext.Year(), monthAfterNext.Month())

	// Partition Suffix e.g., "_y2026m04"
	suffix := fmt.Sprintf("_y%04dm%02d", year, month)

	// List of partitioned tables
	tables := []string{
		"token_usage_ledgers",
		"chat_messages",
		"enterprise_audit_logs",
	}

	for _, table := range tables {
		partitionName := fmt.Sprintf("%s%s", table, suffix)

		// Check if partition already exists in pg_class
		var exists bool
		checkQuery := `
			SELECT EXISTS (
				SELECT FROM pg_class c
				JOIN pg_namespace n ON n.oid = c.relnamespace
				WHERE c.relname = ?
			)
		`
		if err := db.WithContext(ctx).Raw(checkQuery, partitionName).Scan(&exists).Error; err != nil {
			return fmt.Errorf("failed to check existence for partition %s: %w", partitionName, err)
		}

		if exists {
			log.Printf("Partition %s already exists, skipping.\n", partitionName)
			continue
		}

		// Create partition
		query := fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s');",
			partitionName, table, startDateStr, endDateStr,
		)

		log.Printf("Executing DDL: %s", query)
		if err := db.WithContext(ctx).Exec(query).Error; err != nil {
			return fmt.Errorf("failed to create partition %s: %w", partitionName, err)
		}
	}

	return nil
}
