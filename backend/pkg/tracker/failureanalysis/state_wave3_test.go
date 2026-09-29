package failureanalysis

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderState_RecentOutcomesAndRelatedFailures(t *testing.T) {
	in := baseContext() // redaction on
	state, _ := RenderState(BuildEvidenceWithBudget(in, TypeSafeBudget()))
	require.NotContains(t, state["history"].(map[string]any), "recent_outcomes", "absent, not empty, without earlier results")
	require.NotContains(t, state, "group", "absent without related failures")

	in.RecentOutcomes = "PPFPF"
	in.GroupMembers = []string{"socket hang up", "auth failed: token=hunter2secret"}
	ev := BuildEvidenceWithBudget(in, TypeSafeBudget())
	require.Equal(t, "PPFPF", ev.RecentOutcomes)
	state, meta := RenderState(ev)
	require.Empty(t, meta.TruncationPrefix)
	require.Equal(t, "PPFPF", state["history"].(map[string]any)["recent_outcomes"])
	require.Equal(t, []string{"socket hang up", "auth failed: token=<REDACTED>"},
		state["group"].(map[string]any)["related_failures"], "redacted and capped")
	require.Equal(t, BuildEvidence(in).GroupMembers, ev.GroupMembers, "the same lines the narrator receives (R6)")

	prompt, _, err := BuildPrompt(BuildEvidence(in).PromptInput(DefaultPromptTemplate))
	require.NoError(t, err)
	require.NotContains(t, prompt, "PPFPF", "recent outcomes are TypeSafe state only; the LLM prompt is unchanged")
}

func TestRenderState_DropLadderWave3(t *testing.T) {
	ev := Evidence{GroupMembers: []string{"m"}, Examples: []TriageExample{{ErrorMessage: "a"}}, LogText: "L",
		SimilarFailures: []SimilarFailure{{ErrorMessage: "x"}}, SimilarFailuresRollup: "r", RecentOutcomes: "PFP",
		Steps: []PromptStep{{Order: 1, Action: "a"}}}
	var order []string
	for {
		name, ok := dropNext(&ev)
		if !ok {
			break
		}
		order = append(order, name)
	}
	require.Equal(t, []string{"no related failures", "fewer examples", "no logs", "no similar failures", "no steps"}, order)
	require.Empty(t, ev.RecentOutcomes, "recent outcomes go with the history block")
	require.Empty(t, ev.SimilarFailuresRollup)

	onlyOutcomes := Evidence{RecentOutcomes: "PFP"}
	name, ok := dropNext(&onlyOutcomes)
	require.True(t, ok)
	require.Equal(t, "no similar failures", name, "dropped with the history block even when no failure is listed (R7)")
	require.Empty(t, onlyOutcomes.RecentOutcomes)
}

func TestRenderState_ReturnsTheEvidenceItSent(t *testing.T) {
	in := baseContext()
	in.GroupMembers = []string{"socket hang up"}
	in.RecentOutcomes = "PFP"
	ev := BuildEvidenceWithBudget(in, TypeSafeBudget())
	_, _, sent := renderState(ev)
	require.Equal(t, ev.GroupMembers, sent.GroupMembers, "nothing dropped: everything sent")

	without := ev
	without.GroupMembers = nil
	b, _ := json.Marshal(stateObject(without))
	ev.StateCap = len(b)
	state, meta, sent := renderState(ev)
	require.Nil(t, sent.GroupMembers, "the ladder dropped the related failures first")
	require.Equal(t, "PFP", sent.RecentOutcomes)
	require.NotContains(t, state, "group")
	require.Equal(t, "[context: no related failures] ", meta.TruncationPrefix)
}

// At every TypeSafe cap with the maximum of maxed examples and member lines (JSON-escaped to six
// bytes a character), the bound holds and the related failures go first.
func TestRenderState_TypeSafeCapHoldsWithRelatedFailures(t *testing.T) {
	in := maxedContext()
	in.Examples = maxedExamples()
	in.RecentOutcomes = strings.Repeat("P", RecentOutcomesLimit)
	for i := 0; i < GroupMembersMax; i++ {
		in.GroupMembers = append(in.GroupMembers, string(rune('a'+i))+strings.Repeat("<", GroupMemberMsgCap))
	}
	ev := BuildEvidenceWithBudget(in, TypeSafeBudget())
	require.Len(t, ev.GroupMembers, GroupMembersMax)
	state, _ := RenderState(ev)
	b, _ := json.Marshal(state)
	require.LessOrEqual(t, len(b), TypeSafeStateCharCap)

	in.Result.ErrorMessage = strings.Repeat("E", TypeSafeErrorHeadCap+100)
	in.Result.StackTrace = strings.Repeat("T", TypeSafeStackHeadCap+100)
	in.Result.LogText = strings.Repeat("L", TypeSafeLogTailCap+100)
	state, meta := RenderState(BuildEvidenceWithBudget(in, TypeSafeBudget()))
	b, _ = json.Marshal(state)
	require.LessOrEqual(t, len(b), TypeSafeStateCharCap)
	require.NotContains(t, state, "group")
	require.True(t, strings.HasPrefix(meta.TruncationPrefix, "[context: no related failures; fewer examples; no logs"), meta.TruncationPrefix)
	// Without the log the state is ≈58k characters (error and stack 16k each, 30 steps ≈19k,
	// defects ≈4.5k): the history block, and recent_outcomes with it, stays.
	require.Equal(t, strings.Repeat("P", RecentOutcomesLimit), state["history"].(map[string]any)["recent_outcomes"])
}
