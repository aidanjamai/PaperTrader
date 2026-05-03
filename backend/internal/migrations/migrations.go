package migrations

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed sql/*.sql
var migrationFS embed.FS

func newMigrate(db *sql.DB) (*migrate.Migrate, error) {
	src, err := iofs.New(migrationFS, "sql")
	if err != nil {
		return nil, fmt.Errorf("migrations: source: %w", err)
	}

	drv, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("migrations: driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", drv)
	if err != nil {
		return nil, fmt.Errorf("migrations: instance: %w", err)
	}

	return m, nil
}

// Run applies all pending up migrations. It is a no-op if the schema is
// already at the latest version.
func Run(db *sql.DB) error {
	m, err := newMigrate(db)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrations: up: %w", err)
	}
	return nil
}

// Up is an alias for Run, exposed for the CLI.
func Up(db *sql.DB) error {
	return Run(db)
}

// Down rolls back one migration step.
func Down(db *sql.DB) error {
	m, err := newMigrate(db)
	if err != nil {
		return err
	}
	if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrations: down: %w", err)
	}
	return nil
}

// Version returns the current schema version and whether the state is dirty.
func Version(db *sql.DB) (uint, bool, error) {
	m, err := newMigrate(db)
	if err != nil {
		return 0, false, err
	}
	v, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("migrations: version: %w", err)
	}
	return v, dirty, nil
}

// Force sets the migration version manually without running any SQL. Used to
// recover from a dirty state after a failed migration was manually repaired.
func Force(db *sql.DB, version int) error {
	m, err := newMigrate(db)
	if err != nil {
		return err
	}
	if err := m.Force(version); err != nil {
		return fmt.Errorf("migrations: force: %w", err)
	}
	return nil
}
