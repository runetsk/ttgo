package failureanalysis

import (
	"context"
	"fmt"
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func TestGroupMemberErrors_DistinctOthersOnly(t *testing.T) {
	rep := &models.RunResult{ID: "r0", FailureType: "timeout", ErrorMessage: "Timeout after 5000ms on #checkout"}
	members := []*models.RunResult{
		rep,
		{ID: "r1", FailureType: "timeout", ErrorMessage: "Timeout after 5000ms on #checkout"}, // same signature as rep
		{ID: "r2", FailureType: "timeout", ErrorMessage: "Timeout after 7000ms on #checkout"},
		{ID: "r3", FailureType: "timeout", ErrorMessage: "Timeout after 7000ms on #checkout"}, // duplicate of r2
		{ID: "r4", FailureType: "timeout", ErrorMessage: "   "},
	}
	require.Equal(t, []string{"Timeout after 7000ms on #checkout"}, GroupMemberErrors(rep, members))

	var many []*models.RunResult
	for i := 0; i < 9; i++ {
		many = append(many, &models.RunResult{ID: fmt.Sprintf("m%d", i), FailureType: "timeout", ErrorMessage: fmt.Sprintf("distinct failure %c", 'a'+i)})
	}
	require.Len(t, GroupMemberErrors(rep, many), MaxGroupMemberErrors)
	require.Nil(t, GroupMemberErrors(rep, []*models.RunResult{rep}), "a single-member group adds nothing")
	require.Nil(t, GroupMemberErrors(nil, many))
}

func TestDecidedFromRow_NarratesTheStoredDecision(t *testing.T) {
	score := 0.93
	stored := &models.RunResultAnalysis{Engine: models.AnalysisEngineTypeSafe, Verdict: models.VerdictFlakyTest,
		Confidence: models.ConfidenceHigh, ConfidenceScore: &score, SuggestedDefectType: "automation_bug",
		VerdictProbabilities: `{"flaky_test":0.52,"product_bug":0.45}`, NarrativeStatus: models.NarrativeStatusSkipped,
		DecisionStatus: models.DecisionStatusOK, PolicyVersion: "fa-verdict-v5", HistoryAvailable: true}
	decided := DecidedFromRow(stored)
	require.Equal(t, models.NarrativeStatusPending, decided.NarrativeStatus, "a stored decision is narrated as a pending one")
	require.Equal(t, models.VerdictFlakyTest, decided.Verdict)
	require.Equal(t, "automation_bug", decided.SuggestedDefectType)
	require.True(t, decided.HistoryAvailable)

	prov := &recordingProvider{stubProvider: stubProvider{responses: []string{`{"summary":"S","next_action":"N","rationale":"R"}`}}}
	d, ok := Narrate(context.Background(), AnalyzeDeps{Narrative: prov, NarrativeModel: "m"}, baseContext(), decided)
	require.True(t, ok)
	require.Equal(t, models.NarrativeStatusOK, d.NarrativeStatus)
	require.Equal(t, "S", d.Summary)
	sys := prov.reqs[0].Messages[0].Content
	require.Contains(t, sys, "`flaky_test` (confidence 0.93)")
	require.Contains(t, sys, "runner-up verdict was `product_bug`", "stored probabilities feed the runner-up hint")
	require.Contains(t, sys, "at most two pieces of evidence", "the short contract is used")
}
