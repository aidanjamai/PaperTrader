// cmd/reconcile is a CLI tool that compares the trade ledger against the
// materialized portfolio table and reports any discrepancies.
//
// Usage:
//
//	go run ./cmd/reconcile              # full sweep, table output
//	go run ./cmd/reconcile -user <id>   # single user
//	go run ./cmd/reconcile -json        # JSON output (for CI)
//
// Exit codes: 0 = clean, 1 = discrepancies found, 2 = error.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"papertrader/internal/config"
	"papertrader/internal/data"
	"papertrader/internal/service"
)

func main() {
	// Configure shopspring/decimal to serialize as unquoted JSON numbers.
	// Must run before any decimal value is marshalled (the -json flag below
	// emits Discrepancy structs containing decimal.Decimal fields).
	data.EnableUnquotedDecimalJSON()

	userFlag := flag.String("user", "", "reconcile a single user ID (default: all users)")
	jsonFlag := flag.Bool("json", false, "output discrepancies as JSON")
	flag.Parse()

	// Load env vars (non-fatal if .env is absent — container environments use
	// system env vars directly).
	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file found, using system environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "reconcile: invalid configuration: %v\n", err)
		os.Exit(2)
	}
	config.SetupLogger(cfg.Environment, cfg.LogLevel)

	db, err := config.ConnectPostgreSQL(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to connect to database: %v\n", err)
		os.Exit(2)
	}
	defer db.Close()

	portfolioStore := data.NewPortfolioStore(db)
	tradesStore := data.NewTradesStore(db)
	svc := service.NewReconcileService(db, portfolioStore, tradesStore)

	if *userFlag != "" {
		// Single-user mode.
		discrepancies, err := svc.Reconcile(context.Background(), *userFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: reconcile(%s): %v\n", *userFlag, err)
			os.Exit(2)
		}
		result := map[string][]service.Discrepancy{}
		if len(discrepancies) > 0 {
			result[*userFlag] = discrepancies
		}
		printResult(result, *jsonFlag)
		if len(result) > 0 {
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Full-sweep mode.
	result, err := svc.ReconcileAll(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reconcile all: %v\n", err)
		os.Exit(2)
	}
	printResult(result, *jsonFlag)
	if len(result) > 0 {
		os.Exit(1)
	}
	os.Exit(0)
}

func printResult(result map[string][]service.Discrepancy, asJSON bool) {
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "error encoding JSON: %v\n", err)
		}
		return
	}

	if len(result) == 0 {
		fmt.Println("No discrepancies found.")
		return
	}

	fmt.Printf("%-36s  %-8s  %-20s  %-12s  %-12s  %-12s  %-12s\n",
		"user_id", "symbol", "kind", "portfolio_qty", "ledger_qty", "portfolio_avg", "ledger_avg")
	fmt.Println("--------------------------------------------------------------------------------------------------------------------------------------")
	for _, discrepancies := range result {
		for _, d := range discrepancies {
			fmt.Printf("%-36s  %-8s  %-20s  %-12d  %-12d  %-12s  %-12s\n",
				d.UserID, d.Symbol, d.Kind, d.PortfolioQty, d.LedgerQty, d.PortfolioAvg.StringFixed(2), d.LedgerAvg.StringFixed(2))
		}
	}
}
