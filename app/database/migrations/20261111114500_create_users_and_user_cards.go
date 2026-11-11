package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"

	"github.com/saharak/kardee-go/app/models"
)

func init() {
	goose.AddMigrationNoTxContext(upCreateUsersAndUserCards, downCreateUsersAndUserCards)
}

func upCreateUsersAndUserCards(_ context.Context, db *sql.DB) error {
	gdb, err := gormDB(db)
	if err != nil {
		return err
	}
	return gdb.Migrator().AutoMigrate(&models.User{}, &models.UserCard{})
}

func downCreateUsersAndUserCards(_ context.Context, db *sql.DB) error {
	gdb, err := gormDB(db)
	if err != nil {
		return err
	}
	return gdb.Migrator().DropTable(&models.UserCard{}, &models.User{})
}

