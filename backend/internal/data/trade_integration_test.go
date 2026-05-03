//go:build integration

// Package data_test contains integration tests for the data layer that require
// a real PostgreSQL database. They are gated by the `integration` build tag and
// skip automatically when INTEGRATION_DB_URL is unset.
//
// This file uses the external test package (data_test) rather than the internal
// package (data) to avoid an import cycle: testutil imports data, so a file in
// package data cannot import testutil.
package data_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"papertrader/internal/data"
	"papertrader/internal/testutil"
)

// TestTradesAppendOnly_UpdateRejected inserts a trade via CreateTrade then
// attempts a raw UPDATE via SQL. The BEFORE UPDATE trigger installed in Phase 1
// must fire and return an error whose message contains "append-only".
func TestTradesAppendOnly_UpdateRejected(t *testing.T) {
	db := testutil.NewIntegrationDB(t)
	testutil.Truncate(t, db, "trades", "portfolio", "users")

	// Insert a minimal user row so the trade has a coherent user_id.
	userID := uuid.New().String()
	_, err := db.Exec(
		`INSERT INTO users (id, email, password, email_verified, created_via) VALUES ($1, $2, 'x', FALSE, 'email')`,
		userID, "append-update@example.com",
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	store := data.NewTradesStore(db)
	trade := &data.Trade{
		ID:       uuid.New().String(),
		UserID:   userID,
		Symbol:   "AAPL",
		Action:   "BUY",
		Quantity: 1,
		Price:    decimal.NewFromFloat(100.0),
		Status:   "COMPLETED",
	}
	if err := store.CreateTrade(context.Background(), trade); err != nil {
		t.Fatalf("CreateTrade: %v", err)
	}

	// Raw UPDATE — must be rejected by the trigger.
	_, err = db.Exec(`UPDATE trades SET status = 'CANCELLED' WHERE id = $1`, trade.ID)
	if err == nil {
		t.Fatal("expected UPDATE to be rejected by append-only trigger, got nil error")
	}
	if !containsAppendOnly(err.Error()) {
		t.Errorf("expected error to mention 'append-only', got: %v", err)
	}
}

// TestTradesAppendOnly_DeleteRejected inserts a trade then attempts a raw DELETE.
// The BEFORE DELETE trigger from Phase 1 must fire and return an error containing
// "append-only".
func TestTradesAppendOnly_DeleteRejected(t *testing.T) {
	db := testutil.NewIntegrationDB(t)
	testutil.Truncate(t, db, "trades", "portfolio", "users")

	userID := uuid.New().String()
	_, err := db.Exec(
		`INSERT INTO users (id, email, password, email_verified, created_via) VALUES ($1, $2, 'x', FALSE, 'email')`,
		userID, "append-delete@example.com",
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	store := data.NewTradesStore(db)
	trade := &data.Trade{
		ID:       uuid.New().String(),
		UserID:   userID,
		Symbol:   "TSLA",
		Action:   "SELL",
		Quantity: 2,
		Price:    decimal.NewFromFloat(250.0),
		Status:   "COMPLETED",
	}
	if err := store.CreateTrade(context.Background(), trade); err != nil {
		t.Fatalf("CreateTrade: %v", err)
	}

	// Raw DELETE — must be rejected by the trigger.
	_, err = db.Exec(`DELETE FROM trades WHERE id = $1`, trade.ID)
	if err == nil {
		t.Fatal("expected DELETE to be rejected by append-only trigger, got nil error")
	}
	if !containsAppendOnly(err.Error()) {
		t.Errorf("expected error to mention 'append-only', got: %v", err)
	}
}

// containsAppendOnly reports whether the error message from the DB trigger is
// present. The trigger raises: 'trades is append-only — % is not permitted'.
func containsAppendOnly(msg string) bool {
	return strings.Contains(msg, "append-only") || strings.Contains(msg, "append only")
}
