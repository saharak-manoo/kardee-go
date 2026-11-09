package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"

	"github.com/saharak/kardee-go/app/models"
)

func init() {
	goose.AddMigrationNoTxContext(upCreatePromotions, downCreatePromotions)
}

func upCreatePromotions(_ context.Context, db *sql.DB) error {
	gdb, err := gormDB(db)
	if err != nil {
		return err
	}
	return gdb.Migrator().AutoMigrate(&models.Promotion{})
}

func downCreatePromotions(_ context.Context, db *sql.DB) error {
	gdb, err := gormDB(db)
	if err != nil {
		return err
	}
	return gdb.Migrator().DropTable("promotions")
}
