package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/require"
)

// gradeThroughTheHandler stores n failing results with a direct TypeSafe v7 analysis suggesting
// product_bug at score and has a person agree with each through the real triage endpoint, so the
// graded rows carry exactly the snapshot, source and R10 flag production writes.
func gradeThroughTheHandler(t *testing.T, s *store.Store, n int, score float64) {
	t.Helper()
	srv := NewServer(s)
	run := &models.TestRun{Name: "graded"}
	require.NoError(t, s.CreateTestRun(run))
	for i := 0; i < n; i++ {
		rr := &models.RunResult{TestRunID: run.ID, TestNameSnapshot: fmt.Sprintf("g%d", i), AttemptNumber: i + 1,
			Status: models.StatusFail, ErrorMessage: "boom"}
		require.NoError(t, s.AddRunResult(rr))
		sc := score
		_, err := s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: rr.ID, Engine: models.AnalysisEngineTypeSafe,
			ModelName: "jev-1.13.0", Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, ConfidenceScore: &sc,
			SuggestedDefectType: "product_bug", SuggestedDefectTypeConfidence: &sc,
			PolicyVersion:  failureanalysis.PolicyVersionNoExamples,
			DecisionStatus: models.DecisionStatusOK, NarrativeStatus: models.NarrativeStatusOK})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPut, "/api/runs/"+run.ID+"/results/"+rr.ID, strings.NewReader(`{"defect_type":"product_bug"}`))
		req.Header.Set("Content-Type", "application/json")
		addTestAuth(t, s, req)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
}

func TestResolver_AutoApplyFollowsTheSettingAndTheGate(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, false)
	r := newAnalyzeDepsResolver(s, nil)
	resolve := func(trigger string) failureanalysis.JobDeps {
		t.Helper()
		d, err := r(trigger)
		require.NoError(t, err)
		return d
	}
	manual := func() failureanalysis.JobDeps { return resolve(models.RunAnalysisJobTriggerManual) }

	d := manual()
	require.Nil(t, d.AutoApply)
	require.Equal(t, models.AutoApplyStateOff, d.AutoApplyState, "off by default")

	_, err := s.SetFailureAnalysisAutoApply(true, 95)
	require.NoError(t, err)
	d = manual()
	require.Nil(t, d.AutoApply, "nothing graded yet: the gate is closed")
	require.Equal(t, models.AutoApplyStatePaused, d.AutoApplyState)

	gradeThroughTheHandler(t, s, 50, 0.97)
	d = manual()
	require.NotNil(t, d.AutoApply, "decisions recorded by the real triage handler open the gate")
	require.InDelta(t, 0.95, d.AutoApply.MinConfidence, 1e-12)
	require.Equal(t, models.AutoApplyStateOn, d.AutoApplyState)
	require.NotNil(t, d.Transfer, "P2.6's transfer check survives the merged block")

	explain := resolve(failureanalysis.TriggerExplain)
	require.NotNil(t, explain.AutoApply, "group Explain maintains the semantic clones' labels (R10)")
	require.Equal(t, models.AutoApplyStateOn, explain.AutoApplyState)

	_, err = s.SetFailureAnalysisAutoApply(true, 99)
	require.NoError(t, err)
	d = manual()
	require.Nil(t, d.AutoApply, "the graded rows are below a 99% threshold")
	require.Equal(t, models.AutoApplyStatePaused, d.AutoApplyState)

	_, err = s.SetFailureAnalysisAutoApply(true, 95)
	require.NoError(t, err)
	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{ClearAPIKey: true})
	require.NoError(t, err)
	d = manual()
	require.NotNil(t, d.Decider, "a missing key still attaches the unavailable decider")
	require.Nil(t, d.AutoApply)
	require.Equal(t, models.AutoApplyStateOff, d.AutoApplyState, "no usable TypeSafe client: nothing could qualify")

	enableTypeSafe(t, s, false)
	off := false
	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{VerdictEngineEnabled: &off})
	require.NoError(t, err)
	d = manual()
	require.Nil(t, d.AutoApply)
	require.Equal(t, models.AutoApplyStateOff, d.AutoApplyState, "without a TypeSafe decider nothing could qualify")
}
