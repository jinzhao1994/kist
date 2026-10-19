package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// migrationProvider avoids process-global Goose configuration when libraries
// and historical scratch databases are opened concurrently.
func migrationProvider(db *sql.DB) (*goose.Provider, error) {
	source, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(goose.DialectSQLite3, db, source)
}

// Migrate brings the database up to the embedded schema. The caller serializes
// access to a live library with its write lock. Each migration is transactional.
func Migrate(db *sql.DB) error {
	p, err := migrationProvider(db)
	if err != nil {
		return err
	}
	ctx := context.Background()
	current, target, err := p.GetVersions(ctx)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current > target {
		return fmt.Errorf("database schema %d is newer than supported schema %d; use a newer kist version", current, target)
	}
	if current == target {
		return nil
	}
	slog.Info("upgrading database schema", "from", current, "to", target)
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	slog.Info("database schema upgrade complete", "version", target)
	return nil
}
