package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecover_CatchesPanicAndReturns500(t *testing.T) {
	h := Recover()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	w := httptest.NewRecorder()

	// If Recover does not catch, the test will fail with a goroutine panic.
	h.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want 500", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("content-type: got %q, want application/json", got)
	}
}

func TestRecover_PassesThroughNormalResponse(t *testing.T) {
	h := Recover()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusTeapot {
		t.Errorf("status: got %d, want 418", w.Code)
	}
}

func TestRecover_RepanicsAbortHandler(t *testing.T) {
	defer func() {
		rec := recover()
		if rec != http.ErrAbortHandler {
			t.Errorf("expected re-panic with http.ErrAbortHandler, got %v", rec)
		}
	}()

	h := Recover()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req) // must propagate the panic
	t.Fatal("expected ServeHTTP to re-panic, but it returned")
}
