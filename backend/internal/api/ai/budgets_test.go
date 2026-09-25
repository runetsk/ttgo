package ai_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Settings → AI shows this month's estimated spend against the monthly budget, so both budget
// endpoints return it next to the settings.
func TestAIBudgetSettings_ReportMonthSpend(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	readBudgets := func(body []byte) map[string]interface{} {
		var got map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &got))
		return got
	}

	rr := doRequest(env, "GET", "/api/settings/ai-budgets", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, 0.0, readBudgets(rr.Body.Bytes())["month_spent_usd"])

	run := &models.AIGenerationRun{RequirementID: "r"}
	_, _, err := env.store.CreateGenerationRun(run)
	require.NoError(t, err)
	cost := 1.25
	require.NoError(t, env.store.CreateGenerationAttempt(&models.AIGenerationAttempt{
		RunID: run.ID, Kind: models.AIGenAttemptGeneration, EstimatedCost: &cost,
	}))

	rr = doRequest(env, "GET", "/api/settings/ai-budgets", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.InDelta(t, 1.25, readBudgets(rr.Body.Bytes())["month_spent_usd"], 1e-9)

	rr = doRequest(env, "PUT", "/api/settings/ai-budgets", map[string]float64{"monthly_usd": 5})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	got := readBudgets(rr.Body.Bytes())
	assert.Equal(t, 5.0, got["monthly_usd"])
	assert.InDelta(t, 1.25, got["month_spent_usd"], 1e-9)
}
