package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func TestGenerationAttemptsRoundTrip(t *testing.T) {
	s := newTestStore(t)
	run, draft := seedRunWithDraft(t, s)

	require.NoError(t, s.CreateGenerationAttempt(&models.AIGenerationAttempt{
		RunID: run.ID, Kind: models.AIGenAttemptGeneration, ModelName: "m",
		PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30, DurationMs: 5, Retries: 1,
	}))
	require.NoError(t, s.CreateGenerationAttempt(&models.AIGenerationAttempt{
		RunID: run.ID, DraftID: &draft.ID, Kind: models.AIGenAttemptRegenerate,
		ErrorCategory: "timeout",
	}))

	attempts, err := s.ListGenerationAttempts(run.ID)
	require.NoError(t, err)
	require.Len(t, attempts, 2)
	assert.Equal(t, models.AIGenAttemptGeneration, attempts[0].Kind)
	assert.NotEmpty(t, attempts[0].ID, "ID assigned on create")
	assert.Equal(t, draft.ID, *attempts[1].DraftID)
}
