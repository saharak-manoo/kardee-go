package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"

	"github.com/saharak/kardee-go/app/models"
)

func init() {
	goose.AddMigrationNoTxContext(upCreatePromotionMerchants, downCreatePromotionMerchants)
}

func upCreatePromotionMerchants(_ context.Context, db *sql.DB) error {
	gdb, err := gormDB(db)
	if err != nil {
		return err
	}
	return gdb.Migrator().AutoMigrate(&models.PromotionMerchant{})
}

func downCreatePromotionMerchants(_ context.Context, db *sql.DB) error {
	gdb, err := gormDB(db)
	if err != nil {
		return err
	}
	return gdb.Migrator().DropTable(&models.PromotionMerchant{})
}
