package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func TestGetAIGenerationReport(t *testing.T) {
	s := newTestStore(t)
	req := &models.Requirement{Identifier: "REQ-REP-1", Title: "R"}
	require.NoError(t, s.CreateRequirement(req))

	cost1, cost2 := 0.02, 0.04
	completed := &models.AIGenerationRun{
		RequirementID: req.ID, Status: models.AIGenerationRunStatusCompleted,
		ProviderLabel: "P1", ModelName: "m1", TotalTokens: 100, DurationMs: 1000,
		EstimatedCost: &cost1, RetryCount: 1,
	}
	_, _, err := s.CreateGenerationRun(completed)
	require.NoError(t, err)
	require.NoError(t, s.UpdateGenerationRun(completed))

	failed := &models.AIGenerationRun{
		RequirementID: req.ID, Status: models.AIGenerationRunStatusFailed,
		ProviderLabel: "P1", ModelName: "m1", ErrorCategory: "parse",
		EstimatedCost: &cost2, DurationMs: 2000,
	}
	_, _, err = s.CreateGenerationRun(failed)
	require.NoError(t, err)
	require.NoError(t, s.UpdateGenerationRun(failed))

	// Drafts: accepted-unchanged, accepted-edited, rejected (with event), pending.
	mk := func(status string, edited bool) *models.AIGeneratedDraft {
		d := &models.AIGeneratedDraft{RunID: completed.ID}
		require.NoError(t, d.ApplyContent(models.DraftContent{
			Name: "n", Steps: []models.GeneratedStep{{Action: "a", ExpectedResult: "e"}},
		}))
		require.NoError(t, s.CreateGenerationDrafts(completed.ID, nil, []*models.AIGeneratedDraft{d}, 0))
		if status != models.AIDraftStatusPending || edited {
			d.Status = status
			d.Edited = edited
			require.NoError(t, s.db.Save(d).Error)
		}
		return d
	}
	acceptedDup := mk(models.AIDraftStatusAccepted, false)
	require.NoError(t, s.db.Model(&models.AIGeneratedDraft{}).
		Where("id = ?", acceptedDup.ID).
		Update("duplicates_json", `[{"kind":"existing","similarity":0.95,"name":"x","reason":"r"}]`).Error)
	mk(models.AIDraftStatusAccepted, true)
	rejected := mk(models.AIDraftStatusRejected, false)
	mk(models.AIDraftStatusPending, false)
	ev := &models.AIGenerationEvent{
		RunID: completed.ID, DraftID: &rejected.ID,
		EventType: models.AIGenEventRejected, Reason: "too_vague",
	}
	require.NoError(t, s.AppendGenerationEvent(ev))

	// Exercise the real decision-latency and coverage-recall arithmetic (not
	// just their zero/guard paths): give the completed run a completed_at and
	// a coverage report, and pin the reject event to a known offset after it.
	base := time.Now().Add(-10 * time.Minute)
	require.NoError(t, s.db.Model(&models.AIGenerationRun{}).Where("id = ?", completed.ID).
		Updates(map[string]interface{}{
			"created_at":    base,
			"completed_at":  base,
			"coverage_json": `{"uncovered_count":3}`,
		}).Error)
	require.NoError(t, s.db.Model(&models.AIGenerationEvent{}).Where("id = ?", ev.ID).
		Update("created_at", base.Add(90*time.Second)).Error)

	rep, err := s.GetAIGenerationReport(time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)

	assert.Equal(t, 2, rep.Runs.Total)
	assert.Equal(t, 1, rep.Runs.Completed)
	assert.Equal(t, 1, rep.Runs.Failed)
	assert.Equal(t, 1, rep.Runs.ParseFailures)
	assert.Equal(t, 1, rep.Runs.RetriedRuns)
	assert.InDelta(t, 0.06, rep.Runs.TotalCostUSD, 1e-9)
	assert.Equal(t, int64(1000), rep.Runs.AvgDurationMs, "avg over completed runs only")
	assert.Equal(t, int64(1000), rep.Runs.P50DurationMs)
	assert.Equal(t, int64(1000), rep.Runs.P95DurationMs)
	assert.InDelta(t, 90.0, rep.Runs.AvgDecisionSeconds, 0.5, "completed_at -> reject event is 90s")
	assert.InDelta(t, 3.0, rep.Runs.AvgUncoveredTargets, 1e-9, "one completed run, uncovered_count=3")

	assert.Equal(t, 4, rep.Drafts.Generated)
	assert.Equal(t, 1, rep.Drafts.AcceptedUnchanged)
	assert.Equal(t, 1, rep.Drafts.AcceptedEdited)
	assert.Equal(t, 1, rep.Drafts.Rejected)
	assert.Equal(t, 1, rep.Drafts.Pending)
	assert.Equal(t, 1, rep.Drafts.AcceptedWithDupWarning, "duplicate-override count")
	assert.Equal(t, map[string]int{"too_vague": 1}, rep.RejectionReasons)

	require.Len(t, rep.Providers, 1)
	assert.Equal(t, "P1", rep.Providers[0].ProviderLabel)
	assert.Equal(t, 2, rep.Providers[0].Runs)

	// Empty window.
	rep, err = s.GetAIGenerationReport(time.Now().Add(2*time.Hour), time.Now().Add(3*time.Hour))
	require.NoError(t, err)
	assert.Zero(t, rep.Runs.Total)
	assert.NotNil(t, rep.RejectionReasons)
	assert.NotNil(t, rep.Providers)
}
