package research

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"papertrader/internal/api/auth"
	svcresearch "papertrader/internal/service/research"
)

// stubAnswerSvc satisfies the answerer interface for handler tests.
type stubAnswerSvc struct {
	answer *svcresearch.Answer
	err    error
}

func (s *stubAnswerSvc) Ask(_ context.Context, _, _ string, _ svcresearch.AskOpts) (*svcresearch.Answer, error) {
	return s.answer, s.err
}

func postAsk(t *testing.T, h *Handler, body string, userID string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/research/ask", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		req = req.WithContext(auth.WithUserID(req.Context(), userID))
	}
	w := httptest.NewRecorder()
	h.Ask(w, req)
	return w
}

func TestAsk_400OnEmptyQuery(t *testing.T) {
	h := NewHandler(&stubAnswerSvc{})
	w := postAsk(t, h, `{"query":""}`, "user-1")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAsk_400OnTooLongQuery(t *testing.T) {
	h := NewHandler(&stubAnswerSvc{})
	long := strings.Repeat("x", 2001)
	body, _ := json.Marshal(map[string]string{"query": long})
	w := postAsk(t, h, string(body), "user-1")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestAsk_200OnHappyPath(t *testing.T) {
	expected := &svcresearch.Answer{
		QueryID:   "abc123",
		Answer:    "AAPL revenue was $X.",
		Citations: []svcresearch.Citation{{ChunkID: "c1", SourceURL: "https://x.com", Excerpt: "text", Score: 0.9}},
		Refused:   false,
		LatencyMS: 42,
	}
	h := NewHandler(&stubAnswerSvc{answer: expected})
	body, _ := json.Marshal(map[string]string{"query": "What is AAPL revenue?"})
	w := postAsk(t, h, string(body), "user-1")
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	var got svcresearch.Answer
	if err := json.NewDecoder(bytes.NewReader(w.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.QueryID != expected.QueryID {
		t.Errorf("QueryID = %q, want %q", got.QueryID, expected.QueryID)
	}
	if got.Answer != expected.Answer {
		t.Errorf("Answer = %q, want %q", got.Answer, expected.Answer)
	}
	if len(got.Citations) != 1 {
		t.Errorf("Citations len = %d, want 1", len(got.Citations))
	}
}

func TestAsk_200OnRefusal(t *testing.T) {
	refused := &svcresearch.Answer{
		QueryID:       "xyz",
		Refused:       true,
		RefusalReason: "forward_looking",
		Citations:     []svcresearch.Citation{},
		LatencyMS:     5,
	}
	h := NewHandler(&stubAnswerSvc{answer: refused})
	body, _ := json.Marshal(map[string]string{"query": "Will AAPL go up?"})
	w := postAsk(t, h, string(body), "user-1")
	if w.Code != http.StatusOK {
		t.Errorf("refusal should return 200, got %d", w.Code)
	}
	var got svcresearch.Answer
	if err := json.NewDecoder(bytes.NewReader(w.Body.Bytes())).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !got.Refused {
		t.Error("expected Refused=true in response")
	}
	if got.RefusalReason != "forward_looking" {
		t.Errorf("RefusalReason = %q, want %q", got.RefusalReason, "forward_looking")
	}
}

func TestAsk_401OnMissingUserID(t *testing.T) {
	h := NewHandler(&stubAnswerSvc{})
	req := httptest.NewRequest(http.MethodPost, "/api/research/ask",
		strings.NewReader(`{"query":"test"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.Ask(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}
