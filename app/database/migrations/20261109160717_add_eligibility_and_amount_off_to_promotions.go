package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"

	"github.com/saharak/kardee-go/app/models"
)

func init() {
	goose.AddMigrationNoTxContext(upAddEligibilityAndAmountOffToPromotions, downAddEligibilityAndAmountOffToPromotions)
}

var eligibilityColumns = []string{"AmountOff", "Eligibility"}

func upAddEligibilityAndAmountOffToPromotions(_ context.Context, db *sql.DB) error {
	gdb, err := gormDB(db)
	if err != nil {
		return err
	}
	m := gdb.Migrator()
	for _, col := range eligibilityColumns {
		if !m.HasColumn(&models.Promotion{}, col) {
			if err := m.AddColumn(&models.Promotion{}, col); err != nil {
				return err
			}
		}
	}
	return nil
}

func downAddEligibilityAndAmountOffToPromotions(_ context.Context, db *sql.DB) error {
	gdb, err := gormDB(db)
	if err != nil {
		return err
	}
	m := gdb.Migrator()
	for _, col := range eligibilityColumns {
		if m.HasColumn(&models.Promotion{}, col) {
			if err := m.DropColumn(&models.Promotion{}, col); err != nil {
				return err
			}
		}
	}
	return nil
}
