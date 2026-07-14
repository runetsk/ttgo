package llm

// EstimateCostUSD computes the configured cost of a call from per-million-token
// prices (spec: administrators configure prices; no hosted catalog). Returns
// nil when no price is configured — cost is unknown, not zero. A missing side
// contributes zero (e.g. only completion pricing configured).
func EstimateCostUSD(promptTokens, completionTokens int, promptPricePerMTok, completionPricePerMTok *float64) *float64 {
	if promptPricePerMTok == nil && completionPricePerMTok == nil {
		return nil
	}
	cost := 0.0
	if promptPricePerMTok != nil {
		cost += float64(promptTokens) * *promptPricePerMTok / 1e6
	}
	if completionPricePerMTok != nil {
		cost += float64(completionTokens) * *completionPricePerMTok / 1e6
	}
	return &cost
}
