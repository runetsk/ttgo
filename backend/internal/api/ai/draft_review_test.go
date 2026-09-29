package ai

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"ttgo/pkg/tracker/aigen"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

// seedDrafts stores a generation run with the given drafts (pending) and returns them.
func seedDrafts(t *testing.T, e *useEnv, contents ...models.DraftContent) []*models.AIGeneratedDraft {
	t.Helper()
	run, _, err := e.s.CreateGenerationRun(&models.AIGenerationRun{AdditionalInstructions: "checkout"})
	require.NoError(t, err)
	var rows []*models.AIGeneratedDraft
	for i, c := range contents {
		d := &models.AIGeneratedDraft{RunID: run.ID, Position: i}
		require.NoError(t, d.ApplyContent(c))
		d.OriginalJSON = "{}"
		rows = append(rows, d)
	}
	require.NoError(t, e.s.CreateGenerationDrafts(run.ID, nil, rows, 0))
	return rows
}

func TestDraftReview_AddsFindingsAndJudgesDuplicates(t *testing.T) {
	e := newUseEnv(t, true, answerAll(
		func(id string, _ typesafe.Question) string {
			if strings.HasPrefix(id, "clarity_0") {
				return "poor"
			}
			return "good"
		},
		func(string) float64 { return 0.92 }, // the two drafts test the same behaviour
	))
	rows := seedDrafts(t, e,
		models.DraftContent{Name: "Pay with a card", Steps: []models.GeneratedStep{{Action: "Do the payment", ExpectedResult: "It works"}}},
		models.DraftContent{Name: "Card payment succeeds", Steps: []models.GeneratedStep{{Action: "Pay 10 EUR with Visa 4111", ExpectedResult: "Order confirmed"}}},
	)
	// The rubric showed no duplicate (the names share few words); draft 1 is draft 0's only candidate.
	pos1 := 1
	for _, r := range rows {
		r.QualityJSON = "[]"
		r.DuplicatesJSON = "[]"
	}
	rows[0].DuplicatesJSON = mustJSON(t, []aigen.DuplicateCandidate{{Kind: aigen.DupKindBatch, DraftPosition: &pos1, Name: "Card payment succeeds", Similarity: 0.2}})

	require.Equal(t, "", e.h.runTypeSafeDraftReview(t.Context(), rows, ""))
	reqs := e.client.requests()
	require.Len(t, reqs, 1, "one request for the batch")
	require.Len(t, reqs[0].Questions, 7, "three ratings per draft and one duplicate question")

	stored, err := e.s.GetGenerationRunWithDrafts(rows[0].RunID)
	require.NoError(t, err)
	var dims []aigen.QualityDimension
	require.NoError(t, json.Unmarshal([]byte(stored.Drafts[0].QualityJSON), &dims))
	require.Len(t, dims, 1)
	require.Equal(t, aigen.ReviewDimensionKey, dims[0].Key)
	require.Equal(t, "ts_clarity", dims[0].Findings[0].Code)
	var dups []aigen.DuplicateCandidate
	require.NoError(t, json.Unmarshal([]byte(stored.Drafts[0].DuplicatesJSON), &dups))
	require.Len(t, dups, 1)
	require.InDelta(t, 0.92, dups[0].Similarity, 1e-12)
	require.Contains(t, dups[0].Reason, "same behaviour")

	var cost []models.AIAnalysisCostEvent
	require.NoError(t, e.s.DB().Where("kind = ?", models.AnalysisCostKindDraftReview).Find(&cost).Error)
	require.Len(t, cost, 1)
}

func TestDraftReview_OffOrFailingLeavesTheDraftsAlone(t *testing.T) {
	off := newUseEnv(t, false, nil)
	rows := seedDrafts(t, off, models.DraftContent{Name: "x", Steps: []models.GeneratedStep{{Action: "a"}}})
	require.Equal(t, "", off.h.runTypeSafeDraftReview(t.Context(), rows, ""))

	failing := newUseEnv(t, true, func(typesafe.Request) (*typesafe.Response, error) { return nil, errors.New("down") })
	rows = seedDrafts(t, failing, models.DraftContent{Name: "x", Steps: []models.GeneratedStep{{Action: "a"}}})
	before := rows[0].QualityJSON
	require.Contains(t, failing.h.runTypeSafeDraftReview(t.Context(), rows, ""), "draft review failed")
	require.Equal(t, before, rows[0].QualityJSON)
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}
