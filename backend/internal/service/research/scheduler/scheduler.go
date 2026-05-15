package scheduler

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/go-co-op/gocron/v2"

	"papertrader/internal/service/research/ingest"
)

// ingestPipeline is the subset of ingest.Pipeline consumed by IngestScheduler.
// Declared as an interface so tests can stub it without a real pipeline.
type ingestPipeline interface {
	IngestSymbol(ctx context.Context, symbol string, opts ingest.IngestOpts) (*ingest.IngestResult, error)
}

// IngestScheduler runs the research ingest pipeline on a cron schedule.
type IngestScheduler struct {
	pipeline   ingestPipeline
	universe   []string
	maxFilings int
	schedule   string
	sched      gocron.Scheduler
}

// NewIngestScheduler constructs an IngestScheduler. universe must be non-empty
// and schedule must be a valid 5-field cron expression.
func NewIngestScheduler(pipeline ingestPipeline, universe []string, maxFilings int, schedule string) (*IngestScheduler, error) {
	if len(universe) == 0 {
		return nil, schedulerError("universe must not be empty")
	}
	if strings.TrimSpace(schedule) == "" {
		return nil, schedulerError("schedule must not be empty")
	}

	sched, err := gocron.NewScheduler()
	if err != nil {
		return nil, err
	}

	s := &IngestScheduler{
		pipeline:   pipeline,
		universe:   universe,
		maxFilings: maxFilings,
		schedule:   schedule,
		sched:      sched,
	}
	return s, nil
}

type schedulerError string

func (e schedulerError) Error() string { return "ingest scheduler: " + string(e) }

// Start registers the cron job and begins the scheduler. Idempotent — safe to
// call once per process lifetime.
func (s *IngestScheduler) Start() error {
	_, err := s.sched.NewJob(
		gocron.CronJob(s.schedule, false),
		gocron.NewTask(s.run, context.Background()),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	)
	if err != nil {
		return err
	}
	s.sched.Start()
	slog.Info("ingest scheduler: started", "schedule", s.schedule, "universe_size", len(s.universe))
	return nil
}

// Stop drains in-flight jobs (up to the deadline in ctx) and shuts the
// scheduler down. Called from main.go's graceful shutdown before DB close so
// any active ingest can finish its current DB writes.
func (s *IngestScheduler) Stop(ctx context.Context) error {
	slog.Info("ingest scheduler: stopping")
	return s.sched.ShutdownWithContext(ctx)
}

// run is the cron job function. It iterates every symbol in the universe,
// calling IngestSymbol for each. A failed ticker is logged and skipped; it
// never aborts the rest of the run.
func (s *IngestScheduler) run(ctx context.Context) {
	start := time.Now()
	slog.Info("ingest scheduler: starting run", "universe_size", len(s.universe))

	opts := ingest.IngestOpts{
		FormTypes:  []string{"10-K", "10-Q", "8-K"},
		MaxFilings: s.maxFilings,
		Force:      false,
	}

	var totals ingest.IngestResult
	failed := 0
	for _, sym := range s.universe {
		r, err := s.pipeline.IngestSymbol(ctx, sym, opts)
		if err != nil {
			slog.Warn("ingest scheduler: ticker failed", "symbol", sym, "err", err)
			failed++
			continue
		}
		slog.Info("ingest scheduler: ticker complete",
			"symbol", sym,
			"docs_added", r.DocumentsAdded,
			"chunks_added", r.ChunksAdded,
			"embeds_added", r.EmbeddingsAdded,
			"skipped", r.Skipped,
		)
		totals.DocumentsAdded += r.DocumentsAdded
		totals.ChunksAdded += r.ChunksAdded
		totals.EmbeddingsAdded += r.EmbeddingsAdded
		totals.Skipped += r.Skipped
	}

	elapsed := time.Since(start)
	slog.Info("ingest scheduler: run complete",
		"tickers", len(s.universe),
		"failed", failed,
		"docs", totals.DocumentsAdded,
		"embeds", totals.EmbeddingsAdded,
		"duration_ms", elapsed.Milliseconds(),
	)
}

// SplitTickerUniverse splits a comma-separated ticker string into an uppercase,
// trimmed slice with empty entries removed. Shared by main.go scheduler wiring.
func SplitTickerUniverse(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, strings.ToUpper(t))
		}
	}
	return out
}
