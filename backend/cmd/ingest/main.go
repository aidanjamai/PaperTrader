// cmd/ingest pulls SEC filings for one or more tickers, chunks them, embeds via
// Voyage AI, and stores results in Postgres/pgvector.
//
// Usage:
//
//	go run ./cmd/ingest                              # uses RESEARCH_TICKER_UNIVERSE (default top 10)
//	go run ./cmd/ingest -symbol=AAPL                 # single ticker
//	go run ./cmd/ingest -symbol=AAPL,MSFT,NVDA       # explicit list
//	go run ./cmd/ingest -max-filings=3 -force        # all universe tickers, redo all filings
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"papertrader/internal/config"
	"papertrader/internal/data"
	"papertrader/internal/service/research"
	"papertrader/internal/service/research/ingest"
)

func main() {
	symbol := flag.String("symbol", "", "ticker(s) to ingest (comma-separated). If empty, uses RESEARCH_TICKER_UNIVERSE.")
	maxFilings := flag.Int("max-filings", 3, "maximum number of filings to process per ticker")
	formTypesFlag := flag.String("form-types", "10-K,10-Q,8-K", "comma-separated list of form types")
	force := flag.Bool("force", false, "re-ingest documents that already exist")
	flag.Parse()

	loaded := false
	for _, path := range []string{".env", "../.env"} {
		if err := godotenv.Load(path); err == nil {
			slog.Info("loaded env file", "path", path)
			loaded = true
			break
		}
	}
	if !loaded {
		slog.Info("no .env file found, using system environment variables")
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ingest: invalid configuration: %v\n", err)
		os.Exit(1)
	}

	if cfg.VoyageAPIKey == "" {
		fmt.Fprintln(os.Stderr, "ingest: VOYAGE_API_KEY is required")
		os.Exit(1)
	}
	if cfg.SecUserAgent == "" {
		fmt.Fprintln(os.Stderr, "ingest: SEC_USER_AGENT is required")
		os.Exit(1)
	}

	config.SetupLogger(cfg.Environment, cfg.LogLevel)

	db, err := config.ConnectPostgreSQL(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ingest: database connection failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	redisClient, err := config.ConnectRedis(cfg)
	if err != nil {
		slog.Warn("Redis unavailable; embedding cache disabled", "err", err)
		redisClient = nil
	}

	var embedder research.Embedder
	voyage := research.NewVoyageEmbedder(cfg.VoyageAPIKey)
	if redisClient != nil {
		embedder = research.NewCachedEmbedder(voyage, redisClient)
	} else {
		embedder = voyage
	}

	docsStore := data.NewDocumentsStore(db)
	chunksStore := data.NewChunksStore(db)
	embeddingsStore := data.NewEmbeddingsStore(db)

	edgarClient := ingest.NewEdgarClient(cfg.SecUserAgent)
	pipeline := ingest.NewPipeline(edgarClient, embedder, docsStore, chunksStore, embeddingsStore)

	formTypes := strings.Split(*formTypesFlag, ",")
	for i, ft := range formTypes {
		formTypes[i] = strings.TrimSpace(ft)
	}

	opts := ingest.IngestOpts{
		FormTypes:  formTypes,
		MaxFilings: *maxFilings,
		Force:      *force,
	}

	symbolsRaw := *symbol
	if symbolsRaw == "" {
		symbolsRaw = cfg.ResearchTickerUniverse
	}
	symbols := splitAndTrim(symbolsRaw)
	if len(symbols) == 0 {
		fmt.Fprintln(os.Stderr, "ingest: no symbols to process (set -symbol or RESEARCH_TICKER_UNIVERSE)")
		os.Exit(1)
	}

	fmt.Printf("ingesting %d ticker(s): %s\n", len(symbols), strings.Join(symbols, ", "))
	fmt.Printf("form types: %s, max-filings: %d, force: %v\n\n", *formTypesFlag, *maxFilings, *force)

	var totals ingest.IngestResult
	failed := 0
	for i, sym := range symbols {
		fmt.Printf("[%d/%d] %s ...\n", i+1, len(symbols), sym)
		result, err := pipeline.IngestSymbol(context.Background(), sym, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  failed: %v\n", err)
			failed++
			continue
		}
		fmt.Printf("  docs=%d chunks=%d embeds=%d skipped=%d\n",
			result.DocumentsAdded, result.ChunksAdded, result.EmbeddingsAdded, result.Skipped)
		totals.DocumentsAdded += result.DocumentsAdded
		totals.ChunksAdded += result.ChunksAdded
		totals.EmbeddingsAdded += result.EmbeddingsAdded
		totals.Skipped += result.Skipped
	}

	fmt.Printf("\nIngest complete (%d ticker(s), %d failed):\n", len(symbols), failed)
	fmt.Printf("  Documents added:   %d\n", totals.DocumentsAdded)
	fmt.Printf("  Chunks added:      %d\n", totals.ChunksAdded)
	fmt.Printf("  Embeddings added:  %d\n", totals.EmbeddingsAdded)
	fmt.Printf("  Skipped (exists):  %d\n", totals.Skipped)
	if failed > 0 {
		os.Exit(1)
	}
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, strings.ToUpper(t))
		}
	}
	return out
}
