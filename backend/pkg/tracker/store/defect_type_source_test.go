package store

import (
	"fmt"
	"testing"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

type labelSeed struct {
	status  models.ExecutionStatus
	defect  string
	source  string
	decided bool
}

// seedLabel adds a result in the given label state and returns its id.
func seedLabel(t *testing.T, s *Store, runID string, n int, l labelSeed) string {
	t.Helper()
	rr := &models.RunResult{TestRunID: runID, TestNameSnapshot: fmt.Sprintf("l%d", n), AttemptNumber: n,
		Status: l.status, ErrorMessage: "boom", DefectType: l.defect, DefectTypeSource: l.source}
	if l.decided {
		at := time.Now().UTC().Add(-time.Hour)
		rr.DecidedAt = &at
	}
	require.NoError(t, s.AddRunResult(rr))
	return rr.ID
}

func resultRow(t *testing.T, s *Store, id string) *models.RunResult {
	t.Helper()
	got, err := s.GetRunResultByID(id)
	require.NoError(t, err)
	require.NotNil(t, got)
	return got
}

func labelOf(t *testing.T, s *Store, id string) (defect, source string) {
	t.Helper()
	got := resultRow(t, s, id)
	return got.DefectType, got.DefectTypeSource
}

// asPerson writes what the single-result triage handler writes for a person's explicit choice on
// a result without an analysis: the R2 fields and a cleared snapshot (decided_at stays NULL).
func asPerson(value string) map[string]interface{} {
	upd := HumanDefectTypeFields(value, true)
	upd["suggested_defect_type"], upd["decided_at"] = "", nil
	return upd
}

func TestHumanDefectTypeFields(t *testing.T) {
	explicit := HumanDefectTypeFields("system_issue", true)
	require.Equal(t, "system_issue", explicit["defect_type"])
	require.Equal(t, models.DefectTypeSourceHuman, explicit["defect_type_source"], "a person's choice")
	require.Contains(t, explicit, "suggested_auto_applied", "R10: every write records whether it confirmed an AI label")

	auto := HumanDefectTypeFields("to_investigate", false)
	require.Equal(t, "", auto["defect_type_source"], "the status-change default and clears are nobody's choice")
	require.Contains(t, auto, "suggested_auto_applied")

	require.Equal(t, map[string]interface{}{"defect_type": "product_bug", "defect_type_source": models.DefectTypeSourceHuman},
		DefectTypePatch("product_bug", true), "the live patch carries no SQL expression")
	require.Equal(t, map[string]interface{}{"defect_type": "", "defect_type_source": ""}, DefectTypePatch("", false))
}

// R10: the write records whether the label it replaced was set by auto-apply — a Confirm.
func TestTriageWriteRecordsWhetherItConfirmedAnAILabel(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	id := seedLabel(t, s, runID, 1, labelSeed{status: models.StatusFail})
	n, err := s.ApplyAutoDefectType([]string{id}, "product_bug")
	require.NoError(t, err)
	require.EqualValues(t, 1, n)

	_, err = s.TriageRunResult(runID, id, HumanDefectTypeFields("product_bug", true))
	require.NoError(t, err)
	got := resultRow(t, s, id)
	require.True(t, got.SuggestedAutoApplied, "the person confirmed an AI label")
	require.Equal(t, models.DefectTypeSourceHuman, got.DefectTypeSource)

	_, err = s.TriageRunResult(runID, id, HumanDefectTypeFields("automation_bug", true))
	require.NoError(t, err)
	got = resultRow(t, s, id)
	require.False(t, got.SuggestedAutoApplied, "a later change of mind replaces a person's label")

	fresh := seedLabel(t, s, runID, 2, labelSeed{status: models.StatusFail})
	_, err = s.TriageRunResult(runID, fresh, HumanDefectTypeFields("product_bug", true))
	require.NoError(t, err)
	require.False(t, resultRow(t, s, fresh).SuggestedAutoApplied)
}

func TestApplyAutoDefectType_WritesOnlyWhereNoPersonDecided(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	fail := models.StatusFail
	untriaged := seedLabel(t, s, runID, 1, labelSeed{status: fail})
	defaulted := seedLabel(t, s, runID, 2, labelSeed{status: fail, defect: "to_investigate"})
	errored := seedLabel(t, s, runID, 3, labelSeed{status: models.StatusError})
	earlierAI := seedLabel(t, s, runID, 4, labelSeed{status: fail, defect: "system_issue", source: models.DefectTypeSourceAI})
	human := seedLabel(t, s, runID, 5, labelSeed{status: fail, defect: "product_bug", source: models.DefectTypeSourceHuman})
	legacyHuman := seedLabel(t, s, runID, 6, labelSeed{status: fail, defect: "product_bug"}) // triaged before Wave 3
	sentBack := seedLabel(t, s, runID, 7, labelSeed{status: fail, defect: "to_investigate", source: models.DefectTypeSourceHuman, decided: true})
	passed := seedLabel(t, s, runID, 8, labelSeed{status: models.StatusPass})

	n, err := s.ApplyAutoDefectType([]string{untriaged, defaulted, errored, earlierAI, human, legacyHuman, sentBack, passed}, "automation_bug")
	require.NoError(t, err)
	require.EqualValues(t, 4, n)
	for _, id := range []string{untriaged, defaulted, errored, earlierAI} {
		d, src := labelOf(t, s, id)
		require.Equal(t, "automation_bug", d, id)
		require.Equal(t, models.DefectTypeSourceAI, src, id)
	}
	for _, id := range []string{human, legacyHuman} {
		d, _ := labelOf(t, s, id)
		require.Equal(t, "product_bug", d, "a person's label is never overwritten")
	}
	d, _ := labelOf(t, s, sentBack)
	require.Equal(t, "to_investigate", d, "a person who sent it back to the untriaged pile decided")
	d, _ = labelOf(t, s, passed)
	require.Equal(t, "", d, "a passing result carries no defect type")

	got := resultRow(t, s, untriaged)
	require.Nil(t, got.DecidedAt, "an AI label is not a decision")
	require.Empty(t, got.SuggestedDefectType, "and snapshots nothing")
}

// Header amendment 1: a person who picks "to investigate" on a result WITHOUT an analysis leaves
// decided_at NULL (the snapshot is cleared), and still no later analysis may label it.
func TestApplyAutoDefectType_NeverOverAnExplicitToInvestigateWithoutAnAnalysis(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	id := seedLabel(t, s, runID, 1, labelSeed{status: models.StatusFail})
	_, err := s.TriageRunResult(runID, id, asPerson("to_investigate"))
	require.NoError(t, err)
	got := resultRow(t, s, id)
	require.Nil(t, got.DecidedAt, "no analysis, so no decision instant")
	require.Equal(t, models.DefectTypeSourceHuman, got.DefectTypeSource)

	n, err := s.ApplyAutoDefectType([]string{id}, "product_bug")
	require.NoError(t, err)
	require.Zero(t, n, "a later qualifying analysis never labels it")
	d, src := labelOf(t, s, id)
	require.Equal(t, "to_investigate", d)
	require.Equal(t, models.DefectTypeSourceHuman, src)
}

func TestApplyAutoDefectType_RefusesAnInconclusiveValue(t *testing.T) {
	s := newTestStore(t)
	for _, v := range []string{"", "to_investigate", "insufficient_evidence"} {
		_, err := s.ApplyAutoDefectType([]string{"x"}, v)
		require.Error(t, err, v)
	}
	n, err := s.ApplyAutoDefectType(nil, "product_bug")
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestResetAutoDefectType_TouchesOnlyAILabels(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	aiRow := seedLabel(t, s, runID, 1, labelSeed{status: models.StatusFail, defect: "product_bug", source: models.DefectTypeSourceAI})
	human := seedLabel(t, s, runID, 2, labelSeed{status: models.StatusFail, defect: "product_bug", source: models.DefectTypeSourceHuman})

	n, err := s.ResetAutoDefectType([]string{aiRow, human})
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	d, src := labelOf(t, s, aiRow)
	require.Equal(t, "to_investigate", d)
	require.Equal(t, "", src)
	d, src = labelOf(t, s, human)
	require.Equal(t, "product_bug", d)
	require.Equal(t, models.DefectTypeSourceHuman, src)
}

// R3: a person's write between the decision and the apply wins, because it marks the label as a
// person's (and, with an analysis, sets decided_at), which both auto predicates exclude.
func TestAutoDefectType_APersonsWriteInBetweenWins(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	id := seedLabel(t, s, runID, 1, labelSeed{status: models.StatusFail})
	n, err := s.ApplyAutoDefectType([]string{id}, "product_bug")
	require.NoError(t, err)
	require.EqualValues(t, 1, n)

	upd := HumanDefectTypeFields("automation_bug", true)
	upd["decided_at"] = time.Now().UTC()
	landed, err := s.TriageRunResult(runID, id, upd)
	require.NoError(t, err)
	require.EqualValues(t, 1, landed)

	n, err = s.ApplyAutoDefectType([]string{id}, "system_issue")
	require.NoError(t, err)
	require.Zero(t, n, "a later decision never follows over a person's label")
	n, err = s.ResetAutoDefectType([]string{id})
	require.NoError(t, err)
	require.Zero(t, n, "and never resets it")
	d, src := labelOf(t, s, id)
	require.Equal(t, "automation_bug", d)
	require.Equal(t, models.DefectTypeSourceHuman, src)
}

// The generic writers treat a map that writes defect_type without a source as a non-explicit
// write (backstop): the AI source is cleared and the R10 flag recorded.
func TestRunResultWriters_ClearTheSourceWhenTheyWriteDefectType(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	ids := make([]string, 4)
	for i := range ids {
		ids[i] = seedLabel(t, s, runID, i+1, labelSeed{status: models.StatusFail, defect: "product_bug", source: models.DefectTypeSourceAI})
	}
	require.NoError(t, s.UpdateRunResult(runID, ids[0], map[string]interface{}{"defect_type": "to_investigate"}))
	_, err := s.TriageRunResult(runID, ids[1], map[string]interface{}{"defect_type": "to_investigate"})
	require.NoError(t, err)
	_, err = s.BulkUpdateRunResults(runID, []string{ids[2]}, map[string]interface{}{"defect_type": "to_investigate"})
	require.NoError(t, err)
	_, err = s.BulkTriageRunResults(runID, []string{ids[3]}, map[string]interface{}{"defect_type": "to_investigate"})
	require.NoError(t, err)
	for _, id := range ids {
		got := resultRow(t, s, id)
		require.Equal(t, "to_investigate", got.DefectType, id)
		require.Equal(t, "", got.DefectTypeSource, id)
		require.True(t, got.SuggestedAutoApplied, id)
	}

	other := seedLabel(t, s, runID, 9, labelSeed{status: models.StatusFail, defect: "product_bug", source: models.DefectTypeSourceAI})
	require.NoError(t, s.UpdateRunResult(runID, other, map[string]interface{}{"log_text": "more"}))
	d, src := labelOf(t, s, other)
	require.Equal(t, "product_bug", d)
	require.Equal(t, models.DefectTypeSourceAI, src, "a write that leaves defect_type alone leaves its source alone")
}

// AI labels never write decided_at, so the accuracy report and the few-shot examples — both of
// which require it — never see them, even if suggestion columns were somehow on the row.
func TestAILabelsNeverReachAccuracyOrExamples(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	direct, score := false, 0.99
	snap := func(id, human string, decided *time.Time) {
		upd := map[string]interface{}{
			"suggested_verdict": models.VerdictProductBug, "suggested_defect_type": "product_bug",
			"suggested_confidence": models.ConfidenceHigh, "suggested_engine": models.AnalysisEngineTypeSafe,
			"suggested_policy_version": failureanalysis.PolicyVersionNoExamples, "suggested_is_clone": &direct,
			"suggested_confidence_score": &score, "decided_at": decided,
		}
		if human != "" {
			for k, v := range HumanDefectTypeFields(human, true) {
				upd[k] = v
			}
		}
		require.NoError(t, s.UpdateRunResult(runID, id, upd))
	}
	decided := time.Now().UTC().Add(-time.Hour)
	human := seedLabel(t, s, runID, 1, labelSeed{status: models.StatusFail})
	snap(human, "product_bug", &decided)
	aiRow := seedLabel(t, s, runID, 2, labelSeed{status: models.StatusFail})
	snap(aiRow, "", nil)
	n, err := s.ApplyAutoDefectType([]string{aiRow}, "system_issue")
	require.NoError(t, err)
	require.EqualValues(t, 1, n)

	acc, err := s.GetFailureAnalysisAccuracy(time.Now().UTC().AddDate(0, 0, -30))
	require.NoError(t, err)
	require.Equal(t, 1, acc.Total, "only the person's decision is graded")

	ex, err := s.ListTriageExamples(failureanalysis.TriageExampleFilter{ExcludeRunID: "another-run",
		Before: time.Now().UTC().Add(time.Hour), Since: time.Now().UTC().AddDate(0, 0, -90), Limit: 8})
	require.NoError(t, err)
	require.Len(t, ex, 1)
	require.Equal(t, human, ex[0].ResultID, "an AI label is never a few-shot example")
}
