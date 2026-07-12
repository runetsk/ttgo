package store

import (
	"encoding/json"
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

func TestAIGeneratedDraftNilStepsRoundTripsAsEmpty(t *testing.T) {
	s := newTestStore(t)
	run := &models.AIGenerationRun{ID: "run-ns", IdempotencyKey: "ns", RequirementID: "req", Status: "pending"}
	require.NoError(t, s.db.Create(run).Error)

	d := &models.AIGeneratedDraft{ID: "draft-ns", RunID: "run-ns", Position: 0, Version: 1, Status: "pending"}
	require.NoError(t, d.ApplyContent(models.DraftContent{Name: "N"})) // Steps + SourceRefs nil
	assert.Equal(t, "[]", d.StepsJSON, "nil steps must marshal to [], not null")
	require.NoError(t, s.db.Create(d).Error)

	var back models.AIGeneratedDraft
	require.NoError(t, s.db.First(&back, "id = ?", "draft-ns").Error)
	content, err := back.Content()
	require.NoError(t, err)
	assert.NotNil(t, content.Steps, "steps must be non-nil empty after round-trip")
	assert.Empty(t, content.Steps)

	resp, err := back.ToResponse()
	require.NoError(t, err)
	raw, err := json.Marshal(resp)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"steps":[]`, "must serialize steps as [], not null")
}

func TestCreateGenerationRunIdempotent(t *testing.T) {
	s := newTestStore(t)

	r1, created1, err := s.CreateGenerationRun(&models.AIGenerationRun{
		IdempotencyKey: "k1", RequirementID: "req-1",
	})
	require.NoError(t, err)
	require.True(t, created1)
	assert.NotEmpty(t, r1.ID)
	assert.Equal(t, models.AIGenerationRunStatusPending, r1.Status)

	r2, created2, err := s.CreateGenerationRun(&models.AIGenerationRun{
		IdempotencyKey: "k1", RequirementID: "req-1",
	})
	require.NoError(t, err)
	require.False(t, created2, "replay must return the existing run")
	assert.Equal(t, r1.ID, r2.ID)

	byKey, err := s.GetGenerationRunByKey("k1")
	require.NoError(t, err)
	require.NotNil(t, byKey)
	assert.Equal(t, r1.ID, byKey.ID)

	missing, err := s.GetGenerationRunByKey("no-such-key")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestCreateGenerationRunDefaultsKeyToID(t *testing.T) {
	s := newTestStore(t)
	r, created, err := s.CreateGenerationRun(&models.AIGenerationRun{RequirementID: "req-1"})
	require.NoError(t, err)
	require.True(t, created)
	assert.Equal(t, r.ID, r.IdempotencyKey)
}

func TestGenerationRunLifecycleTransitions(t *testing.T) {
	s := newTestStore(t)
	r, _, err := s.CreateGenerationRun(&models.AIGenerationRun{RequirementID: "req-1"})
	require.NoError(t, err)

	require.NoError(t, s.MarkGenerationRunRunning(r.ID))
	got, err := s.GetGenerationRun(r.ID)
	require.NoError(t, err)
	assert.Equal(t, models.AIGenerationRunStatusRunning, got.Status)
	require.NotNil(t, got.StartedAt)

	got.Status = models.AIGenerationRunStatusCompleted
	now := time.Now()
	got.CompletedAt = &now
	got.TotalTokens = 42
	require.NoError(t, s.UpdateGenerationRun(got))
	back, err := s.GetGenerationRun(r.ID)
	require.NoError(t, err)
	assert.Equal(t, models.AIGenerationRunStatusCompleted, back.Status)
	assert.Equal(t, 42, back.TotalTokens)
}

func TestListGenerationRunsNewestFirst(t *testing.T) {
	s := newTestStore(t)
	a, _, err := s.CreateGenerationRun(&models.AIGenerationRun{RequirementID: "req-1"})
	require.NoError(t, err)
	require.NoError(t, s.db.Model(&models.AIGenerationRun{}).Where("id = ?", a.ID).
		Update("created_at", time.Now().Add(-time.Hour)).Error)
	b, _, err := s.CreateGenerationRun(&models.AIGenerationRun{RequirementID: "req-1"})
	require.NoError(t, err)
	_, _, err = s.CreateGenerationRun(&models.AIGenerationRun{RequirementID: "other-req"})
	require.NoError(t, err)

	runs, err := s.ListGenerationRuns("req-1", 50)
	require.NoError(t, err)
	require.Len(t, runs, 2)
	assert.Equal(t, b.ID, runs[0].ID)
	assert.Equal(t, a.ID, runs[1].ID)
}

func seedRunWithDraft(t *testing.T, s *Store) (*models.AIGenerationRun, *models.AIGeneratedDraft) {
	t.Helper()
	run, _, err := s.CreateGenerationRun(&models.AIGenerationRun{RequirementID: "req-1"})
	require.NoError(t, err)
	draft := &models.AIGeneratedDraft{RunID: run.ID, Position: 0}
	require.NoError(t, draft.ApplyContent(models.DraftContent{
		Name: "Original name", Category: "Functional", Description: "d",
		Steps: []models.GeneratedStep{{Action: "a", ExpectedResult: "e"}},
	}))
	draft.OriginalJSON = draft.StepsJSON // placeholder; real callers store full content
	require.NoError(t, s.CreateGenerationDrafts(run.ID, nil, []*models.AIGeneratedDraft{draft}, 0))
	return run, draft
}

func TestCreateGenerationDraftsEmitsEvents(t *testing.T) {
	s := newTestStore(t)
	run, draft := seedRunWithDraft(t, s)
	assert.NotEmpty(t, draft.ID)
	assert.Equal(t, models.AIDraftStatusPending, draft.Status)
	assert.Equal(t, 1, draft.Version)

	events, err := s.ListGenerationEvents(run.ID)
	require.NoError(t, err)
	types := []string{}
	for _, e := range events {
		types = append(types, e.EventType)
	}
	assert.Contains(t, types, models.AIGenEventGenerated)
	assert.Contains(t, types, models.AIGenEventValidated)
}

func TestSaveDraftEdit(t *testing.T) {
	s := newTestStore(t)
	run, draft := seedRunWithDraft(t, s)

	actor := "user-1"
	updated, err := s.SaveDraftEdit(draft.ID, models.DraftContent{
		Name: "Edited name", Category: "Negative", Description: "d2",
		Steps: []models.GeneratedStep{{Action: "a2", ExpectedResult: "e2"}},
	}, "[]", &actor)
	require.NoError(t, err)
	assert.Equal(t, "Edited name", updated.Name)
	assert.True(t, updated.Edited)
	assert.Equal(t, 2, updated.Version)

	events, err := s.ListGenerationEvents(run.ID)
	require.NoError(t, err)
	last := events[len(events)-1]
	assert.Equal(t, models.AIGenEventEdited, last.EventType)
	require.NotNil(t, last.DraftID)
	assert.Equal(t, draft.ID, *last.DraftID)
}

func TestSaveDraftEditRefusesNonPending(t *testing.T) {
	s := newTestStore(t)
	_, draft := seedRunWithDraft(t, s)
	require.NoError(t, s.db.Model(&models.AIGeneratedDraft{}).Where("id = ?", draft.ID).
		Update("status", models.AIDraftStatusAccepted).Error)

	_, err := s.SaveDraftEdit(draft.ID, models.DraftContent{Name: "x"}, "[]", nil)
	assert.ErrorIs(t, err, ErrDraftNotPending)
}

func TestRejectGenerationDraft(t *testing.T) {
	s := newTestStore(t)
	run, draft := seedRunWithDraft(t, s)

	rejected, err := s.RejectGenerationDraft(draft.ID, "too_vague", "steps lack data", nil)
	require.NoError(t, err)
	assert.Equal(t, models.AIDraftStatusRejected, rejected.Status)

	_, err = s.RejectGenerationDraft(draft.ID, "duplicate", "", nil)
	assert.ErrorIs(t, err, ErrDraftNotPending, "double reject must fail")

	events, err := s.ListGenerationEvents(run.ID)
	require.NoError(t, err)
	last := events[len(events)-1]
	assert.Equal(t, models.AIGenEventRejected, last.EventType)
	assert.Equal(t, "too_vague", last.Reason)
	assert.Equal(t, "steps lack data", last.Note)
}
