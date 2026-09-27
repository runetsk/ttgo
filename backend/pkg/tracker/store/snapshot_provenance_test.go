package store

import (
	"path/filepath"
	"testing"
	"time"

	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// TestSnapshotProvenanceBackfill_ConservativeOneShot boots a DB whose run_results predate the
// provenance columns and checks the backfill: filled only when exactly one analysis explains the
// snapshot and it is the version that was current at the decision; everything else stays NULL;
// and a later boot never runs it again.
func TestSnapshotProvenanceBackfill_ConservativeOneShot(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir) // bootstrap creates backups/ in cwd
	dsn := filepath.Join(dir, "ttgo.db")
	s, err := New(dsn)
	require.NoError(t, err)

	run := &models.TestRun{Name: "legacy"}
	require.NoError(t, s.CreateTestRun(run))
	decided := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	before, after := decided.Add(-time.Hour), decided.Add(time.Hour)

	newResult := func(name string) *models.RunResult {
		rr := &models.RunResult{TestRunID: run.ID, TestNameSnapshot: name, AttemptNumber: 1, Status: models.StatusFail, ErrorMessage: "boom"}
		require.NoError(t, s.AddRunResult(rr))
		return rr
	}
	addAnalysis := func(rrID string, a models.RunResultAnalysis, at time.Time) *models.RunResultAnalysis {
		a.RunResultID = rrID
		row, err := s.CreateAnalysis(&a)
		require.NoError(t, err)
		require.NoError(t, s.db.Exec(`UPDATE run_result_analyses SET created_at = ? WHERE id = ?`, at, row.ID).Error)
		return row
	}
	snapshot := func(rr *models.RunResult, verdict, defect, conf, engine string, score *float64) {
		require.NoError(t, s.db.Exec(`UPDATE run_results SET defect_type = 'product_bug', suggested_verdict = ?,
			suggested_defect_type = ?, suggested_confidence = ?, suggested_engine = ?, suggested_confidence_score = ?,
			decided_at = ? WHERE id = ?`, verdict, defect, conf, engine, score, decided, rr.ID).Error)
	}
	flaky := models.RunResultAnalysis{Verdict: "flaky_test", Confidence: "high", ModelName: "m", SuggestedDefectType: "automation_bug"}
	productBug := models.RunResultAnalysis{Verdict: "product_bug", Confidence: "high", ModelName: "m", SuggestedDefectType: "product_bug"}
	ts := models.RunResultAnalysis{Verdict: "flaky_test", Confidence: "medium", ModelName: "jev", Engine: models.AnalysisEngineTypeSafe,
		ConfidenceScore: fptr(0.7), SuggestedDefectType: "automation_bug", SuggestedDefectTypeConfidence: fptr(0.93), PolicyVersion: "fa-verdict-v4"}

	// Direct generative prediction, the only analysis before the decision: filled, direct.
	direct := newResult("direct")
	addAnalysis(direct.ID, flaky, before)
	snapshot(direct, "flaky_test", "automation_bug", "high", "generative", nil)

	// TypeSafe representative and its clone: filled with the policy, clone status from source_analysis_id.
	rep := newResult("rep")
	repRow := addAnalysis(rep.ID, ts, before)
	snapshot(rep, "flaky_test", "automation_bug", "high", "typesafe", fptr(0.93))
	clone := newResult("clone")
	cl := ts
	cl.SourceAnalysisID = &repRow.ID
	addAnalysis(clone.ID, cl, before)
	snapshot(clone, "flaky_test", "automation_bug", "high", "typesafe", fptr(0.93))

	// Two identical versions before the decision: ambiguous, stays unknown.
	amb := newResult("ambiguous")
	addAnalysis(amb.ID, flaky, before.Add(-time.Hour))
	addAnalysis(amb.ID, flaky, before)
	snapshot(amb, "flaky_test", "automation_bug", "high", "generative", nil)

	// The only analysis was written after the decision: stays unknown.
	late := newResult("late")
	addAnalysis(late.ID, flaky, after)
	snapshot(late, "flaky_test", "automation_bug", "high", "generative", nil)

	// The version current at the decision does not match; an older one does: stays unknown.
	stale := newResult("stale")
	addAnalysis(stale.ID, flaky, before.Add(-time.Hour))
	addAnalysis(stale.ID, productBug, before)
	snapshot(stale, "flaky_test", "automation_bug", "high", "generative", nil)

	// The pre-Wave-1 schema: the two columns do not exist yet.
	require.NoError(t, s.db.Exec(`ALTER TABLE run_results DROP COLUMN suggested_is_clone`).Error)
	require.NoError(t, s.db.Exec(`ALTER TABLE run_results DROP COLUMN suggested_policy_version`).Error)
	require.NoError(t, s.Close())

	s, err = New(dsn)
	require.NoError(t, err)
	get := func(id string) *models.RunResult {
		rr, err := s.GetRunResultByID(id)
		require.NoError(t, err)
		return rr
	}
	for _, tc := range []struct {
		rr     *models.RunResult
		clone  *bool
		policy string
		why    string
	}{
		{direct, tsBool(false), "", "one matching analysis before the decision"},
		{rep, tsBool(false), "fa-verdict-v4", "TypeSafe representative"},
		{clone, tsBool(true), "fa-verdict-v4", "clone status comes from source_analysis_id"},
		{amb, nil, "", "two candidates: ambiguous"},
		{late, nil, "", "no analysis existed at the decision"},
		{stale, nil, "", "the current version at the decision does not match"},
	} {
		got := get(tc.rr.ID)
		require.Equal(t, tc.clone, got.SuggestedIsClone, tc.why)
		require.Equal(t, tc.policy, got.SuggestedPolicyVersion, tc.why)
	}

	// One-shot: once the column exists, a later boot does not run the backfill again.
	require.NoError(t, s.db.Exec(`UPDATE run_results SET suggested_is_clone = NULL WHERE id = ?`, direct.ID).Error)
	require.NoError(t, s.Close())
	s, err = New(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	require.Nil(t, get(direct.ID).SuggestedIsClone)
}
