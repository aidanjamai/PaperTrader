package data

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// TestMain enables unquoted decimal JSON for the data test binary, mirroring
// what each main() does in production. The flag is process-global, so setting
// it once here covers every test in the package.
func TestMain(m *testing.M) {
	EnableUnquotedDecimalJSON()
	os.Exit(m.Run())
}

// TestDecimalMarshalUnquoted guards against accidentally flipping the global
// MarshalJSONWithoutQuotes flag back to false. If this test fails the frontend
// would start receiving "123.45" (quoted string) instead of 123.45 (number).
func TestDecimalMarshalUnquoted(t *testing.T) {
	type payload struct {
		Price decimal.Decimal `json:"price"`
	}
	p := payload{Price: decimal.RequireFromString("123.45")}

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	got := string(b)

	// Must contain an unquoted number — not a quoted string.
	if strings.Contains(got, `"price":"`) {
		t.Errorf("decimal serialized as quoted string: %s (want unquoted number)", got)
	}
	if !strings.Contains(got, `"price":123.45`) {
		t.Errorf("decimal serialized unexpectedly: %s (want {\"price\":123.45})", got)
	}
}

// TestDecimalUnmarshalAcceptsNumber confirms that an inbound JSON number
// (not a string) deserializes correctly — the frontend sends raw numbers.
func TestDecimalUnmarshalAcceptsNumber(t *testing.T) {
	type payload struct {
		Price decimal.Decimal `json:"price"`
	}
	var p payload
	if err := json.Unmarshal([]byte(`{"price":99.99}`), &p); err != nil {
		t.Fatalf("json.Unmarshal from number: %v", err)
	}
	want := decimal.RequireFromString("99.99")
	if !p.Price.Equal(want) {
		t.Errorf("got %s, want %s", p.Price, want)
	}
}
