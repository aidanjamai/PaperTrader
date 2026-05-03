package data

import "github.com/shopspring/decimal"

// EnableUnquotedDecimalJSON makes shopspring/decimal serialize as unquoted
// JSON numbers (e.g. 100.50) instead of quoted strings (e.g. "100.50"), which
// is what the frontend expects. shopspring's UnmarshalJSON accepts both forms,
// so inbound JSON keeps working unchanged.
//
// This sets a process-global flag in the decimal package, so each binary's
// main() must call it explicitly before any decimal value is serialized. Doing
// it here (rather than in an init()) keeps the dependency visible at the
// binary entrypoint — a future cmd/* that forgets the call will fail loudly in
// tests rather than silently shipping quoted strings to the frontend.
func EnableUnquotedDecimalJSON() {
	decimal.MarshalJSONWithoutQuotes = true
}
