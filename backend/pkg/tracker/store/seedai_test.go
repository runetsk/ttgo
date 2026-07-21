package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
)

func aiTestCfg() AISeedConfig {
	// 12 days so the "fixed" template window (age 25..10) is actually entered.
	return AISeedConfig{Seed: 1, Days: 12, ResultsPerRun: 200, TestCases: 250}
}

func TestSeedAIFailureDatasetCounts(t *testing.T) {
	s := newTestStore(t)
	cfg := aiTestCfg()

	res, err := s.SeedAIFailureDataset(cfg)
	require.NoError(t, err)

	assert.Equal(t, cfg.Days, res.TestRuns)
	assert.Equal(t, cfg.Days*cfg.ResultsPerRun, res.RunResults)
	require.Len(t, res.RunIDs, cfg.Days)
	assert.Equal(t, res.RunIDs[0], res.LatestRunID)
	assert.Positive(t, res.FailingRows)
	assert.Positive(t, res.LabeledRows)

	var total int64
	require.NoError(t, s.db.Model(&models.RunResult{}).Count(&total).Error)
	assert.EqualValues(t, res.RunResults, total)

	// Every failing row must be analyzable: message, stack, type, log, steps.
	var bare int64
	require.NoError(t, s.db.Model(&models.RunResult{}).
		Where("status IN ('FAIL','ERROR') AND (error_message = '' OR stack_trace = '' OR failure_type = '' OR log_text = '')").
		Count(&bare).Error)
	assert.Zero(t, bare)

	// Both failure statuses appear (ERROR comes from incident/infra templates).
	var errCount, failCount int64
	require.NoError(t, s.db.Model(&models.RunResult{}).Where("status = ?", models.StatusError).Count(&errCount).Error)
	require.NoError(t, s.db.Model(&models.RunResult{}).Where("status = ?", models.StatusFail).Count(&failCount).Error)
	assert.Positive(t, errCount)
	assert.Positive(t, failCount)
}

// The newest run must land in a realistic dedup band: each planted template
// collapses to one signature (volatile parts normalize away), and distinct
// root causes stay distinct.
func TestSeedAIFailureDatasetLatestRunGroups(t *testing.T) {
	s := newTestStore(t)
	res, err := s.SeedAIFailureDataset(aiTestCfg())
	require.NoError(t, err)

	var rows []*models.RunResult
	require.NoError(t, s.db.
		Where("test_run_id = ? AND status IN ('FAIL','ERROR')", res.LatestRunID).
		Find(&rows).Error)
	require.NotEmpty(t, rows)

	groups := failureanalysis.GroupFailures(rows)
	assert.GreaterOrEqual(t, len(groups), 8, "latest run should have a realistic spread of failure groups")
	assert.LessOrEqual(t, len(groups), 12, "planted templates must not shatter into per-row groups")

	// The latest-run incident slice is the biggest group.
	assert.GreaterOrEqual(t, len(groups[0].Members), 20)

	// Ground truth agrees with what actually landed in the newest run.
	for _, g := range res.GroundTruth {
		require.NotEmpty(t, g.SampleMessage, g.TemplateKey)
		require.True(t, models.IsValidDefectType(g.ExpectedDefect), g.TemplateKey)
		if g.Scenario == "persistent" || g.Scenario == "latest-incident" || g.Scenario == "singleton" {
			assert.Positive(t, g.LatestRunRows, g.TemplateKey)
		}
		if g.Scenario == "fixed" || g.Scenario == "incident" {
			assert.Zero(t, g.LatestRunRows, g.TemplateKey)
			assert.Positive(t, g.TotalRows, g.TemplateKey)
		}
		if g.Scenario == "singleton" {
			assert.Equal(t, 1, g.TotalRows, g.TemplateKey)
		}
	}
}

// Newest 3 runs are untriaged; older failing rows carry mostly-conclusive
// human labels so the enrichment's defect-type rollup has signal.
func TestSeedAIFailureDatasetTriageLabels(t *testing.T) {
	s := newTestStore(t)
	cfg := aiTestCfg()
	res, err := s.SeedAIFailureDataset(cfg)
	require.NoError(t, err)

	var fresh int64
	require.NoError(t, s.db.Model(&models.RunResult{}).
		Where("test_run_id IN ? AND status IN ('FAIL','ERROR') AND defect_type <> 'to_investigate'", res.RunIDs[:3]).
		Count(&fresh).Error)
	assert.Zero(t, fresh, "newest 3 runs must be untriaged")

	var labeled int64
	require.NoError(t, s.db.Model(&models.RunResult{}).
		Where("test_run_id IN ? AND defect_type IN ('product_bug','automation_bug','system_issue')", res.RunIDs[3:]).
		Count(&labeled).Error)
	assert.Positive(t, labeled, "older runs must carry conclusive labels")
	assert.EqualValues(t, res.LabeledRows, labeled)

	// Non-failing rows never carry a triage value.
	var mislabeled int64
	require.NoError(t, s.db.Model(&models.RunResult{}).
		Where("status NOT IN ('FAIL','ERROR') AND defect_type <> ''").
		Count(&mislabeled).Error)
	assert.Zero(t, mislabeled)
}

// The historical incident day mass-fails with ERROR rows, giving analytics and
// the analyzer a one-root-cause spike to chew on.
func TestSeedAIFailureDatasetIncidentDay(t *testing.T) {
	s := newTestStore(t)
	res, err := s.SeedAIFailureDataset(aiTestCfg())
	require.NoError(t, err)

	var errRows int64
	require.NoError(t, s.db.Model(&models.RunResult{}).
		Where("test_run_id = ? AND status = 'ERROR'", res.RunIDs[6]).
		Count(&errRows).Error)
	assert.GreaterOrEqual(t, errRows, int64(30), "incident day should mass-ERROR")
}

func TestSeedAIFailureDatasetDeterministic(t *testing.T) {
	s1, s2 := newTestStore(t), newTestStore(t)
	cfg := aiTestCfg()

	res1, err := s1.SeedAIFailureDataset(cfg)
	require.NoError(t, err)
	res2, err := s2.SeedAIFailureDataset(cfg)
	require.NoError(t, err)
	assert.Equal(t, res1.RunIDs, res2.RunIDs)
	assert.Equal(t, res1.GroundTruth, res2.GroundTruth)
	assert.Equal(t, res1.FailingRows, res2.FailingRows)
	assert.Equal(t, res1.LabeledRows, res2.LabeledRows)

	s3 := newTestStore(t)
	cfg.Seed = 2
	res3, err := s3.SeedAIFailureDataset(cfg)
	require.NoError(t, err)
	assert.NotEqual(t, res1.LatestRunID, res3.LatestRunID)
}

func TestSeedAIFailureDatasetRejectsBadConfig(t *testing.T) {
	s := newTestStore(t)

	_, err := s.SeedAIFailureDataset(AISeedConfig{Seed: 1, Days: 3, ResultsPerRun: 200, TestCases: 250})
	assert.ErrorContains(t, err, "days must be >= 7")

	_, err = s.SeedAIFailureDataset(AISeedConfig{Seed: 1, Days: 10, ResultsPerRun: 300, TestCases: 250})
	assert.ErrorContains(t, err, "exceeds distinct test cases")

	_, err = s.SeedAIFailureDataset(AISeedConfig{Seed: 1, Days: 10, ResultsPerRun: 40, TestCases: 50})
	assert.ErrorContains(t, err, "too few")

	_, err = s.SeedAIFailureDataset(AISeedConfig{Seed: 1, Days: 10, ResultsPerRun: 120, TestCases: 150})
	assert.ErrorContains(t, err, "incident slice would overlap")
}
