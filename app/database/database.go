// Package database opens Postgres and keeps the schema up to date with the versioned
// migrations in app/database/migrations (goose, history in goose_db_version).
package database

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open connects and applies pending migrations.
func Open(url string) (*gorm.DB, error) {
	db, err := connect(url)
	if err != nil {
		return nil, err
	}
	if err := Migrate(context.Background(), db); err != nil {
		return nil, err
	}
	return db, nil
}

func connect(url string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(url), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return db, nil
}
