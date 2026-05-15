// cmd/retrieve embeds a query and prints the top matching chunks from
// pgvector. A smoke test for the retrieval path before any LLM is wired in.
//
// Usage:
//
//	go run ./cmd/retrieve -q "supply chain risks"
//	go run ./cmd/retrieve -q "AI infrastructure investment" -symbol=NVDA,GOOGL -k=10
//	go run ./cmd/retrieve -q "regulatory headwinds" -min-score=0.4
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"papertrader/internal/config"
	"papertrader/internal/data"
	"papertrader/internal/service/research"
)

func main() {
	query := flag.String("q", "", "query text (required)")
	symbol := flag.String("symbol", "", "filter to ticker(s), comma-separated (optional)")
	k := flag.Int("k", 5, "top-k chunks to return")
	minScore := flag.Float64("min-score", 0.4, "minimum cosine similarity (0..1)")
	flag.Parse()

	if *query == "" {
		fmt.Fprintln(os.Stderr, "retrieve: -q is required")
		flag.Usage()
		os.Exit(1)
	}

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
		fmt.Fprintf(os.Stderr, "retrieve: invalid configuration: %v\n", err)
		os.Exit(1)
	}
	if cfg.VoyageAPIKey == "" {
		fmt.Fprintln(os.Stderr, "retrieve: VOYAGE_API_KEY is required")
		os.Exit(1)
	}

	config.SetupLogger(cfg.Environment, cfg.LogLevel)

	db, err := config.ConnectPostgreSQL(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "retrieve: database connection failed: %v\n", err)
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

	embeddingsStore := data.NewEmbeddingsStore(db)
	svc := research.NewRetrievalService(embedder, embeddingsStore)

	var symbols []string
	if *symbol != "" {
		for _, s := range strings.Split(*symbol, ",") {
			if t := strings.TrimSpace(s); t != "" {
				symbols = append(symbols, strings.ToUpper(t))
			}
		}
	}

	start := time.Now()
	hits, err := svc.Retrieve(context.Background(), *query, research.RetrieveOpts{
		Symbols:  symbols,
		K:        *k,
		MinScore: *minScore,
	})
	elapsed := time.Since(start)
	if err != nil {
		fmt.Fprintf(os.Stderr, "retrieve: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nQuery: %q\n", *query)
	if len(symbols) > 0 {
		fmt.Printf("Filter: symbols=%v\n", symbols)
	}
	fmt.Printf("Latency: %dms (embed + ANN search + filter)\n", elapsed.Milliseconds())
	fmt.Printf("Results: %d hits (k=%d, min_score=%.2f)\n\n", len(hits), *k, *minScore)

	if len(hits) == 0 {
		fmt.Println("No matches above min-score threshold. Try lowering -min-score or broadening the query.")
		return
	}

	for i, h := range hits {
		filed := "unknown date"
		if h.FiledAt != nil {
			filed = h.FiledAt.Format("2006-01-02")
		}
		section := h.Section
		if section == "" {
			section = "(no section)"
		}
		fmt.Printf("[%d] score=%.3f  %s  filed=%s  section=%s\n", i+1, h.Score, h.Symbol, filed, section)
		fmt.Printf("    url:     %s\n", h.SourceURL)
		fmt.Printf("    chunk:   %s\n", h.ChunkID)
		fmt.Printf("    excerpt: %s\n\n", truncate(h.Text, 300))
	}
}

func truncate(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
