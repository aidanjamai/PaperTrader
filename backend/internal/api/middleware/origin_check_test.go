package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const allowed = "https://app.example.com"

func runOrigin(method string, headers map[string]string) *httptest.ResponseRecorder {
	called := false
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	h := OriginCheck(allowed)(stub)

	req := httptest.NewRequest(method, "/whatever", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !called && w.Code == http.StatusOK {
		// sanity: stub should not report 200 without being called
		w.Code = -1
	}
	return w
}

func TestOriginCheck_AllowsGet(t *testing.T) {
	w := runOrigin(http.MethodGet, nil)
	if w.Code != http.StatusOK {
		t.Errorf("GET with no Origin: got %d, want 200", w.Code)
	}
}

func TestOriginCheck_AllowsOptionsForPreflight(t *testing.T) {
	w := runOrigin(http.MethodOptions, map[string]string{"Origin": "https://attacker.example"})
	if w.Code != http.StatusOK {
		t.Errorf("OPTIONS preflight: got %d, want 200", w.Code)
	}
}

func TestOriginCheck_BlocksMissingOriginOnPost(t *testing.T) {
	w := runOrigin(http.MethodPost, nil)
	if w.Code != http.StatusForbidden {
		t.Errorf("POST with no Origin: got %d, want 403", w.Code)
	}
}

func TestOriginCheck_BlocksMismatchedOriginOnPost(t *testing.T) {
	w := runOrigin(http.MethodPost, map[string]string{"Origin": "https://attacker.example"})
	if w.Code != http.StatusForbidden {
		t.Errorf("POST with foreign Origin: got %d, want 403", w.Code)
	}
}

func TestOriginCheck_AllowsMatchingOriginOnPost(t *testing.T) {
	w := runOrigin(http.MethodPost, map[string]string{"Origin": allowed})
	if w.Code != http.StatusOK {
		t.Errorf("POST with allowed Origin: got %d, want 200", w.Code)
	}
}

// Sec-Fetch-Site: same-origin lets a request through even when Origin is
// absent — covers older browsers / unusual clients that still send the
// fetch-metadata signals.
func TestOriginCheck_AllowsSameOriginFetchMetadata(t *testing.T) {
	w := runOrigin(http.MethodPost, map[string]string{"Sec-Fetch-Site": "same-origin"})
	if w.Code != http.StatusOK {
		t.Errorf("POST with Sec-Fetch-Site=same-origin: got %d, want 200", w.Code)
	}
}

func TestOriginCheck_BlocksCrossSiteFetchMetadata(t *testing.T) {
	w := runOrigin(http.MethodPost, map[string]string{"Sec-Fetch-Site": "cross-site"})
	if w.Code != http.StatusForbidden {
		t.Errorf("POST with Sec-Fetch-Site=cross-site: got %d, want 403", w.Code)
	}
}
