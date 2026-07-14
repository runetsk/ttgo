package store

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/aigen"
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
	orig, err := json.Marshal(models.DraftContent{
		Name: "Original name", Category: "Functional", Description: "d",
		Steps: []models.GeneratedStep{{Action: "a", ExpectedResult: "e"}},
	})
	require.NoError(t, err)
	draft.OriginalJSON = string(orig)
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
	}, "[]", "", "", &actor)
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

	_, err := s.SaveDraftEdit(draft.ID, models.DraftContent{Name: "x"}, "[]", "", "", nil)
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

func TestRestoreGenerationDraft(t *testing.T) {
	s := newTestStore(t)
	_, draft := seedRunWithDraft(t, s)

	_, err := s.RejectGenerationDraft(draft.ID, "duplicate", "", nil)
	require.NoError(t, err)

	restored, err := s.RestoreGenerationDraft(draft.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, models.AIDraftStatusPending, restored.Status)

	events, err := s.ListGenerationEvents(draft.RunID)
	require.NoError(t, err)
	last := events[len(events)-1]
	assert.Equal(t, models.AIGenEventRestored, last.EventType)
	require.NotNil(t, last.DraftID)
	assert.Equal(t, draft.ID, *last.DraftID)

	// Restoring a pending draft is a state error.
	_, err = s.RestoreGenerationDraft(draft.ID, nil)
	assert.ErrorIs(t, err, ErrDraftNotRejected)
}

func TestDraftResponseCarriesOriginal(t *testing.T) {
	s := newTestStore(t)
	_, draft := seedRunWithDraft(t, s)
	resp, err := draft.ToResponse()
	require.NoError(t, err)
	require.NotNil(t, resp.Original)
	assert.Equal(t, "Original name", resp.Original.Name)
}

// seedAcceptFixture creates a real requirement + folder + run with n pending drafts.
func seedAcceptFixture(t *testing.T, s *Store, n int) (*models.Requirement, *models.Folder, *models.AIGenerationRun, []*models.AIGeneratedDraft) {
	t.Helper()
	req := &models.Requirement{Identifier: "REQ-ACC-" + uuid.New().String()[:8], Title: "Login"}
	require.NoError(t, s.CreateRequirement(req))
	folder, err := s.CreateFolder("AI Generated", nil)
	require.NoError(t, err)
	run, _, err := s.CreateGenerationRun(&models.AIGenerationRun{RequirementID: req.ID})
	require.NoError(t, err)

	drafts := make([]*models.AIGeneratedDraft, n)
	for i := range drafts {
		d := &models.AIGeneratedDraft{}
		require.NoError(t, d.ApplyContent(models.DraftContent{
			Name:        fmt.Sprintf("Draft %d", i),
			Category:    "Functional",
			Description: "desc",
			Steps: []models.GeneratedStep{
				{Action: "do", ExpectedResult: "done"},
				{Action: "check", ExpectedResult: "ok"},
			},
		}))
		drafts[i] = d
	}
	require.NoError(t, s.CreateGenerationDrafts(run.ID, nil, drafts, 0))
	return req, folder, run, drafts
}

func TestAcceptGenerationDrafts_Success(t *testing.T) {
	s := newTestStore(t)
	req, folder, run, drafts := seedAcceptFixture(t, s, 2)

	res, err := s.AcceptGenerationDrafts(run.ID, []string{drafts[0].ID, drafts[1].ID}, folder.ID, true, nil)
	require.NoError(t, err)
	require.Len(t, res.CreatedTestCaseIDs, 2)
	assert.Equal(t, 1, res.SubfoldersCreated, "both drafts share the Functional subfolder")
	assert.False(t, res.AlreadyAccepted)

	// Test cases exist with steps, in the category subfolder, linked to the requirement.
	for _, id := range res.CreatedTestCaseIDs {
		tc, err := s.GetTestCase(id)
		require.NoError(t, err)
		assert.Len(t, tc.Steps, 2)
		var sub models.Folder
		require.NoError(t, s.db.First(&sub, "id = ?", tc.FolderID).Error)
		assert.Equal(t, "Functional", sub.Name)
		var link models.RequirementTestCaseLink
		require.NoError(t, s.db.First(&link, "requirement_id = ? AND test_case_id = ?", req.ID, id).Error)
	}

	// Drafts flipped to accepted and reference their test case.
	for _, d := range drafts {
		var back models.AIGeneratedDraft
		require.NoError(t, s.db.First(&back, "id = ?", d.ID).Error)
		assert.Equal(t, models.AIDraftStatusAccepted, back.Status)
		require.NotNil(t, back.AcceptedTestCaseID)
	}

	// Accepted events recorded per draft.
	events, err := s.ListGenerationEvents(run.ID)
	require.NoError(t, err)
	accepted := 0
	for _, e := range events {
		if e.EventType == models.AIGenEventAccepted {
			accepted++
		}
	}
	assert.Equal(t, 2, accepted)
}

func TestAcceptGenerationDrafts_ReplayIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	_, folder, run, drafts := seedAcceptFixture(t, s, 1)
	ids := []string{drafts[0].ID}

	first, err := s.AcceptGenerationDrafts(run.ID, ids, folder.ID, true, nil)
	require.NoError(t, err)
	replay, err := s.AcceptGenerationDrafts(run.ID, ids, folder.ID, true, nil)
	require.NoError(t, err)
	assert.True(t, replay.AlreadyAccepted)
	assert.Equal(t, first.CreatedTestCaseIDs, replay.CreatedTestCaseIDs)

	var count int64
	require.NoError(t, s.db.Model(&models.TestCase{}).Count(&count).Error)
	assert.Equal(t, int64(1), count, "replay must not create duplicate test cases")
}

func TestAcceptGenerationDrafts_MixedStateFails(t *testing.T) {
	s := newTestStore(t)
	_, folder, run, drafts := seedAcceptFixture(t, s, 2)
	_, err := s.AcceptGenerationDrafts(run.ID, []string{drafts[0].ID}, folder.ID, true, nil)
	require.NoError(t, err)

	_, err = s.AcceptGenerationDrafts(run.ID, []string{drafts[0].ID, drafts[1].ID}, folder.ID, true, nil)
	assert.ErrorIs(t, err, ErrDraftNotPending)
}

func TestAcceptGenerationDrafts_InvalidDraftBlocksBatch(t *testing.T) {
	s := newTestStore(t)
	_, folder, run, drafts := seedAcceptFixture(t, s, 2)
	findings, _ := json.Marshal([]aigen.Finding{{
		Field: "steps", Code: "no_steps", Message: "a test case needs at least one step",
		Severity: aigen.SeverityError,
	}})
	require.NoError(t, s.db.Model(&models.AIGeneratedDraft{}).Where("id = ?", drafts[1].ID).
		Update("validation_json", string(findings)).Error)

	_, err := s.AcceptGenerationDrafts(run.ID, []string{drafts[0].ID, drafts[1].ID}, folder.ID, true, nil)
	assert.ErrorIs(t, err, ErrDraftInvalid)

	var count int64
	require.NoError(t, s.db.Model(&models.TestCase{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "validation happens before any write")
}

func TestAcceptGenerationDrafts_UnknownDraftFails(t *testing.T) {
	s := newTestStore(t)
	_, folder, run, drafts := seedAcceptFixture(t, s, 1)
	_, err := s.AcceptGenerationDrafts(run.ID, []string{drafts[0].ID, "not-a-draft"}, folder.ID, true, nil)
	assert.ErrorIs(t, err, ErrUnknownDrafts)
}

// countRows is a rollback-assertion helper.
func countRows(t *testing.T, s *Store, model interface{}) int64 {
	t.Helper()
	var n int64
	require.NoError(t, s.db.Model(model).Count(&n).Error)
	return n
}

func TestAcceptGenerationDrafts_RollsBackOnTestCaseInsertFailure(t *testing.T) {
	s := newTestStore(t)
	_, folder, run, drafts := seedAcceptFixture(t, s, 2)
	// Make the SECOND draft's insert blow up mid-transaction.
	require.NoError(t, s.db.Model(&models.AIGeneratedDraft{}).Where("id = ?", drafts[1].ID).
		Update("name", "BOOM").Error)
	require.NoError(t, s.db.Exec(`CREATE TRIGGER fail_tc BEFORE INSERT ON test_cases
		WHEN new.name = 'BOOM' BEGIN SELECT RAISE(ABORT, 'injected failure'); END`).Error)
	t.Cleanup(func() { s.db.Exec(`DROP TRIGGER IF EXISTS fail_tc`) })

	baseFolders := countRows(t, s, &models.Folder{})
	_, err := s.AcceptGenerationDrafts(run.ID, []string{drafts[0].ID, drafts[1].ID}, folder.ID, true, nil)
	require.Error(t, err)

	// EVERYTHING rolled back: no test cases, steps, links, subfolders, or status flips.
	assert.Equal(t, int64(0), countRows(t, s, &models.TestCase{}))
	assert.Equal(t, int64(0), countRows(t, s, &models.TestStep{}))
	assert.Equal(t, int64(0), countRows(t, s, &models.RequirementTestCaseLink{}))
	assert.Equal(t, baseFolders, countRows(t, s, &models.Folder{}), "injected failure must roll back the Functional subfolder")
	var back models.AIGeneratedDraft
	require.NoError(t, s.db.First(&back, "id = ?", drafts[0].ID).Error)
	assert.Equal(t, models.AIDraftStatusPending, back.Status)
	assert.Nil(t, back.AcceptedTestCaseID)
}

func TestAcceptGenerationDrafts_RollsBackOnLinkInsertFailure(t *testing.T) {
	s := newTestStore(t)
	_, folder, run, drafts := seedAcceptFixture(t, s, 1)
	require.NoError(t, s.db.Exec(`CREATE TRIGGER fail_link BEFORE INSERT ON requirement_test_case_links
		BEGIN SELECT RAISE(ABORT, 'injected link failure'); END`).Error)
	t.Cleanup(func() { s.db.Exec(`DROP TRIGGER IF EXISTS fail_link`) })

	baseFolders := countRows(t, s, &models.Folder{})
	_, err := s.AcceptGenerationDrafts(run.ID, []string{drafts[0].ID}, folder.ID, true, nil)
	require.Error(t, err)

	// Release gate: "Every accepted test is linked to its requirement or the transaction fails."
	assert.Equal(t, int64(0), countRows(t, s, &models.TestCase{}))
	assert.Equal(t, int64(0), countRows(t, s, &models.TestStep{}))
	assert.Equal(t, baseFolders, countRows(t, s, &models.Folder{}), "subfolder rolled back")
	var back models.AIGeneratedDraft
	require.NoError(t, s.db.First(&back, "id = ?", drafts[0].ID).Error)
	assert.Equal(t, models.AIDraftStatusPending, back.Status)
	assert.Nil(t, back.AcceptedTestCaseID)
}

func TestAcceptGenerationDrafts_UnparseableFindingsBlockBatch(t *testing.T) {
	s := newTestStore(t)
	_, folder, run, drafts := seedAcceptFixture(t, s, 2)
	// Corrupt one draft's stored findings so they can't be parsed.
	require.NoError(t, s.db.Model(&models.AIGeneratedDraft{}).Where("id = ?", drafts[1].ID).
		Update("validation_json", "{ this is not valid json").Error)

	_, err := s.AcceptGenerationDrafts(run.ID, []string{drafts[0].ID, drafts[1].ID}, folder.ID, true, nil)
	assert.ErrorIs(t, err, ErrDraftInvalid)

	// Fail-closed happens in the pre-write validation loop → nothing materialized.
	assert.Equal(t, int64(0), countRows(t, s, &models.TestCase{}))
}

func TestSaveDraftEditRefusesRejectedDraft(t *testing.T) {
	s := newTestStore(t)
	_, draft := seedRunWithDraft(t, s)

	// Reject it, then attempt to edit: the status-guarded update must refuse the
	// edit (ErrDraftNotPending) rather than resurrect a rejected draft to pending.
	_, err := s.RejectGenerationDraft(draft.ID, "duplicate", "", nil)
	require.NoError(t, err)

	_, err = s.SaveDraftEdit(draft.ID, models.DraftContent{
		Name:  "resurrected",
		Steps: []models.GeneratedStep{{Action: "a", ExpectedResult: "e"}},
	}, "[]", "", "", nil)
	assert.ErrorIs(t, err, ErrDraftNotPending)

	var back models.AIGeneratedDraft
	require.NoError(t, s.db.First(&back, "id = ?", draft.ID).Error)
	assert.Equal(t, models.AIDraftStatusRejected, back.Status, "draft must stay rejected")
	assert.NotEqual(t, "resurrected", back.Name, "edit content must not have been written")
}

func TestDraftQualityAndDuplicatesRoundTrip(t *testing.T) {
	s := newTestStore(t)
	run, draft := seedRunWithDraft(t, s)

	updated, err := s.SaveDraftEdit(draft.ID, models.DraftContent{
		Name: "Edited name", Category: "Functional", Description: "d",
		Steps: []models.GeneratedStep{{Action: "do the thing", ExpectedResult: "the thing is done"}},
	}, `[]`, `[{"key":"specificity","label":"Test-data specificity","findings":[]}]`, `[{"kind":"batch","name":"x","similarity":1,"reason":"r"}]`, nil)
	require.NoError(t, err)
	assert.Contains(t, updated.QualityJSON, `"specificity"`)
	assert.Contains(t, updated.DuplicatesJSON, `"batch"`)

	// Re-fetch from DB to verify persistence, not just in-memory assignment.
	var fresh models.AIGeneratedDraft
	require.NoError(t, s.db.First(&fresh, "id = ?", draft.ID).Error)
	assert.Contains(t, fresh.QualityJSON, `"specificity"`, "quality must persist to the DB, not just the returned struct")
	assert.Contains(t, fresh.DuplicatesJSON, `"batch"`, "duplicates must persist to the DB")

	resp, err := updated.ToResponse()
	require.NoError(t, err)
	assert.NotNil(t, resp.Quality)
	assert.NotNil(t, resp.Duplicates)

	// Empty payloads are omitted from the response.
	updated, err = s.SaveDraftEdit(draft.ID, models.DraftContent{
		Name: "Edited again", Category: "Functional", Description: "d",
		Steps: []models.GeneratedStep{{Action: "do", ExpectedResult: "done"}},
	}, `[]`, `[]`, `[]`, nil)
	require.NoError(t, err)
	resp, err = updated.ToResponse()
	require.NoError(t, err)
	assert.Nil(t, resp.Quality)
	assert.Nil(t, resp.Duplicates)

	// Re-fetch from DB to verify empty values persisted.
	require.NoError(t, s.db.First(&fresh, "id = ?", draft.ID).Error)
	assert.Equal(t, `[]`, fresh.QualityJSON)
	assert.Equal(t, `[]`, fresh.DuplicatesJSON)
	_ = run
}
