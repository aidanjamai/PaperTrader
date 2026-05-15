package research

// CalcCostMicros returns the would-be paid cost in USD micros (1 cent = 10 000).
// Free-tier usage still records a cost so the per-query metric is meaningful.
func CalcCostMicros(client LLMClient, promptTokens, completionTokens int) int {
	inMicros, outMicros := client.PriceMicrosPer1KTokens()
	return (promptTokens*inMicros)/1000 + (completionTokens*outMicros)/1000
}

// EmbedCostMicros returns the Voyage AI embed cost in USD micros for the given
// token count. Rates come from Voyage's published pricing page.
// voyage-finance-2: $0.12/M tokens → 120 micros/1K tokens.
// Unknown models return 0 rather than failing.
func EmbedCostMicros(tokens int, model string) int {
	switch model {
	case "voyage-finance-2":
		return (tokens * 120) / 1000
	default:
		return 0
	}
}
