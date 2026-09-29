package ai_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// analyzedFailure stores a failing result with a direct TypeSafe v7 analysis suggesting
// product_bug at score, the way the worker stores a representative.
func analyzedFailure(t *testing.T, env *testEnv, runID string, n int, score float64) *models.RunResult {
	t.Helper()
	rr := &models.RunResult{TestRunID: runID, TestNameSnapshot: fmt.Sprintf("g%d", n), AttemptNumber: n,
		Status: models.StatusFail, ErrorMessage: "boom"}
	require.NoError(t, env.store.AddRunResult(rr))
	sc := score
	_, err := env.store.CreateAnalysis(&models.RunResultAnalysis{RunResultID: rr.ID, Engine: models.AnalysisEngineTypeSafe,
		ModelName: "jev-1.13.0", Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, ConfidenceScore: &sc,
		SuggestedDefectType: "product_bug", SuggestedDefectTypeConfidence: &sc,
		PolicyVersion:  failureanalysis.PolicyVersionNoExamples,
		DecisionStatus: models.DecisionStatusOK, NarrativeStatus: models.NarrativeStatusOK})
	require.NoError(t, err)
	return rr
}

// triage writes a person's defect type through the real single-result handler (snapshot,
// source and R10 flag exactly as production writes them).
func triage(t *testing.T, env *testEnv, rr *models.RunResult, defectType string) {
	t.Helper()
	rec := doRequest(env, "PUT", "/api/runs/"+rr.TestRunID+"/results/"+rr.ID, map[string]interface{}{"defect_type": defectType})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// seedGradedDecisions triages n analyzed failures through the handler, agreed of them agreeing
// with the suggestion.
func seedGradedDecisions(t *testing.T, env *testEnv, n, agreed int, score float64) {
	t.Helper()
	run := &models.TestRun{Name: "graded"}
	require.NoError(t, env.store.CreateTestRun(run))
	for i := 0; i < n; i++ {
		human := "product_bug"
		if i >= agreed {
			human = "automation_bug"
		}
		triage(t, env, analyzedFailure(t, env, run.ID, i+1, score), human)
	}
}

func TestFailureAnalysisSettings_AutoApply(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	put := func(extra map[string]interface{}) *httptest.ResponseRecorder {
		b := map[string]interface{}{
			"enabled_on_completion": false, "max_analyses_per_run": 20, "dedup_enabled": true,
			"redaction_enabled": true, "prompt_template": "x",
		}
		for k, v := range extra {
			b[k] = v
		}
		return doRequest(env, "PUT", "/api/settings/ai-failure-analysis", b)
	}
	decode := func(rr *httptest.ResponseRecorder) models.AIFailureAnalysisSettings {
		t.Helper()
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var got models.AIFailureAnalysisSettings
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
		return got
	}

	got := decode(doRequest(env, "GET", "/api/settings/ai-failure-analysis", nil))
	require.False(t, got.AutoApplyDefectType)
	require.Equal(t, 95, got.AutoApplyMinConfidence)

	for _, bad := range []int{79, 100, 0} {
		rr := put(map[string]interface{}{"auto_apply_min_confidence": bad})
		require.Equal(t, http.StatusBadRequest, rr.Code, "min=%d: %s", bad, rr.Body.String())
	}

	// Enabling while the gate is closed is refused with the figures.
	rr := put(map[string]interface{}{"auto_apply_defect_type": true})
	require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
	var refused struct {
		Error string                     `json:"error"`
		Gate  failureanalysis.GateStatus `json:"gate"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &refused))
	require.NotEmpty(t, refused.Error)
	require.False(t, refused.Gate.Open)
	require.Equal(t, 0, refused.Gate.Graded)
	got = decode(doRequest(env, "GET", "/api/settings/ai-failure-analysis", nil))
	require.False(t, got.AutoApplyDefectType, "a refused PUT writes nothing")

	// The threshold alone may change while it is off.
	got = decode(put(map[string]interface{}{"auto_apply_min_confidence": 90}))
	require.Equal(t, 90, got.AutoApplyMinConfidence)
	require.False(t, got.AutoApplyDefectType)

	seedGradedDecisions(t, env, 50, 49, 0.97)
	got = decode(put(map[string]interface{}{"auto_apply_defect_type": true}))
	require.True(t, got.AutoApplyDefectType, "the gate is open at 90%")
	require.Equal(t, 90, got.AutoApplyMinConfidence)

	got = decode(put(nil))
	require.True(t, got.AutoApplyDefectType, "omitting the fields keeps the stored values")
	require.Equal(t, 90, got.AutoApplyMinConfidence)

	got = decode(put(map[string]interface{}{"auto_apply_min_confidence": 99}))
	require.True(t, got.AutoApplyDefectType, "only switching it on is gated; a job at a closed threshold pauses")
	require.Equal(t, 99, got.AutoApplyMinConfidence)

	got = decode(put(map[string]interface{}{"auto_apply_defect_type": false}))
	require.False(t, got.AutoApplyDefectType, "switching off is never refused")
}

func TestAutoApplyGateEndpoint(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	seedGradedDecisions(t, env, 50, 50, 0.97)

	get := func(q string) (int, failureanalysis.GateStatus) {
		rr := doRequest(env, "GET", "/api/settings/ai-failure-analysis/auto-apply-gate"+q, nil)
		var g failureanalysis.GateStatus
		if rr.Code == http.StatusOK {
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &g))
		}
		return rr.Code, g
	}
	code, g := get("")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, 50, g.Graded)
	require.True(t, g.Open)
	require.InDelta(t, 0.95, g.MinConfidence, 1e-12, "the stored threshold")
	require.Equal(t, []string{failureanalysis.PolicyVersionNoExamples, failureanalysis.PolicyVersionWithExamples}, g.Policies)

	code, g = get("?min_confidence=99")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, 0, g.Graded, "none graded at 99%")
	require.False(t, g.Open)

	for _, bad := range []string{"?min_confidence=50", "?min_confidence=abc", "?min_confidence=100"} {
		code, _ = get(bad)
		require.Equal(t, http.StatusBadRequest, code, bad)
	}

	// R10: a person confirming an AI label (the UI's Confirm = an ordinary triage write of the same
	// value) never feeds the gate, and neither does correcting one.
	run := &models.TestRun{Name: "confirmed"}
	require.NoError(t, env.store.CreateTestRun(run))
	for i, human := range []string{"product_bug", "system_issue"} {
		rr := analyzedFailure(t, env, run.ID, i+1, 0.97)
		n, err := env.store.ApplyAutoDefectType([]string{rr.ID}, "product_bug")
		require.NoError(t, err)
		require.EqualValues(t, 1, n)
		triage(t, env, rr, human)
		got, err := env.store.GetRunResultByID(rr.ID)
		require.NoError(t, err)
		require.True(t, got.SuggestedAutoApplied)
		require.Equal(t, models.DefectTypeSourceHuman, got.DefectTypeSource)
		require.NotNil(t, got.DecidedAt, "it is still a person's decision for the accuracy report")
	}
	code, g = get("")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, 50, g.Graded, "Confirms and corrections of AI labels are not graded")
}
