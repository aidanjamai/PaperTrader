package scheduler

import (
	"context"
	"errors"
	"testing"

	"papertrader/internal/service/research/ingest"
)

// stubPipeline implements ingestPipeline for tests.
type stubPipeline struct {
	calls  []string // symbols IngestSymbol was called with, in order
	errFor string   // if non-empty, return an error for this symbol
}

func (s *stubPipeline) IngestSymbol(_ context.Context, symbol string, _ ingest.IngestOpts) (*ingest.IngestResult, error) {
	s.calls = append(s.calls, symbol)
	if s.errFor == symbol {
		return nil, errors.New("stub error")
	}
	return &ingest.IngestResult{DocumentsAdded: 1, EmbeddingsAdded: 2}, nil
}

func TestIngestScheduler_Construction(t *testing.T) {
	t.Run("valid inputs succeed", func(t *testing.T) {
		p := &stubPipeline{}
		s, err := NewIngestScheduler(p, []string{"AAPL", "MSFT"}, 3, "0 2 1 * *")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if s == nil {
			t.Fatal("expected non-nil scheduler")
		}
		_ = s.sched.Shutdown()
	})

	t.Run("empty universe returns error", func(t *testing.T) {
		p := &stubPipeline{}
		_, err := NewIngestScheduler(p, []string{}, 3, "0 2 1 * *")
		if err == nil {
			t.Fatal("expected error for empty universe, got nil")
		}
	})

	t.Run("empty schedule returns error", func(t *testing.T) {
		p := &stubPipeline{}
		_, err := NewIngestScheduler(p, []string{"AAPL"}, 3, "")
		if err == nil {
			t.Fatal("expected error for empty schedule, got nil")
		}
	})
}

func TestIngestScheduler_RunSucceedsWithStubPipeline(t *testing.T) {
	universe := []string{"AAPL", "MSFT", "NVDA"}
	p := &stubPipeline{}
	s, err := NewIngestScheduler(p, universe, 3, "0 2 1 * *")
	if err != nil {
		t.Fatalf("NewIngestScheduler: %v", err)
	}
	defer s.sched.Shutdown()

	s.run(context.Background())

	if len(p.calls) != len(universe) {
		t.Errorf("IngestSymbol called %d time(s), want %d", len(p.calls), len(universe))
	}
	for i, sym := range universe {
		if i >= len(p.calls) {
			break
		}
		if p.calls[i] != sym {
			t.Errorf("call[%d] = %q, want %q", i, p.calls[i], sym)
		}
	}
}

func TestIngestScheduler_RunContinuesOnTickerError(t *testing.T) {
	universe := []string{"AAPL", "FAIL", "NVDA"}
	p := &stubPipeline{errFor: "FAIL"}
	s, err := NewIngestScheduler(p, universe, 3, "0 2 1 * *")
	if err != nil {
		t.Fatalf("NewIngestScheduler: %v", err)
	}
	defer s.sched.Shutdown()

	s.run(context.Background())

	// All three symbols must have been attempted despite the middle one failing.
	if len(p.calls) != len(universe) {
		t.Errorf("IngestSymbol called %d time(s), want %d (should continue past error)", len(p.calls), len(universe))
	}
}
