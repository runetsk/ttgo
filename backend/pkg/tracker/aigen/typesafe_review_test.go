package aigen

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReviewFindings(t *testing.T) {
	got := ReviewFindings(map[string]Rating{
		"clarity":    {Value: "poor", Confidence: 0.82},
		"observable": {Value: "weak", Confidence: 0.61},
		"specific":   {Value: "weak", Confidence: 0.55}, // below the confidence floor
	})
	require.Len(t, got, 2)
	require.Equal(t, "ts_clarity", got[0].Code)
	require.Equal(t, "TypeSafe.ai rates the actions for clarity poor (82%)", got[0].Message)
	require.Equal(t, SeverityWarning, got[0].Severity)
	require.Equal(t, "ts_observable", got[1].Code)
	require.Empty(t, ReviewFindings(map[string]Rating{"clarity": {Value: "good", Confidence: 0.99}}))
	require.Empty(t, ReviewFindings(nil))
}

func TestWithReviewDimension_ReplacesOnlyItsOwnDimension(t *testing.T) {
	base := `[{"key":"action_clarity","label":"Action clarity","findings":[{"field":"steps[0].action","code":"vague_action","message":"m","severity":"warning"}]}]`
	withTS := WithReviewDimension(base, []Finding{{Field: "draft", Code: "ts_clarity", Message: "x", Severity: SeverityWarning}})
	var dims []QualityDimension
	require.NoError(t, json.Unmarshal([]byte(withTS), &dims))
	require.Len(t, dims, 2)
	require.Equal(t, ReviewDimensionKey, dims[1].Key)
	require.Equal(t, "TypeSafe.ai review", dims[1].Label)

	again := WithReviewDimension(withTS, nil)
	require.NoError(t, json.Unmarshal([]byte(again), &dims))
	require.Len(t, dims, 1, "a clean review drops the dimension")
	require.Equal(t, "action_clarity", dims[0].Key)
}

func pos(i int) *int { return &i }

func TestAskCandidatesAndMergeVerdicts(t *testing.T) {
	shown := []DuplicateCandidate{
		{Kind: DupKindExisting, TestCaseID: "tc-1", Name: "Login ok", Similarity: 0.7},
		{Kind: DupKindBatch, DraftPosition: pos(2), Name: "Login works", Similarity: 0.6},
	}
	wider := []DuplicateCandidate{
		{Kind: DupKindExisting, TestCaseID: "tc-1", Name: "Login ok", Similarity: 0.7}, // repeat
		{Kind: DupKindExisting, TestCaseID: "tc-9", Name: "Sign in with a password", Similarity: 0.35},
		{Kind: DupKindExisting, TestCaseID: "tc-8", Name: "Log out", Similarity: 0.31},
	}
	asked := AskCandidates(shown, wider)
	require.Len(t, asked, 3)
	require.Equal(t, []string{"tc:tc-1", "draft:2", "tc:tc-9"}, []string{DupKey(asked[0]), DupKey(asked[1]), DupKey(asked[2])})

	merged := MergeDuplicateVerdicts(shown, asked, map[string]float64{
		"tc:tc-1": 0.1,  // name-alike but a different behaviour: dropped
		"draft:2": 0.5,  // unsure: the name verdict stands
		"tc:tc-9": 0.93, // other words, same behaviour: added
	})
	require.Len(t, merged, 2)
	require.Equal(t, "tc-9", merged[0].TestCaseID)
	require.InDelta(t, 0.93, merged[0].Similarity, 1e-12)
	require.Equal(t, "TypeSafe.ai: same behaviour (93%)", merged[0].Reason)
	require.Equal(t, 2, *merged[1].DraftPosition)

	require.Equal(t, shown, MergeDuplicateVerdicts(shown, nil, nil), "no answers: nothing changes")
}
