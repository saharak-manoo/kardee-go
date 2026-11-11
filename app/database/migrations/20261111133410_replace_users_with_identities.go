package migrations

import (
	"context"
	"database/sql"
	"errors"

	"github.com/pressly/goose/v3"

	"github.com/saharak/kardee-go/app/models"
)

func init() {
	goose.AddMigrationNoTxContext(upReplaceUsersWithIdentities, downReplaceUsersWithIdentities)
}

// The draft users table (20261111114500) keyed users by a LINE user ID sent by the
// client. It never held real users; replace it with UUID users + login identities.
func upReplaceUsersWithIdentities(_ context.Context, db *sql.DB) error {
	gdb, err := gormDB(db)
	if err != nil {
		return err
	}
	m := gdb.Migrator()
	if m.HasTable("users") && m.HasColumn("users", "line_user_id") {
		var n int64
		if err := gdb.Table("users").Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return errors.New("draft users table has rows; move them to user_identities by hand before this migration")
		}
		if err := m.DropTable("user_cards", "users"); err != nil {
			return err
		}
	}
	return m.AutoMigrate(&models.User{}, &models.UserIdentity{}, &models.UserCard{}, &models.Session{},
		&models.SearchLog{}, &models.AffiliateClick{})
}

// down drops the new tables; the draft shape is not brought back.
func downReplaceUsersWithIdentities(_ context.Context, db *sql.DB) error {
	gdb, err := gormDB(db)
	if err != nil {
		return err
	}
	return gdb.Migrator().DropTable(&models.AffiliateClick{}, &models.SearchLog{}, &models.Session{},
		&models.UserCard{}, &models.UserIdentity{}, &models.User{})
}
