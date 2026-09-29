package ai

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
	"ttgo/pkg/tracker/typesafe"
)

// RerankSearch re-orders the first failureanalysis.SearchRerankCandidates results of a search by
// TypeSafe.ai's probability that each is what the query looks for (spec Wave 5 §4); the rest keep
// their BM25 order after them. It returns the results unchanged and false when the use is
// unavailable or the request fails.
func (h *Handler) RerankSearch(ctx context.Context, query string, results []store.SearchResult) ([]store.SearchResult, bool) {
	use, _ := h.TypeSafeFor(TypeSafeUseSearchRerank)
	if use == nil || len(results) < 2 {
		return results, false
	}
	n := min(len(results), failureanalysis.SearchRerankCandidates)
	red := func(s string) string {
		if use.Redact {
			return failureanalysis.Redact(s)
		}
		return s
	}
	cases := make([]map[string]any, n)
	qs := make(map[string]typesafe.Question, n)
	for j := 0; j < n; j++ {
		cases[j] = map[string]any{"name": red(results[j].Name), "description": red(results[j].Description)}
		qs[fmt.Sprintf("case_%d", j)] = failureanalysis.SearchRelevanceQuestion(j)
	}
	resp, err := use.Client.Evaluate(ctx, typesafe.Request{State: map[string]any{"query": red(query), "cases": cases},
		Model: use.Model, Questions: qs})
	if err != nil {
		slog.WarnContext(ctx, "search: TypeSafe re-ranking failed; keeping the BM25 order", "err", err)
		return results, false
	}
	h.recordTypeSafeUse(models.AnalysisCostKindSearch, use, resp)
	out := append([]store.SearchResult(nil), results...)
	for j := 0; j < n; j++ {
		if a, ok := resp.Answers[fmt.Sprintf("case_%d", j)]; ok {
			p := a.Noul
			out[j].Relevance = &p
		}
	}
	top := out[:n]
	sort.SliceStable(top, func(i, j int) bool { return relevance(top[i]) > relevance(top[j]) })
	return out, true
}

func relevance(r store.SearchResult) float64 {
	if r.Relevance == nil {
		return -1
	}
	return *r.Relevance
}
