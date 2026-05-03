// cmd/migrate is an out-of-band migration CLI for PaperTrader.
//
// Usage:
//
//	go run ./cmd/migrate up               # apply all pending up migrations
//	go run ./cmd/migrate down             # roll back one migration
//	go run ./cmd/migrate version          # print current schema version + dirty flag
//	go run ./cmd/migrate force <version>  # set version manually (recover from dirty state)
package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"papertrader/internal/config"
	"papertrader/internal/migrations"
)

func main() {
	if err := godotenv.Load(); err != nil {
		slog.Info("no .env file found, using system environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: invalid configuration: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	db, dbErr := config.ConnectPostgreSQL(cfg)
	if dbErr != nil {
		fmt.Fprintf(os.Stderr, "migrate: failed to connect to database: %v\n", dbErr)
		os.Exit(1)
	}
	defer db.Close()

	cmd := os.Args[1]

	switch cmd {
	case "up":
		if err := migrations.Up(db); err != nil {
			fmt.Fprintf(os.Stderr, "migrate up: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("migrate up: done")

	case "down":
		if err := migrations.Down(db); err != nil {
			fmt.Fprintf(os.Stderr, "migrate down: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("migrate down: done")

	case "version":
		v, dirty, err := migrations.Version(db)
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrate version: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("version: %d  dirty: %v\n", v, dirty)

	case "force":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "migrate force: <version> argument required")
			os.Exit(1)
		}
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "migrate force: invalid version %q: %v\n", os.Args[2], err)
			os.Exit(1)
		}
		if err := migrations.Force(db, v); err != nil {
			fmt.Fprintf(os.Stderr, "migrate force: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("migrate force: version set to %d\n", v)

	default:
		fmt.Fprintf(os.Stderr, "migrate: unknown command %q\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  go run ./cmd/migrate up               apply all pending up migrations
  go run ./cmd/migrate down             roll back one migration
  go run ./cmd/migrate version          print current schema version + dirty flag
  go run ./cmd/migrate force <version>  set version manually (recover from dirty state)`)
}
