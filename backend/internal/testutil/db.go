// Package testutil provides helpers for integration tests that run against a
// real PostgreSQL database.
//
// Integration helpers. Tests using these MUST be gated by INTEGRATION_DB_URL
// and skip cleanly when it is unset, so the default `go test ./...` run on a
// developer machine without Postgres still passes.
//
// Usage:
//
//	db := testutil.NewIntegrationDB(t)
//	testutil.Truncate(t, db, "trades", "portfolio", "users")
package testutil

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"

	"papertrader/internal/migrations"
)

// NewIntegrationDB returns a *sql.DB connected to the database identified by
// the INTEGRATION_DB_URL environment variable. If the variable is unset the
// test is skipped with t.Skip so the suite is safe to run without Postgres.
//
// After connecting, migrations.Run is called so that every trigger, index, and
// schema migration exists before any test row is inserted.
//
// db.Close is registered on t.Cleanup; callers do not need to close it.
func NewIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()

	url := os.Getenv("INTEGRATION_DB_URL")
	if url == "" {
		t.Skip("INTEGRATION_DB_URL not set — skipping integration test")
	}

	db, err := sql.Open("postgres", url)
	if err != nil {
		t.Fatalf("testutil.NewIntegrationDB: sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Ping(); err != nil {
		t.Fatalf("testutil.NewIntegrationDB: ping: %v", err)
	}

	// Run all schema migrations so triggers and indexes exist.
	if err := migrations.Run(db); err != nil {
		t.Fatalf("testutil.NewIntegrationDB: migrations.Run: %v", err)
	}

	return db
}

// Truncate clears the named tables for test isolation. Because the trades table
// has BEFORE DELETE triggers that reject all deletes (Phase 1 append-only
// enforcement), this helper temporarily disables both append-only triggers,
// runs TRUNCATE ... RESTART IDENTITY CASCADE, then re-enables the triggers.
//
// This is the only sanctioned escape hatch for the append-only trigger: it is
// used exclusively in integration test setup/teardown and must never be called
// from production code paths.
// truncateAllowlist is the set of table names that Truncate is permitted to clear.
// Add a name here when a new table needs test-isolation support.
var truncateAllowlist = map[string]bool{
	"users":     true,
	"trades":    true,
	"portfolio": true,
}

func Truncate(t *testing.T, db *sql.DB, tables ...string) {
	t.Helper()

	if len(tables) == 0 {
		return
	}

	// Reject any name not in the allowlist to prevent accidental data loss.
	for _, name := range tables {
		if !truncateAllowlist[name] {
			t.Fatalf("Truncate: %q is not in the allowlist", name)
		}
	}

	// Disable the append-only triggers so TRUNCATE can remove trade rows.
	disableSQL := `
		ALTER TABLE trades DISABLE TRIGGER trades_no_update;
		ALTER TABLE trades DISABLE TRIGGER trades_no_delete;`
	if _, err := db.Exec(disableSQL); err != nil {
		t.Fatalf("Truncate: disable triggers: %v", err)
	}

	// Build a single TRUNCATE statement for all requested tables.
	tableList := strings.Join(tables, ", ")
	truncateSQL := fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE", tableList)
	if _, err := db.Exec(truncateSQL); err != nil {
		// Re-enable before failing so we don't leave the DB in a broken state.
		_, _ = db.Exec(`
			ALTER TABLE trades ENABLE TRIGGER trades_no_update;
			ALTER TABLE trades ENABLE TRIGGER trades_no_delete;`)
		t.Fatalf("Truncate: %v", err)
	}

	// Re-enable the append-only triggers.
	enableSQL := `
		ALTER TABLE trades ENABLE TRIGGER trades_no_update;
		ALTER TABLE trades ENABLE TRIGGER trades_no_delete;`
	if _, err := db.Exec(enableSQL); err != nil {
		t.Fatalf("Truncate: re-enable triggers: %v", err)
	}
}
