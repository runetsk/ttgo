package failureanalysis

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// history_available (spec B5 #25) records whether the evidence carried this test's recent
// failures or the rollup of their human labels, on both the generative and the TypeSafe path.
func TestAnalyze_RecordsWhetherHistoryWasAvailable(t *testing.T) {
	verdictReply := `{"verdict":"product_bug","confidence":"high","summary":"s","next_action":"n","rationale":"r"}`
	out, err := Analyze(context.Background(), AnalyzeDeps{Narrative: &stubProvider{responses: []string{verdictReply}}, NarrativeModel: "gpt-test"}, baseContext())
	require.NoError(t, err)
	require.False(t, out.HistoryAvailable, "no similar failures and no rollup")

	in := baseContext()
	in.SimilarFailures = []SimilarFailure{{Status: "FAIL", ErrorMessage: "boom"}}
	out, err = Analyze(context.Background(), AnalyzeDeps{Narrative: &stubProvider{responses: []string{verdictReply}}, NarrativeModel: "gpt-test"}, in)
	require.NoError(t, err)
	require.True(t, out.HistoryAvailable)

	in = baseContext()
	in.SimilarFailuresRollup = "product_bug " + timesGlyph + "1"
	narrative := `{"summary":"S","next_action":"N","rationale":"R"}`
	out, err = Analyze(context.Background(), AnalyzeDeps{Narrative: &stubProvider{responses: []string{narrative}}, Decider: fixedDecider{d: flakyDecision()}}, in)
	require.NoError(t, err)
	require.True(t, out.HistoryAvailable, "the TypeSafe path records it too")

	row := AnalysisRowFrom(out, "rr1")
	require.True(t, row.HistoryAvailable)
	raw, err := json.Marshal(row)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"history_available":true`)
}
