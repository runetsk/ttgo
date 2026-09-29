package store

import (
	"fmt"
	"testing"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// gradeSpec is one triage decision as the real single-result handler records it for a stored
// TypeSafe analysis: the R2 fields (HumanDefectTypeFields(human, true)) plus the snapshot columns
// snapshotValues writes, in ONE update (runs.go UpdateRunResult). The store package cannot import
// the handler; the API tests of this task go through the handler itself.
type gradeSpec struct {
	status     models.ExecutionStatus
	human      string
	suggested  string
	engine     string
	policy     string
	clone      *bool
	score      *float64
	decidedAt  *time.Time // nil = the snapshot could not be recorded
	aiLabelled bool       // auto-apply had labelled the result before the person wrote (R10)
	legacy     bool       // decided before Wave 3: the write carried no source
	untriaged  bool       // an AI label nobody triaged
}

type gateSeeder struct {
	t     *testing.T
	s     *Store
	runID string
	n     int
}

func newGateSeeder(t *testing.T, s *Store) *gateSeeder {
	return &gateSeeder{t: t, s: s, runID: seedRun(t, s)}
}

// add records one decision: by default a person agreeing with a direct TypeSafe v7 suggestion of
// product_bug at 0.97, decided a day ago; edit changes one thing.
func (g *gateSeeder) add(edit func(sp *gradeSpec)) {
	g.t.Helper()
	g.n++
	direct, score, decided := false, 0.97, time.Now().UTC().Add(-24*time.Hour)
	sp := gradeSpec{status: models.StatusFail, human: "product_bug", suggested: "product_bug",
		engine: models.AnalysisEngineTypeSafe, policy: failureanalysis.PolicyVersionNoExamples,
		clone: &direct, score: &score, decidedAt: &decided}
	if edit != nil {
		edit(&sp)
	}
	rr := &models.RunResult{TestRunID: g.runID, TestNameSnapshot: fmt.Sprintf("g%d", g.n), AttemptNumber: g.n,
		Status: models.StatusFail, ErrorMessage: "boom"}
	require.NoError(g.t, g.s.AddRunResult(rr))
	if sp.aiLabelled || sp.untriaged {
		n, err := g.s.ApplyAutoDefectType([]string{rr.ID}, sp.suggested)
		require.NoError(g.t, err)
		require.EqualValues(g.t, 1, n)
	}
	if sp.untriaged {
		return
	}
	upd := HumanDefectTypeFields(sp.human, true)
	if sp.legacy {
		upd["defect_type_source"] = "" // what a pre-Wave-3 decision reads as after the migration
		delete(upd, "suggested_auto_applied")
	}
	upd["status"] = sp.status
	upd["suggested_verdict"] = models.VerdictProductBug
	upd["suggested_defect_type"] = sp.suggested
	upd["suggested_confidence"] = models.ConfidenceHigh
	upd["suggested_engine"] = sp.engine
	upd["suggested_confidence_score"] = sp.score
	upd["suggested_policy_version"] = sp.policy
	upd["suggested_is_clone"] = sp.clone
	upd["decided_at"] = sp.decidedAt
	require.NoError(g.t, g.s.UpdateRunResult(g.runID, rr.ID, upd))
}

func TestAutoApplyGate_CountsOnlyCurrentPolicyDirectTypeSafeAtTheThreshold(t *testing.T) {
	s := newTestStore(t)
	g := newGateSeeder(t, s)
	g.add(nil)                                                                           // graded, agreed ('human' source, as every explicit triage writes)
	g.add(func(sp *gradeSpec) { sp.policy = failureanalysis.PolicyVersionWithExamples }) // graded, agreed
	g.add(func(sp *gradeSpec) { sp.human = "automation_bug" })                           // graded, disagreed
	g.add(func(sp *gradeSpec) { sp.status = models.StatusError })                        // graded, agreed
	g.add(func(sp *gradeSpec) { edge := 0.95; sp.score = &edge })                        // graded at the threshold
	g.add(func(sp *gradeSpec) { sp.legacy = true })                                      // graded: decided before Wave 3
	// Excluded, one reason each.
	g.add(func(sp *gradeSpec) { sp.engine = models.AnalysisEngineTypeSafeDerived })
	g.add(func(sp *gradeSpec) { sp.engine = models.AnalysisEngineGenerative })
	g.add(func(sp *gradeSpec) { clone := true; sp.clone = &clone })
	g.add(func(sp *gradeSpec) { sp.clone = nil })
	g.add(func(sp *gradeSpec) { sp.policy = "fa-verdict-v6" })
	g.add(func(sp *gradeSpec) { low := 0.94; sp.score = &low })
	g.add(func(sp *gradeSpec) { sp.score = nil })
	g.add(func(sp *gradeSpec) { old := time.Now().UTC().AddDate(0, 0, -91); sp.decidedAt = &old })
	g.add(func(sp *gradeSpec) { sp.human = "to_investigate" })
	g.add(func(sp *gradeSpec) { sp.decidedAt = nil })
	g.add(func(sp *gradeSpec) { sp.status = models.StatusPass })
	g.add(func(sp *gradeSpec) { sp.suggested = "" })
	g.add(func(sp *gradeSpec) { sp.aiLabelled = true })                            // a Confirm of an AI label (R10)
	g.add(func(sp *gradeSpec) { sp.aiLabelled = true; sp.human = "system_issue" }) // a correction of an AI label (R10)
	g.add(func(sp *gradeSpec) { sp.untriaged = true })                             // an AI label nobody triaged

	gate, err := s.AutoApplyGate(0.95)
	require.NoError(t, err)
	require.Equal(t, 6, gate.Graded)
	require.Equal(t, 5, gate.Agreed)
	require.InDelta(t, 5.0/6, gate.Accuracy, 1e-9)
	require.False(t, gate.Open, "too few graded")
	require.Equal(t, []string{failureanalysis.PolicyVersionNoExamples, failureanalysis.PolicyVersionWithExamples}, gate.Policies)
	require.InDelta(t, 0.95, gate.MinConfidence, 1e-12)

	lower, err := s.AutoApplyGate(0.90)
	require.NoError(t, err)
	require.Equal(t, 7, lower.Graded, "the 0.94 row counts at a 90% threshold")
}

func TestAutoApplyGate_OpensAtFiftyGradedAndNinetyFivePercent(t *testing.T) {
	s := newTestStore(t)
	g := newGateSeeder(t, s)
	for i := 0; i < 49; i++ {
		g.add(nil)
	}
	gate, err := s.AutoApplyGate(0.95)
	require.NoError(t, err)
	require.False(t, gate.Open, "49 graded is too few")

	for i := 0; i < 5; i++ {
		g.add(func(sp *gradeSpec) { sp.aiLabelled = true })
	}
	gate, err = s.AutoApplyGate(0.95)
	require.NoError(t, err)
	require.Equal(t, 49, gate.Graded, "Confirms of AI labels never open the gate")

	g.add(func(sp *gradeSpec) { sp.human = "system_issue" })
	gate, err = s.AutoApplyGate(0.95)
	require.NoError(t, err)
	require.Equal(t, 50, gate.Graded)
	require.True(t, gate.Open, "49/50 = 98%")

	g.add(func(sp *gradeSpec) { sp.human = "system_issue" })
	g.add(func(sp *gradeSpec) { sp.human = "system_issue" })
	gate, err = s.AutoApplyGate(0.95)
	require.NoError(t, err)
	require.False(t, gate.Open, "49/52 is below 95%")
}

func TestFailureAnalysisSettings_AutoApplyDefaultsAndWrite(t *testing.T) {
	s := newTestStore(t)
	got, err := s.GetFailureAnalysisSettings()
	require.NoError(t, err)
	require.False(t, got.AutoApplyDefectType, "off on a new install")
	require.Equal(t, models.DefaultAutoApplyMinConfidence, got.AutoApplyMinConfidence)

	got, err = s.SetFailureAnalysisAutoApply(true, 90)
	require.NoError(t, err)
	require.True(t, got.AutoApplyDefectType)
	require.Equal(t, 90, got.AutoApplyMinConfidence)

	got.MaxAnalysesPerRun = 7
	got, err = s.UpdateFailureAnalysisSettings(got)
	require.NoError(t, err)
	require.True(t, got.AutoApplyDefectType, "the general update never touches auto-apply")

	_, err = s.SetFailureAnalysisAutoApply(true, 100)
	require.Error(t, err)
}
