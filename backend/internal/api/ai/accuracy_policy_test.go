package ai_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func TestFailureAnalysisAccuracy_PolicyVersionFilter(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	s := env.store

	run := &models.TestRun{Name: "policies"}
	require.NoError(t, s.CreateTestRun(run))
	direct := false
	add := func(attempt int, policy, human string) {
		decided := time.Now().UTC().Add(-time.Hour)
		require.NoError(t, s.AddRunResult(&models.RunResult{
			TestRunID: run.ID, TestNameSnapshot: "t", AttemptNumber: attempt, Status: models.StatusFail, ErrorMessage: "boom",
			DefectType: human, SuggestedVerdict: models.VerdictProductBug, SuggestedDefectType: "product_bug",
			SuggestedConfidence: models.ConfidenceHigh, SuggestedEngine: models.AnalysisEngineTypeSafe,
			SuggestedPolicyVersion: policy, SuggestedIsClone: &direct, DecidedAt: &decided,
		}))
	}
	add(1, "fa-verdict-v4", "product_bug")
	add(2, "fa-verdict-v5", "automation_bug")
	add(3, "fa-verdict-v5", "product_bug")

	type resp struct {
		Total          int      `json:"total"`
		DirectTotal    int      `json:"direct_total"`
		DirectAgreed   int      `json:"direct_agreed"`
		PolicyVersion  string   `json:"policy_version"`
		PolicyVersions []string `json:"policy_versions"`
	}
	get := func(query string) resp {
		t.Helper()
		rr := doRequest(env, "GET", "/api/ai/failure-analysis/accuracy"+query, nil)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var got resp
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
		return got
	}

	all := get("")
	require.Equal(t, 3, all.Total)
	require.Equal(t, 3, all.DirectTotal)
	require.Equal(t, []string{"fa-verdict-v5", "fa-verdict-v4"}, all.PolicyVersions)

	v5 := get("?policy_version=fa-verdict-v5")
	require.Equal(t, 2, v5.Total)
	require.Equal(t, 1, v5.DirectAgreed)
	require.Equal(t, "fa-verdict-v5", v5.PolicyVersion)
	require.Equal(t, []string{"fa-verdict-v5", "fa-verdict-v4"}, v5.PolicyVersions)

	require.Equal(t, 0, get("?policy_version=fa-verdict-v9").Total)
	require.Equal(t, 3, get("?policy_version=").Total, "an empty filter means all versions")

	rr := doRequest(env, "GET", "/api/ai/failure-analysis/accuracy?policy_version="+strings.Repeat("v", 65), nil)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}
