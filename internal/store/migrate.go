package store

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate brings the database up to the current schema. It is idempotent.
//
// The migrations are embedded rather than shipped alongside the binary: a
// single self-contained executable is the whole distribution promise, and a
// library that cannot open because its migration files went missing would be
// a spectacular way to break it.
func Migrate(db *sql.DB) error {
	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set migration dialect: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	return nil
}
