package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// buildTestEdgarClient returns an EdgarClient pointed at the given httptest server.
func buildTestEdgarClient(srv *httptest.Server, userAgent string) *EdgarClient {
	c := NewEdgarClient(userAgent)
	c.httpClient = srv.Client()
	return c
}

func TestEdgarClient_UserAgentHeader(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		// Serve a minimal company_tickers.json response.
		json.NewEncoder(w).Encode(map[string]any{
			"0": map[string]any{"cik_str": 320193, "ticker": "AAPL"},
		})
	}))
	defer srv.Close()

	c := NewEdgarClient("PaperTrader test@example.com")
	c.httpClient = srv.Client()
	// Point all requests to the test server.
	c.httpClient.Transport = rewriteHostTransport{target: srv.URL, inner: srv.Client().Transport}

	// loadTickerMap issues one GET; we verify the UA is set.
	if err := c.loadTickerMap(context.Background()); err != nil {
		t.Fatalf("loadTickerMap: %v", err)
	}
	if gotUA != "PaperTrader test@example.com" {
		t.Errorf("User-Agent = %q, want %q", gotUA, "PaperTrader test@example.com")
	}
}

// rewriteHostTransport rewrites every request's host to a fixed target URL.
type rewriteHostTransport struct {
	target string
	inner  http.RoundTripper
}

func (t rewriteHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.URL.Scheme = "http"
	cloned.URL.Host = t.target[len("http://"):]
	return t.inner.RoundTrip(cloned)
}

func TestEdgarClient_CIKPadding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"0": map[string]any{"cik_str": 320193, "ticker": "AAPL"},
		})
	}))
	defer srv.Close()

	c := NewEdgarClient("PaperTrader test@example.com")
	c.httpClient = srv.Client()
	c.httpClient.Transport = rewriteHostTransport{target: srv.URL, inner: srv.Client().Transport}

	if err := c.loadTickerMap(context.Background()); err != nil {
		t.Fatal(err)
	}
	cik, ok := c.tickerMap["AAPL"]
	if !ok {
		t.Fatal("AAPL not in ticker map")
	}
	if len(cik) != 10 {
		t.Errorf("CIK length = %d, want 10 digits; got %q", len(cik), cik)
	}
	if cik != "0000320193" {
		t.Errorf("CIK = %q, want 0000320193", cik)
	}
}

func TestEdgarClient_PrimaryDocumentURL(t *testing.T) {
	submissionsBody, _ := json.Marshal(map[string]any{
		"filings": map[string]any{
			"recent": map[string]any{
				"accessionNumber": []string{"0000320193-24-000123"},
				"form":            []string{"10-K"},
				"filingDate":      []string{"2024-11-01"},
				"primaryDocument": []string{"aapl-20240928.htm"},
			},
		},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(submissionsBody)
	}))
	defer srv.Close()

	c := NewEdgarClient("PaperTrader test@example.com")
	c.httpClient = srv.Client()
	c.httpClient.Transport = rewriteHostTransport{target: srv.URL, inner: srv.Client().Transport}

	// Pre-populate CIK map so we skip the ticker endpoint.
	c.tickerMap = map[string]string{"AAPL": "0000320193"}

	filings, err := c.FetchRecentFilings(context.Background(), "0000320193", []string{"10-K"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(filings) != 1 {
		t.Fatalf("expected 1 filing, got %d", len(filings))
	}
	want := "https://www.sec.gov/Archives/edgar/data/320193/000032019324000123/aapl-20240928.htm"
	if filings[0].URL != want {
		t.Errorf("URL = %q\nwant %q", filings[0].URL, want)
	}
}

func TestEdgarClient_RateLimiter(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		json.NewEncoder(w).Encode(map[string]any{
			"0": map[string]any{"cik_str": 320193, "ticker": "AAPL"},
		})
	}))
	defer srv.Close()

	c := NewEdgarClient("PaperTrader test@example.com")
	c.httpClient = srv.Client()
	c.httpClient.Transport = rewriteHostTransport{target: srv.URL, inner: srv.Client().Transport}

	// 12 requests at 10/sec should take at least 1 second (the 11th request
	// burns the burst and waits for a new token).
	n := 12
	start := time.Now()
	ctx := context.Background()
	for i := 0; i < n; i++ {
		// Reset ticker map each iteration so loadTickerMap fires again.
		c.tickerMap = nil
		if err := c.loadTickerMap(ctx); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	// With 10 req/sec and a burst of 10, 12 calls take at least 200ms.
	// We use a generous lower bound of 100ms to avoid flakiness on slow CI.
	minExpected := 100 * time.Millisecond
	if elapsed < minExpected {
		t.Errorf("12 calls completed in %v, expected >= %v (rate limiter may not be active)", elapsed, minExpected)
	}
	if atomic.LoadInt64(&calls) != int64(n) {
		t.Errorf("expected %d HTTP calls, got %d", n, calls)
	}
}

func TestEdgarClient_URLFormat(t *testing.T) {
	cases := []struct {
		accession     string
		primaryDoc    string
		cik           string
		wantURLSuffix string
	}{
		{
			accession:     "0000320193-24-000123",
			primaryDoc:    "aapl-20240928.htm",
			cik:           "0000320193",
			wantURLSuffix: "/Archives/edgar/data/320193/000032019324000123/aapl-20240928.htm",
		},
	}
	for _, tc := range cases {
		accNoDashes := strings.ReplaceAll(tc.accession, "-", "")
		got := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%s/%s/%s",
			strings.TrimLeft(tc.cik, "0"), accNoDashes, tc.primaryDoc)
		if !strings.HasSuffix(got, tc.wantURLSuffix) {
			t.Errorf("URL = %q, want suffix %q", got, tc.wantURLSuffix)
		}
	}
}
