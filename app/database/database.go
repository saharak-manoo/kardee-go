// Package database opens Postgres and keeps the schema up to date.
package database

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/saharak/kardee-go/app/models"
)

// Open connects and migrates. AutoMigrate only adds; switch to versioned migrations
// once the schema settles or a column needs to change type.
func Open(url string) (*gorm.DB, error) {
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
	if err := db.AutoMigrate(&models.Promotion{}, &models.CrawlRun{}); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}
