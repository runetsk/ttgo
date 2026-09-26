package ai_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/require"
)

const verdictReply = `{"verdict":"product_bug","confidence":"high","summary":"s","next_action":"n","rationale":"r"}`

// pricedDeps is an LLM-only route at $10 per million tokens each way. fixedReplyProvider
// reports 200 prompt + 30 completion tokens per call, so one call costs $0.0023.
func pricedDeps(p llm.Provider) failureanalysis.JobDeps {
	in, out := 10.0, 10.0
	return failureanalysis.JobDeps{Narrative: p, NarrativeModel: "mock",
		Pricing: failureanalysis.Pricing{LLMProviderID: "prov-1", LLMPromptPerMTok: &in, LLMCompletionPerMTok: &out}}
}

func TestAnalyzeRunResult_RecordsTheLLMCall(t *testing.T) {
	prov := &fixedReplyProvider{reply: verdictReply}
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) { return pricedDeps(prov), nil })
	rec := serve(e.h.AnalyzeRunResult, "POST", map[string]string{"id": e.result.ID})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var row models.RunResultAnalysis
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &row))

	events, err := e.s.ListAnalysisCostEventsForRun(e.runID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	ev := events[0]
	require.Equal(t, models.AnalysisCostKindAnalysis, ev.Kind)
	require.Equal(t, models.AnalysisCostEngineLLM, ev.Engine)
	require.Equal(t, row.ID, *ev.AnalysisID)
	require.Nil(t, ev.JobID, "a single-result analysis has no job")
	require.Equal(t, 200, ev.PromptTokens)
	require.Equal(t, 30, ev.CompletionTokens)
	require.InDelta(t, 0.0023, *ev.EstimatedCost, 1e-12)
}

// Explain spends now on a decision stored long ago: the spend counts in the current month.
func TestExplainAnalysis_BillsTheCurrentMonth(t *testing.T) {
	prov := &fixedReplyProvider{reply: `{"summary":"Timing race","next_action":"Wait for the button","rationale":"R"}`}
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) { return pricedDeps(prov), nil })
	a := e.typeSafeRow(t, models.NarrativeStatusSkipped, models.DecisionStatusOK)
	old := time.Now().AddDate(0, -2, 0)
	require.NoError(t, e.s.DB().Model(&models.RunResultAnalysis{}).Where("id = ?", a.ID).Update("created_at", old).Error)

	rec := e.explain(a)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	events, err := e.s.ListAnalysisCostEventsForRun(e.runID)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, models.AnalysisCostKindExplain, events[0].Kind)
	require.Equal(t, a.ID, *events[0].AnalysisID)
	spent, err := e.s.SumAnalysisCostSince(store.MonthStartUTC(time.Now()))
	require.NoError(t, err)
	require.InDelta(t, 0.0023, spent, 1e-12)
}
