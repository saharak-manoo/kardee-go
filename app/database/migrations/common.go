// Package migrations holds the schema history as goose Go migrations, one file per
// change, named <timestamp>_<what>.go. Never edit a migration that has run anywhere;
// add a new one. Create one with: go run ./cmd/migrate create <name>
package migrations

import (
	"database/sql"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// gormDB wraps goose's connection so migrations can use the GORM Migrator.
func gormDB(db *sql.DB) (*gorm.DB, error) {
	return gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
}
