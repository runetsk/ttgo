package store

import (
	"testing"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func costPtr(v float64) *float64 { return &v }

func TestAnalysisCostLedger_SumsAndReport(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	now := time.Now()
	events := []*models.AIAnalysisCostEvent{
		{Kind: models.AnalysisCostKindAnalysis, Engine: models.AnalysisCostEngineTypeSafe, RunID: runID, TypeSafeInputTokens: 1000, EstimatedCost: costPtr(0.10)},
		{Kind: models.AnalysisCostKindAnalysis, Engine: models.AnalysisCostEngineLLM, RunID: runID, PromptTokens: 100, CompletionTokens: 10, EstimatedCost: costPtr(0.20)},
		{Kind: models.AnalysisCostKindExplain, Engine: models.AnalysisCostEngineLLM, RunID: runID, PromptTokens: 50, EstimatedCost: costPtr(0.05)},
		{Kind: models.AnalysisCostKindSemantic, Engine: models.AnalysisCostEngineTypeSafe, RunID: runID, TypeSafeInputTokens: 300, EstimatedCost: costPtr(0.01)},
		// An unpriced LLM provider: tokens are recorded, no cost.
		{Kind: models.AnalysisCostKindAnalysis, Engine: models.AnalysisCostEngineLLM, RunID: runID, PromptTokens: 100},
		// Spent 40 days ago: outside both windows below.
		{Kind: models.AnalysisCostKindAnalysis, Engine: models.AnalysisCostEngineLLM, RunID: runID, EstimatedCost: costPtr(9), CreatedAt: now.AddDate(0, 0, -40)},
	}
	for _, e := range events {
		require.NoError(t, s.RecordAnalysisCostEvent(e))
		require.NotEmpty(t, e.ID)
		require.False(t, e.CreatedAt.IsZero())
	}

	sum, err := s.SumAnalysisCostSince(now.Add(-time.Hour))
	require.NoError(t, err)
	require.InDelta(t, 0.36, sum, 1e-9)

	rep, err := s.AnalysisCostReport(now.Add(-time.Hour), now.Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, 5, rep.Events, "unpriced calls are counted as events")
	require.InDelta(t, 0.36, rep.CostUSD, 1e-9)
	require.InDelta(t, 0.11, rep.TypeSafeCostUSD, 1e-9)
	require.InDelta(t, 0.25, rep.LLMCostUSD, 1e-9)
	require.InDelta(t, 0.05, rep.ExplainCostUSD, 1e-9)

	listed, err := s.ListAnalysisCostEventsForRun(runID)
	require.NoError(t, err)
	require.Len(t, listed, 6)
}

func TestSumEstimatedCostSince_AddsAnalysisToGeneration(t *testing.T) {
	s := newTestStore(t)
	run := &models.AIGenerationRun{RequirementID: "r"}
	_, _, err := s.CreateGenerationRun(run)
	require.NoError(t, err)
	require.NoError(t, s.CreateGenerationAttempt(&models.AIGenerationAttempt{
		RunID: run.ID, Kind: models.AIGenAttemptGeneration, EstimatedCost: costPtr(1.0),
	}))
	require.NoError(t, s.RecordAnalysisCostEvent(&models.AIAnalysisCostEvent{
		Kind: models.AnalysisCostKindExplain, Engine: models.AnalysisCostEngineLLM, RunID: "run-x", EstimatedCost: costPtr(0.25),
	}))

	since := MonthStartUTC(time.Now())
	gen, err := s.SumGenerationCostSince(since)
	require.NoError(t, err)
	require.InDelta(t, 1.0, gen, 1e-9)
	an, err := s.SumAnalysisCostSince(since)
	require.NoError(t, err)
	require.InDelta(t, 0.25, an, 1e-9)
	total, err := s.SumEstimatedCostSince(since)
	require.NoError(t, err)
	require.InDelta(t, 1.25, total, 1e-9, "the monthly budget counts both ledgers")

	report, err := s.GetAIGenerationReport(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Equal(t, 1, report.Analysis.Events)
	require.InDelta(t, 0.25, report.Analysis.ExplainCostUSD, 1e-9)
}

func TestMonthStartUTC(t *testing.T) {
	plus3 := time.FixedZone("UTC+3", 3*3600)
	require.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), MonthStartUTC(time.Date(2026, 9, 28, 1, 30, 0, 0, plus3)))
	require.Equal(t, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), MonthStartUTC(time.Date(2026, 10, 1, 1, 0, 0, 0, plus3)),
		"01:00 on the 1st at UTC+3 is still September in UTC")
}
