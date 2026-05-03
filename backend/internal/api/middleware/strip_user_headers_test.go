package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStripUserHeaders_RemovesForgedHeaders(t *testing.T) {
	var got struct {
		userID string
		email  string
	}
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.userID = r.Header.Get("X-User-ID")
		got.email = r.Header.Get("X-User-Email")
		w.WriteHeader(http.StatusOK)
	})
	h := StripUserHeaders()(stub)

	req := httptest.NewRequest(http.MethodPost, "/account/login", nil)
	req.Header.Set("X-User-ID", "forged-victim-id")
	req.Header.Set("X-User-Email", "victim@example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if got.userID != "" {
		t.Errorf("X-User-ID survived: got %q, want empty", got.userID)
	}
	if got.email != "" {
		t.Errorf("X-User-Email survived: got %q, want empty", got.email)
	}
}
