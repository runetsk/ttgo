package failureanalysis

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func guarded(inj float64) Signals { return Signals{Injection: &inj} }

func TestRecordChecked_EveryBlockTheStateCarriedAsBuilt(t *testing.T) {
	in := baseContext()
	in.GroupMembers = []string{"socket hang up"}
	in.LinkedRequirements = []LinkedRequirement{{Key: "REQ-1", Title: "t"}}
	ev := BuildEvidenceWithBudget(in, TypeSafeBudget())
	s := guarded(0.02)
	s.RecordChecked(ev, ev)
	require.Equal(t, []string{BlockTest, BlockSteps, BlockFailureError, BlockFailureStack, BlockFailureLog,
		BlockHistory, BlockExamples, BlockDefects, BlockRelatedFailures}, s.CheckedBlocks,
		"empty blocks the state carried count; requirements are never in the state")
	require.True(t, s.MembersChecked)
	require.Len(t, s.BlockHashes, len(s.CheckedBlocks))
	require.Len(t, s.EvidenceHash, 64)
	require.Equal(t, s, s.VerifiedAgainst(ev), "same evidence: everything is still checked")
}

func TestRecordChecked_DroppedPartialAndHardCappedBlocksAreNotChecked(t *testing.T) {
	built := Evidence{TestName: "t", ErrorMessage: strings.Repeat("E", 2000), StackTrace: "s", LogText: "L",
		SimilarFailures: []SimilarFailure{{ErrorMessage: "x"}},
		Examples:        []TriageExample{{ErrorMessage: "a"}, {ErrorMessage: "b"}},
		GroupMembers:    []string{"m"}}
	sent := built
	sent.GroupMembers, sent.LogText, sent.SimilarFailures = nil, "", nil // dropped by the ladder
	sent.Examples = built.Examples[:1]                                   // partly dropped
	sent.ErrorMessage = built.ErrorMessage[:HardCapErrorChars]           // hard-capped
	s := guarded(0.02)
	s.RecordChecked(built, sent)
	require.Equal(t, []string{BlockTest, BlockSteps, BlockFailureStack, BlockDefects}, s.CheckedBlocks)
	require.False(t, s.MembersChecked)
}

func TestVerifiedAgainst_KeepsOnlyBlocksUnchangedSinceTheDecision(t *testing.T) {
	in := baseContext()
	in.GroupMembers = []string{"socket hang up"}
	in.SimilarFailures = []SimilarFailure{{Status: "FAIL", ErrorMessage: "expected 401, got 500", DefectType: "product_bug"}}
	ev := BuildEvidenceWithBudget(in, TypeSafeBudget())
	s := guarded(0.02)
	s.RecordChecked(ev, ev)

	later := in
	later.GroupMembers = []string{"a member line that was never checked"}
	later.SimilarFailures = []SimilarFailure{{Status: "FAIL", ErrorMessage: "expected 401, got 500", DefectType: "automation_bug"}}
	v := s.VerifiedAgainst(BuildEvidenceWithBudget(later, TypeSafeBudget()))
	require.NotContains(t, v.CheckedBlocks, BlockRelatedFailures)
	require.NotContains(t, v.CheckedBlocks, BlockHistory, "relabelled since the decision")
	require.Contains(t, v.CheckedBlocks, BlockFailureError)
	require.False(t, v.MembersChecked)
	require.Len(t, s.CheckedBlocks, 9, "the receiver is not rewritten")

	require.Equal(t, Signals{}, Signals{}.VerifiedAgainst(ev), "no guard: nothing to verify")
}

func TestOnlyBlocks_EmptiesUncheckedBlocksAndNamesThem(t *testing.T) {
	in := baseContext() // error, stack and log "log"; no steps, history, examples or defects
	in.GroupMembers = []string{"socket hang up"}
	in.LinkedRequirements = []LinkedRequirement{{Key: "REQ-1", Title: "t"}}
	ev := BuildEvidence(in)
	out := ev.OnlyBlocks([]string{BlockTest, BlockSteps, BlockFailureError, BlockFailureStack, BlockHistory, BlockExamples, BlockDefects})
	require.Empty(t, out.LogText)
	require.Nil(t, out.GroupMembers)
	require.Nil(t, out.LinkedRequirements)
	require.Equal(t, ev.ErrorMessage, out.ErrorMessage)
	require.Equal(t, []string{BlockFailureLog, BlockRequirements, BlockRelatedFailures}, out.NotChecked,
		"only blocks that had content are named")
	require.Equal(t, "log", ev.LogText, "the receiver is a copy")

	_, meta, err := BuildPrompt(out.PromptInput(DefaultPromptTemplate))
	require.NoError(t, err)
	require.Equal(t, "[context: not checked: failure.log, requirements, related_failures] ", meta.TruncationPrefix)
	_, meta, err = BuildPrompt(ev.PromptInput(DefaultPromptTemplate))
	require.NoError(t, err)
	require.Empty(t, meta.TruncationPrefix, "without NotChecked the prefix is what it was")
}
