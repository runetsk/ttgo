package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func TestAIBudgetSettingsAndMonthlySum(t *testing.T) {
	s := newTestStore(t)

	cfg, err := s.GetOrCreateAIBudgetSettings()
	require.NoError(t, err)
	assert.Zero(t, cfg.PerRequestUSD)

	cfg, err = s.UpdateAIBudgetSettings(map[string]interface{}{"per_request_usd": 0.5, "monthly_usd": 20.0})
	require.NoError(t, err)
	assert.Equal(t, 0.5, cfg.PerRequestUSD)
	assert.Equal(t, 20.0, cfg.MonthlyUSD)

	cfg, err = s.UpdateAIBudgetSettings(map[string]interface{}{"per_request_usd": 0.0})
	require.NoError(t, err)
	assert.Zero(t, cfg.PerRequestUSD, "explicit zero must persist (map write, not struct update)")

	run := &models.AIGenerationRun{RequirementID: "r"}
	_, _, err = s.CreateGenerationRun(run)
	require.NoError(t, err)

	// Monthly spend is tracked by the attempt ledger (stamped when each LLM
	// round-trip occurs), not the run's own created_at — so regenerating an
	// old run still counts toward the current month.
	cost := 1.25
	require.NoError(t, s.CreateGenerationAttempt(&models.AIGenerationAttempt{
		RunID: run.ID, Kind: models.AIGenAttemptGeneration, EstimatedCost: &cost,
	}))

	sum, err := s.SumEstimatedCostSince(time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.InDelta(t, 1.25, sum, 1e-9)

	sum, err = s.SumEstimatedCostSince(time.Now().Add(time.Hour))
	require.NoError(t, err)
	assert.Zero(t, sum)
}
