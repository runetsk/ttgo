package ai_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func TestExplainAnalysis_InjectionFlaggedNeedsAnOverride(t *testing.T) {
	prov := &promptProvider{reply: explainReply}
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) {
		return failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock"}, nil
	})
	score := 0.95
	a, err := e.s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: e.result.ID, Engine: models.AnalysisEngineTypeSafe,
		ModelName: "jev", Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, ConfidenceScore: &score,
		SuggestedDefectType: "product_bug", NarrativeStatus: models.NarrativeStatusUnavailable,
		Summary: failureanalysis.InjectionSummary})
	require.NoError(t, err)
	signals := markChecked(t, e, e.result, a, 0.97) // flagged; every block of the evidence was checked
	params := map[string]string{"id": e.result.ID, "analysisId": a.ID}

	for _, query := range []string{"", "?override_injection=false"} {
		rec := serveQuery(e.h.ExplainAnalysis, query, params)
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		var body map[string]string
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Equal(t, "possible prompt injection: confirm to send this failure to the LLM", body["error"])
	}
	require.Zero(t, prov.calls, "nothing reaches the LLM without the confirmation")
	still, err := e.s.GetAnalysisByID(a.ID)
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusUnavailable, still.NarrativeStatus, "a refused Explain claims nothing")
	require.Equal(t, a.NarrativeRevision, still.NarrativeRevision)

	rec := serveQuery(e.h.ExplainAnalysis, "?override_injection=true", params)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decodeRow(t, rec.Body.Bytes())
	require.Equal(t, "Shared cause", got.Summary)
	require.Equal(t, models.NarrativeStatusOK, got.NarrativeStatus)
	require.Equal(t, signals, got.Signals, "the flag stays on the row")
	require.Contains(t, prov.prompt, "boom", "the confirmed failure's checked evidence is sent")
	require.Equal(t, 1, prov.calls)
}

func TestExplainAnalysis_UnflaggedNeedsNoOverride(t *testing.T) {
	prov := &promptProvider{reply: explainReply}
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) {
		return failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock"}, nil
	})
	score := 0.95
	a, err := e.s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: e.result.ID, Engine: models.AnalysisEngineTypeSafe,
		ModelName: "jev", Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, ConfidenceScore: &score,
		NarrativeStatus: models.NarrativeStatusSkipped})
	require.NoError(t, err)
	markChecked(t, e, e.result, a, failureanalysis.InjectionMin-0.01)
	rec := e.explain(a)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, prov.calls)
}
