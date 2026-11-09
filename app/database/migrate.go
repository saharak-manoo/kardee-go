package database

import (
	"context"
	"fmt"
	"log"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
	"gorm.io/gorm"

	_ "github.com/saharak/kardee-go/app/database/migrations" // registers the migrations
)

// provider runs the registered Go migrations. The session lock keeps api and jobs
// from migrating at the same time when they start together.
func provider(db *gorm.DB) (*goose.Provider, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(goose.DialectPostgres, sqlDB, nil, goose.WithSessionLocker(locker))
}

// Migrate applies every pending migration.
func Migrate(ctx context.Context, db *gorm.DB) error {
	p, err := provider(db)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	results, err := p.Up(ctx)
	for _, r := range results {
		log.Printf("[migrate] up %d %s", r.Source.Version, r.Duration)
	}
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// Rollback undoes the last applied migration.
func Rollback(ctx context.Context, db *gorm.DB) error {
	p, err := provider(db)
	if err != nil {
		return err
	}
	r, err := p.Down(ctx)
	if r != nil {
		log.Printf("[migrate] down %d %s", r.Source.Version, r.Duration)
	}
	return err
}

// Status lists every migration and whether it has run.
func Status(ctx context.Context, db *gorm.DB) ([]*goose.MigrationStatus, error) {
	p, err := provider(db)
	if err != nil {
		return nil, err
	}
	return p.Status(ctx)
}

// Connect opens Postgres without migrating (for the migrate command).
func Connect(url string) (*gorm.DB, error) {
	return connect(url)
}
