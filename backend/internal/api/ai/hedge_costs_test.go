package ai_test

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// slowFirstProvider stalls its first call until cancelled and answers later ones like
// fixedReplyProvider (200 prompt + 30 completion tokens). Safe for the hedge's two goroutines.
type slowFirstProvider struct {
	reply string
	calls atomic.Int32
}

func (p *slowFirstProvider) Chat(ctx context.Context, _ llm.ChatRequest) (*llm.ChatResponse, error) {
	if p.calls.Add(1) == 1 {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &llm.ChatResponse{Content: p.reply, Model: "mock", FinishReason: "stop",
		Usage: &llm.ChatUsage{PromptTokens: 200, CompletionTokens: 30, TotalTokens: 230}}, nil
}

func TestAnalyzeRunResult_RecordsAFiredHedge(t *testing.T) {
	prov := &slowFirstProvider{reply: verdictReply}
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) {
		d := pricedDeps(llm.WithHedge(prov, 20*time.Millisecond))
		d.HedgingOn = true
		return d, nil
	})
	rec := serve(e.h.AnalyzeRunResult, "POST", map[string]string{"id": e.result.ID})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	events, err := e.s.ListAnalysisCostEventsForRun(e.runID)
	require.NoError(t, err)
	byKind := map[string]*models.AIAnalysisCostEvent{}
	for _, ev := range events {
		byKind[ev.Kind] = ev
	}
	require.Len(t, events, 2)
	require.Equal(t, 200, byKind[models.AnalysisCostKindAnalysis].PromptTokens)
	hedge := byKind[models.AnalysisCostKindHedge]
	require.NotNil(t, hedge)
	require.Equal(t, 200, hedge.PromptTokens)
	require.InDelta(t, 0.002, *hedge.EstimatedCost, 1e-12)
	require.Equal(t, byKind[models.AnalysisCostKindAnalysis].AnalysisID, hedge.AnalysisID, "tied to the analysis it served")
}

func TestExplainAnalysis_RecordsAFiredHedge(t *testing.T) {
	prov := &slowFirstProvider{reply: `{"summary":"Timing race","next_action":"Wait for the button","rationale":"R"}`}
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) {
		return pricedDeps(llm.WithHedge(prov, 20*time.Millisecond)), nil
	})
	a := e.typeSafeRow(t, models.NarrativeStatusSkipped, models.DecisionStatusOK)
	rec := e.explain(a)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	events, err := e.s.ListAnalysisCostEventsForRun(e.runID)
	require.NoError(t, err)
	kinds := map[string]int{}
	for _, ev := range events {
		kinds[ev.Kind]++
		if ev.Kind == models.AnalysisCostKindHedge {
			require.Equal(t, a.ID, *ev.AnalysisID)
		}
	}
	require.Equal(t, map[string]int{models.AnalysisCostKindExplain: 1, models.AnalysisCostKindHedge: 1}, kinds)
}
