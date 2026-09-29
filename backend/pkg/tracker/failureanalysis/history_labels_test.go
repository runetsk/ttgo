package failureanalysis

import (
	"testing"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// Spec §3.1: the history block is "what a person labelled" — a label auto-apply wrote reads as
// unlabelled, in the per-row label and in the rollup, so the engines never learn from themselves.
func TestHistoryIgnoresLabelsSetByAutoApply(t *testing.T) {
	at := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	hist := []*models.RunResult{
		{ID: "h1", Status: models.StatusFail, ErrorMessage: "e1", DefectType: "product_bug", StartTime: at},
		{ID: "h2", Status: models.StatusFail, ErrorMessage: "e2", DefectType: "automation_bug",
			DefectTypeSource: models.DefectTypeSourceAI, StartTime: at.Add(time.Hour)},
		{ID: "h3", Status: models.StatusError, ErrorMessage: "e3", DefectType: "product_bug", StartTime: at.Add(2 * time.Hour)},
	}
	got := mapSimilarFailures(&mockSource{}, hist)
	require.Len(t, got, 3, "the AI-labelled failure is still history, only its label is dropped")
	require.Equal(t, "product_bug", got[0].DefectType)
	require.Equal(t, "", got[1].DefectType, "a label set by auto-apply is not a person's label")
	require.Equal(t, "e2", got[1].ErrorMessage)
	require.Equal(t, "product_bug", got[2].DefectType)

	require.Equal(t, "product_bug "+timesGlyph+"2", rollupDefectTypes(hist))
	require.Equal(t, "", rollupDefectTypes(hist[1:2]), "only AI labels: nothing to roll up")
}
