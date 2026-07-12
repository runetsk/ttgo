package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func TestAIGenerationLifecycleModelsMigrate(t *testing.T) {
	s := newTestStore(t)

	run := &models.AIGenerationRun{
		ID: "run-1", IdempotencyKey: "key-1", RequirementID: "req-1",
		Status: models.AIGenerationRunStatusPending, CoverageLevel: "thorough",
	}
	require.NoError(t, s.db.Create(run).Error)

	draft := &models.AIGeneratedDraft{
		ID: "draft-1", RunID: "run-1", Position: 0, Version: 1,
		Name: "N", Status: models.AIDraftStatusPending,
	}
	require.NoError(t, draft.ApplyContent(models.DraftContent{
		Name: "N", Category: "Functional", Description: "d",
		SourceRefs: []string{"AC-1"},
		Steps:      []models.GeneratedStep{{Action: "a", ExpectedResult: "e"}},
	}))
	require.NoError(t, s.db.Create(draft).Error)

	event := &models.AIGenerationEvent{
		ID: "ev-1", RunID: "run-1", EventType: models.AIGenEventGenerated, CreatedAt: time.Now(),
	}
	require.NoError(t, s.db.Create(event).Error)

	var back models.AIGeneratedDraft
	require.NoError(t, s.db.First(&back, "id = ?", "draft-1").Error)
	content, err := back.Content()
	require.NoError(t, err)
	assert.Equal(t, "Functional", content.Category)
	require.Len(t, content.Steps, 1)
	assert.Equal(t, []string{"AC-1"}, content.SourceRefs)

	resp, err := back.ToResponse()
	require.NoError(t, err)
	assert.Equal(t, "run-1", resp.RunID)
	assert.Equal(t, "a", resp.Steps[0].Action)
}

func TestAIGenerationRunIdempotencyKeyUnique(t *testing.T) {
	s := newTestStore(t)
	r1 := &models.AIGenerationRun{ID: "r1", IdempotencyKey: "same", RequirementID: "req", Status: "pending"}
	r2 := &models.AIGenerationRun{ID: "r2", IdempotencyKey: "same", RequirementID: "req", Status: "pending"}
	require.NoError(t, s.db.Create(r1).Error)
	require.Error(t, s.db.Create(r2).Error, "unique index on idempotency_key must reject duplicates")
}
