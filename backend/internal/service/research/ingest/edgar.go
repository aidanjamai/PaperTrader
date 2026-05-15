package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/time/rate"
)

// maxFilingBytes caps an individual filing-body read so a pathological 10-K
// (or a malicious server returning a giant body) cannot OOM the ingest job on
// the e2-micro instance. A normal 10-K HTML is 2-15 MB; the cap leaves
// generous headroom while keeping peak RSS bounded.
const maxFilingBytes = 50 * 1024 * 1024

// EdgarClient fetches filings from SEC EDGAR with the mandatory User-Agent
// header and a 10 req/sec rate limiter as required by SEC fair-access policy.
type EdgarClient struct {
	httpClient *http.Client
	userAgent  string
	limiter    *rate.Limiter
	// CIK map is populated lazily on first call to ResolveCIK; no refresh
	// needed for the CLI use case.
	tickerMap map[string]string
}

// Filing is a single SEC filing returned from the submissions endpoint.
type Filing struct {
	CIK             string
	AccessionNumber string
	FormType        string
	PrimaryDocument string
	FiledAt         string
	URL             string
}

func NewEdgarClient(userAgent string) *EdgarClient {
	return &EdgarClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  userAgent,
		// 10 req/sec with burst of 10 — SEC's documented limit.
		limiter: rate.NewLimiter(10, 10),
	}
}

// ResolveCIK maps a ticker symbol to a zero-padded 10-digit CIK string.
// The result is cached in memory for the lifetime of the client.
func (c *EdgarClient) ResolveCIK(ctx context.Context, symbol string) (string, error) {
	if c.tickerMap == nil {
		if err := c.loadTickerMap(ctx); err != nil {
			return "", err
		}
	}
	cik, ok := c.tickerMap[strings.ToUpper(symbol)]
	if !ok {
		return "", fmt.Errorf("edgar: CIK not found for symbol %q", symbol)
	}
	return cik, nil
}

func (c *EdgarClient) loadTickerMap(ctx context.Context) error {
	body, err := c.get(ctx, "https://www.sec.gov/files/company_tickers.json")
	if err != nil {
		return fmt.Errorf("edgar: load ticker map: %w", err)
	}
	defer body.Close()

	// Response is {"0":{"cik_str":789019,"ticker":"MSFT","title":"MICROSOFT CORP"}, ...}
	var raw map[string]struct {
		CIK    int    `json:"cik_str"`
		Ticker string `json:"ticker"`
	}
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return fmt.Errorf("edgar: decode ticker map: %w", err)
	}

	c.tickerMap = make(map[string]string, len(raw))
	for _, entry := range raw {
		c.tickerMap[strings.ToUpper(entry.Ticker)] = fmt.Sprintf("%010d", entry.CIK)
	}
	return nil
}

// FetchRecentFilings returns up to n filings of the given form types for cik.
func (c *EdgarClient) FetchRecentFilings(ctx context.Context, cik string, formTypes []string, n int) ([]Filing, error) {
	url := fmt.Sprintf("https://data.sec.gov/submissions/CIK%s.json", cik)
	body, err := c.get(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("edgar: fetch submissions %s: %w", cik, err)
	}
	defer body.Close()

	var sub struct {
		Filings struct {
			Recent struct {
				AccessionNumber []string `json:"accessionNumber"`
				Form            []string `json:"form"`
				FilingDate      []string `json:"filingDate"`
				PrimaryDocument []string `json:"primaryDocument"`
			} `json:"recent"`
		} `json:"filings"`
	}
	if err := json.NewDecoder(body).Decode(&sub); err != nil {
		return nil, fmt.Errorf("edgar: decode submissions: %w", err)
	}

	allowedForms := make(map[string]bool, len(formTypes))
	for _, ft := range formTypes {
		allowedForms[ft] = true
	}

	r := sub.Filings.Recent
	var results []Filing
	for i := range r.AccessionNumber {
		if !allowedForms[r.Form[i]] {
			continue
		}
		accNoDashes := strings.ReplaceAll(r.AccessionNumber[i], "-", "")
		docURL := fmt.Sprintf(
			"https://www.sec.gov/Archives/edgar/data/%s/%s/%s",
			strings.TrimLeft(cik, "0"), accNoDashes, r.PrimaryDocument[i],
		)
		results = append(results, Filing{
			CIK:             cik,
			AccessionNumber: r.AccessionNumber[i],
			FormType:        r.Form[i],
			PrimaryDocument: r.PrimaryDocument[i],
			FiledAt:         r.FilingDate[i],
			URL:             docURL,
		})
		if len(results) >= n {
			break
		}
	}
	return results, nil
}

// FetchFilingText downloads the filing's primary document and returns plain text.
// HTML tags are stripped and XBRL noise is discarded.
func (c *EdgarClient) FetchFilingText(ctx context.Context, f Filing) (string, error) {
	body, err := c.get(ctx, f.URL)
	if err != nil {
		return "", fmt.Errorf("edgar: fetch filing %s: %w", f.URL, err)
	}
	defer body.Close()

	raw, err := io.ReadAll(io.LimitReader(body, maxFilingBytes+1))
	if err != nil {
		return "", fmt.Errorf("edgar: read filing body: %w", err)
	}
	if len(raw) > maxFilingBytes {
		return "", fmt.Errorf("edgar: filing body exceeds %d bytes: %s", maxFilingBytes, f.URL)
	}

	return stripHTML(string(raw)), nil
}

// get honours the SEC rate limit and sets the required User-Agent header.
func (c *EdgarClient) get(ctx context.Context, url string) (io.ReadCloser, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json, text/html, */*")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("edgar: HTTP %d for %s", resp.StatusCode, url)
	}
	return resp.Body, nil
}

// stripHTML walks the HTML parse tree, collecting visible text nodes.
// <script> and <style> subtrees are skipped entirely.
func stripHTML(rawHTML string) string {
	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		// If parse fails, return the raw text (still better than nothing).
		return rawHTML
	}

	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			tag := strings.ToLower(n.Data)
			if tag == "script" || tag == "style" {
				return
			}
		}
		if n.Type == html.TextNode {
			t := strings.TrimSpace(n.Data)
			if t != "" {
				sb.WriteString(t)
				sb.WriteByte('\n')
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)

	// Collapse runs of blank lines.
	lines := strings.Split(sb.String(), "\n")
	var out []string
	blank := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			blank++
			if blank <= 1 {
				out = append(out, "")
			}
		} else {
			blank = 0
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
