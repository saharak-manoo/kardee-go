// Command migrate manages the database schema.
//
//	migrate up             apply pending migrations (api and jobs also do this on start)
//	migrate down           undo the last migration
//	migrate status         list migrations and whether they ran
//	migrate create <name>  new migration file, e.g. create add_merchants
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/saharak/kardee-go/app/config"
	"github.com/saharak/kardee-go/app/database"
)

const dir = "app/database/migrations"

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	if err := run(os.Args[1], os.Args[2:]); err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: migrate up | down | status | create <name>")
	os.Exit(2)
}

func run(cmd string, args []string) error {
	if cmd == "create" {
		if len(args) != 1 {
			usage()
		}
		return create(args[0], time.Now())
	}
	db, err := database.Connect(config.Load().DatabaseURL)
	if err != nil {
		return err
	}
	ctx := context.Background()
	switch cmd {
	case "up":
		return database.Migrate(ctx, db)
	case "down":
		return database.Rollback(ctx, db)
	case "status":
		list, err := database.Status(ctx, db)
		if err != nil {
			return err
		}
		for _, s := range list {
			at := "pending"
			if !s.AppliedAt.IsZero() {
				at = s.AppliedAt.Format("2006-01-02 15:04")
			}
			fmt.Printf("%-16s  %s\n", at, filepath.Base(s.Source.Path))
		}
		return nil
	}
	usage()
	return nil
}

var nameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func create(name string, now time.Time) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("name must be snake_case, got %q", name)
	}
	file := filepath.Join(dir, now.Format("20060102150405")+"_"+name+".go")
	fn := camel(name)
	src := fmt.Sprintf(`package migrations

import (
	"context"
	"database/sql"

	"github.com/pressly/goose/v3"
)

func init() {
	goose.AddMigrationNoTxContext(up%[1]s, down%[1]s)
}

func up%[1]s(_ context.Context, db *sql.DB) error {
	gdb, err := gormDB(db)
	if err != nil {
		return err
	}
	_ = gdb // TODO
	return nil
}

func down%[1]s(_ context.Context, db *sql.DB) error {
	gdb, err := gormDB(db)
	if err != nil {
		return err
	}
	_ = gdb // TODO
	return nil
}
`, fn)
	if err := os.WriteFile(file, []byte(src), 0o600); err != nil {
		return err
	}
	fmt.Println("created", file)
	return nil
}

func camel(s string) string {
	parts := strings.Split(s, "_")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}
