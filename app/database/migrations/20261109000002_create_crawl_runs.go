package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"

	"github.com/saharak/kardee-go/app/models"
)

func init() {
	goose.AddMigrationNoTxContext(upCreateCrawlRuns, downCreateCrawlRuns)
}

func upCreateCrawlRuns(_ context.Context, db *sql.DB) error {
	gdb, err := gormDB(db)
	if err != nil {
		return err
	}
	return gdb.Migrator().AutoMigrate(&models.CrawlRun{})
}

func downCreateCrawlRuns(_ context.Context, db *sql.DB) error {
	gdb, err := gormDB(db)
	if err != nil {
		return err
	}
	return gdb.Migrator().DropTable("crawl_runs")
}
