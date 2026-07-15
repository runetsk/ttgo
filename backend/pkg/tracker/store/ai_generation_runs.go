package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"ttgo/pkg/tracker/aigen"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Sentinel errors for the generation lifecycle (mapped to HTTP codes in the API layer).
var (
	ErrDraftNotPending       = errors.New("draft is not in pending status")
	ErrDraftInvalid          = errors.New("draft has validation errors")
	ErrUnknownDrafts         = errors.New("one or more drafts do not belong to this run")
	ErrDraftNotRejected      = errors.New("draft is not in rejected status")
	ErrAmbiguousDraftVersion = errors.New("multiple unchosen draft versions selected at the same position; choose one first")
)

// CreateGenerationRun inserts run, OR returns the existing run holding the same
// idempotency key (created==false). The unique index on idempotency_key is the
// real guard against concurrent duplicate inserts: the loser re-reads the winner.
func (s *Store) CreateGenerationRun(run *models.AIGenerationRun) (*models.AIGenerationRun, bool, error) {
	if run.ID == "" {
		run.ID = uuid.New().String()
	}
	if run.IdempotencyKey == "" {
		run.IdempotencyKey = run.ID
	}
	if run.Status == "" {
		run.Status = models.AIGenerationRunStatusPending
	}
	existingByKey := func() (*models.AIGenerationRun, error) {
		var r models.AIGenerationRun
		if err := s.db.First(&r, "idempotency_key = ?", run.IdempotencyKey).Error; err != nil {
			return nil, err
		}
		return &r, nil
	}
	if existing, err := existingByKey(); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	if err := s.db.Create(run).Error; err != nil {
		if existing, e2 := existingByKey(); e2 == nil {
			return existing, false, nil
		}
		return nil, false, err
	}
	return run, true, nil
}

// GetGenerationRunByKey returns the run holding an idempotency key, or (nil, nil).
func (s *Store) GetGenerationRunByKey(key string) (*models.AIGenerationRun, error) {
	var run models.AIGenerationRun
	err := s.db.First(&run, "idempotency_key = ?", key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// GetGenerationRun fetches a run by ID.
func (s *Store) GetGenerationRun(id string) (*models.AIGenerationRun, error) {
	var run models.AIGenerationRun
	if err := s.db.First(&run, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &run, nil
}

// GetGenerationRunWithDrafts fetches a run and populates Drafts ordered by position.
func (s *Store) GetGenerationRunWithDrafts(id string) (*models.AIGenerationRun, error) {
	run, err := s.GetGenerationRun(id)
	if err != nil {
		return nil, err
	}
	if err := s.db.Where("run_id = ?", id).Order("position asc").Find(&run.Drafts).Error; err != nil {
		return nil, err
	}
	return run, nil
}

// ListGenerationRuns returns a requirement's runs, newest first.
func (s *Store) ListGenerationRuns(requirementID string, limit int) ([]*models.AIGenerationRun, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var runs []*models.AIGenerationRun
	err := s.db.Where("requirement_id = ?", requirementID).
		Order("created_at DESC").Limit(limit).Find(&runs).Error
	return runs, err
}

// MarkGenerationRunRunning transitions the run to running and stamps StartedAt.
func (s *Store) MarkGenerationRunRunning(id string) error {
	now := time.Now()
	return s.db.Model(&models.AIGenerationRun{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     models.AIGenerationRunStatusRunning,
			"started_at": &now,
		}).Error
}

// UpdateGenerationRun persists all fields of run (single-writer synchronous flow).
func (s *Store) UpdateGenerationRun(run *models.AIGenerationRun) error {
	return s.db.Save(run).Error
}

// AddGenerationRunUsage atomically adds one call's token usage (and optional
// cost) to a run's cumulative totals. Uses SQL expressions so concurrent
// regenerations can't lose an update (read-modify-write would race), and adds
// this call's cost as a delta rather than recomputing from cumulative totals
// (which would reprice history at the current provider's prices).
func (s *Store) AddGenerationRunUsage(runID string, promptTokens, completionTokens, totalTokens int, costDelta *float64) error {
	updates := map[string]interface{}{
		"prompt_tokens":     gorm.Expr("prompt_tokens + ?", promptTokens),
		"completion_tokens": gorm.Expr("completion_tokens + ?", completionTokens),
		"total_tokens":      gorm.Expr("total_tokens + ?", totalTokens),
	}
	if costDelta != nil {
		updates["estimated_cost"] = gorm.Expr("COALESCE(estimated_cost, 0) + ?", *costDelta)
	}
	return s.db.Model(&models.AIGenerationRun{}).Where("id = ?", runID).Updates(updates).Error
}

// createGenerationEventTx appends one lifecycle event inside tx.
func createGenerationEventTx(tx *gorm.DB, ev *models.AIGenerationEvent) error {
	if ev.ID == "" {
		ev.ID = uuid.New().String()
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now()
	}
	return tx.Create(ev).Error
}

// CreateGenerationDrafts persists a run's drafts and appends the generated +
// validated events atomically. Draft IDs and pending status are assigned here.
func (s *Store) CreateGenerationDrafts(runID string, actorID *string, drafts []*models.AIGeneratedDraft, invalidCount int) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		for i, d := range drafts {
			if d.ID == "" {
				d.ID = uuid.New().String()
			}
			d.RunID = runID
			d.Position = i
			if d.Version == 0 {
				d.Version = 1
			}
			if d.Status == "" {
				d.Status = models.AIDraftStatusPending
			}
			if err := tx.Create(d).Error; err != nil {
				return err
			}
		}
		genMeta, _ := json.Marshal(map[string]int{"draft_count": len(drafts)})
		if err := createGenerationEventTx(tx, &models.AIGenerationEvent{
			RunID: runID, EventType: models.AIGenEventGenerated,
			ActorID: actorID, MetadataJSON: string(genMeta),
		}); err != nil {
			return err
		}
		valMeta, _ := json.Marshal(map[string]int{"invalid_drafts": invalidCount})
		return createGenerationEventTx(tx, &models.AIGenerationEvent{
			RunID: runID, EventType: models.AIGenEventValidated,
			ActorID: actorID, MetadataJSON: string(valMeta),
		})
	})
}

// getPendingDraftTx loads a draft and enforces pending status.
func getPendingDraftTx(tx *gorm.DB, draftID string) (*models.AIGeneratedDraft, error) {
	var d models.AIGeneratedDraft
	if err := tx.First(&d, "id = ?", draftID).Error; err != nil {
		return nil, err
	}
	if d.Status != models.AIDraftStatusPending {
		return nil, fmt.Errorf("%w: draft %s is %s", ErrDraftNotPending, d.ID, d.Status)
	}
	return &d, nil
}

// SaveDraftEdit updates a pending draft's editable content, refreshes its
// validation/quality/duplicate findings, bumps Version, and appends an
// `edited` event — all in one transaction. Non-pending drafts return
// ErrDraftNotPending (status-guarded update, see the concurrency note below).
func (s *Store) SaveDraftEdit(draftID string, content models.DraftContent, validationJSON, qualityJSON, duplicatesJSON string, actorID *string) (*models.AIGeneratedDraft, error) {
	var out *models.AIGeneratedDraft
	err := s.db.Transaction(func(tx *gorm.DB) error {
		d, err := getPendingDraftTx(tx, draftID)
		if err != nil {
			return err
		}
		if err := d.ApplyContent(content); err != nil {
			return err
		}
		d.ValidationJSON = validationJSON
		d.QualityJSON = qualityJSON
		d.DuplicatesJSON = duplicatesJSON
		d.Edited = true
		d.Version++
		// Optimistic guard: overwrite only while the row is still pending (status
		// left untouched). Between getPendingDraftTx's read and this write a
		// concurrent reject could have landed; the WHERE status=pending then
		// matches zero rows and the edit fails instead of resurrecting a rejected
		// draft to pending.
		res := tx.Model(d).
			Where("status = ?", models.AIDraftStatusPending).
			Select("Name", "Category", "Description", "StepsJSON", "SourceRefsJSON", "ValidationJSON", "QualityJSON", "DuplicatesJSON", "Edited", "Version").
			Updates(d)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("%w: draft %s", ErrDraftNotPending, d.ID)
		}
		out = d
		return createGenerationEventTx(tx, &models.AIGenerationEvent{
			RunID: d.RunID, DraftID: &d.ID, EventType: models.AIGenEventEdited, ActorID: actorID,
		})
	})
	return out, err
}

// RejectGenerationDraft transitions a pending draft to rejected and records the
// structured reason on the event trail.
func (s *Store) RejectGenerationDraft(draftID, reason, note string, actorID *string) (*models.AIGeneratedDraft, error) {
	var out *models.AIGeneratedDraft
	err := s.db.Transaction(func(tx *gorm.DB) error {
		d, err := getPendingDraftTx(tx, draftID)
		if err != nil {
			return err
		}
		// Optimistic guard: transition only while still pending, so a concurrent
		// edit/reject that already moved the draft is not silently clobbered — the
		// loser gets ErrDraftNotPending.
		res := tx.Model(d).
			Where("status = ?", models.AIDraftStatusPending).
			Update("status", models.AIDraftStatusRejected)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("%w: draft %s", ErrDraftNotPending, d.ID)
		}
		d.Status = models.AIDraftStatusRejected
		out = d
		return createGenerationEventTx(tx, &models.AIGenerationEvent{
			RunID: d.RunID, DraftID: &d.ID, EventType: models.AIGenEventRejected,
			ActorID: actorID, Reason: reason, Note: note,
		})
	})
	return out, err
}

// RestoreGenerationDraft returns a rejected draft to pending and appends a
// `restored` event. The status-guarded update makes concurrent restores or
// accepts lose cleanly (ErrDraftNotRejected).
func (s *Store) RestoreGenerationDraft(draftID string, actorID *string) (*models.AIGeneratedDraft, error) {
	var draft models.AIGeneratedDraft
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&draft, "id = ?", draftID).Error; err != nil {
			return err
		}
		if draft.Status != models.AIDraftStatusRejected {
			return fmt.Errorf("%w: draft %s is %s", ErrDraftNotRejected, draftID, draft.Status)
		}
		res := tx.Model(&models.AIGeneratedDraft{}).
			Where("id = ? AND status = ?", draftID, models.AIDraftStatusRejected).
			Update("status", models.AIDraftStatusPending)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("%w: draft %s changed concurrently", ErrDraftNotRejected, draftID)
		}
		draft.Status = models.AIDraftStatusPending
		return createGenerationEventTx(tx, &models.AIGenerationEvent{
			RunID: draft.RunID, DraftID: &draft.ID,
			EventType: models.AIGenEventRestored, ActorID: actorID,
		})
	})
	if err != nil {
		return nil, err
	}
	return &draft, nil
}

// ListGenerationEvents returns a run's events oldest-first. Ties on created_at
// (common: sub-millisecond clock resolution means same-transaction events can
// share a timestamp) break on SQLite's implicit rowid, which strictly reflects
// insertion order — unlike the event's random UUID id, which does not.
func (s *Store) ListGenerationEvents(runID string) ([]*models.AIGenerationEvent, error) {
	var events []*models.AIGenerationEvent
	err := s.db.Where("run_id = ?", runID).Order("created_at asc, rowid asc").Find(&events).Error
	return events, err
}

// AcceptGenerationResult reports what an acceptance materialized.
type AcceptGenerationResult struct {
	CreatedTestCaseIDs []string `json:"created_ids"`
	SubfoldersCreated  int      `json:"subfolders_created"`
	AlreadyAccepted    bool     `json:"already_accepted"`
}

// AcceptGenerationDrafts atomically materializes the selected drafts of a run
// as test cases: category subfolders, test cases with steps (and version
// snapshots), requirement links, draft status updates, and accepted events all
// commit or roll back together. Replaying a fully-accepted selection returns
// the stored test-case IDs without writing (idempotent acceptance).
func (s *Store) AcceptGenerationDrafts(runID string, draftIDs []string, folderID string, groupByCategory bool, actorID *string) (*AcceptGenerationResult, error) {
	res := &AcceptGenerationResult{CreatedTestCaseIDs: []string{}}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var run models.AIGenerationRun
		if err := tx.First(&run, "id = ?", runID).Error; err != nil {
			return err
		}

		// Dedup selection, load drafts, verify they all belong to the run.
		seen := map[string]bool{}
		ids := make([]string, 0, len(draftIDs))
		for _, id := range draftIDs {
			if id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		if len(ids) == 0 {
			return fmt.Errorf("%w: no drafts selected", ErrUnknownDrafts)
		}
		var drafts []*models.AIGeneratedDraft
		if err := tx.Where("run_id = ? AND id IN ?", runID, ids).Order("position asc").Find(&drafts).Error; err != nil {
			return err
		}
		if len(drafts) != len(ids) {
			return ErrUnknownDrafts
		}

		// Idempotent replay: the whole selection was already accepted.
		allAccepted := true
		for _, d := range drafts {
			if d.Status != models.AIDraftStatusAccepted {
				allAccepted = false
				break
			}
		}
		if allAccepted {
			for _, d := range drafts {
				if d.AcceptedTestCaseID != nil {
					res.CreatedTestCaseIDs = append(res.CreatedTestCaseIDs, *d.AcceptedTestCaseID)
				}
			}
			res.AlreadyAccepted = true
			return nil
		}

		// ── Validate everything BEFORE the first write ──
		if err := tx.First(&models.Folder{}, "id = ?", folderID).Error; err != nil {
			return fmt.Errorf("target folder: %w", err)
		}
		// A no-requirement run has an empty RequirementID — skip the requirement
		// existence check (and, in materializeDraftContentsTx, the link row).
		if run.RequirementID != "" {
			if err := tx.First(&models.Requirement{}, "id = ?", run.RequirementID).Error; err != nil {
				return fmt.Errorf("linked requirement: %w", err)
			}
		}
		contents := make([]models.DraftContent, len(drafts))
		positionCounts := map[int]int{}
		for i, d := range drafts {
			if d.Status != models.AIDraftStatusPending {
				return fmt.Errorf("%w: draft %q is %s", ErrDraftNotPending, d.Name, d.Status)
			}
			if d.ValidationJSON != "" {
				var findings []aigen.Finding
				if err := json.Unmarshal([]byte(d.ValidationJSON), &findings); err != nil {
					return fmt.Errorf("%w: draft %q has unreadable validation findings", ErrDraftInvalid, d.Name)
				}
				if aigen.HasErrors(findings) {
					return fmt.Errorf("%w: draft %q", ErrDraftInvalid, d.Name)
				}
			}
			c, err := d.Content()
			if err != nil {
				return fmt.Errorf("draft %q content: %w", d.Name, err)
			}
			contents[i] = c
			positionCounts[d.Position]++
		}
		// Every draft above is now confirmed pending. Two or more pending
		// drafts selected at the same position means an original draft and its
		// un-chosen regeneration alternative (or a chain of them) are both in
		// this request — accepting both would materialize duplicate test cases
		// for one logical position. ChooseDraftVersion is how a reviewer
		// resolves a family down to one pending draft before it can be
		// accepted. This only guards a single request: it does not (and is not
		// meant to) stop two SEPARATE accept calls from each taking one member
		// of the same family. drafts is ordered by position asc (the query
		// above), so this reports the first offending position deterministically
		// rather than via map iteration order.
		for _, d := range drafts {
			if positionCounts[d.Position] > 1 {
				return fmt.Errorf("%w: position %d", ErrAmbiguousDraftVersion, d.Position)
			}
		}

		// ── Materialize ──
		createdIDs, subfolders, merr := s.materializeDraftContentsTx(tx, run.RequirementID, folderID, groupByCategory, contents)
		if merr != nil {
			return merr
		}
		res.SubfoldersCreated = subfolders
		for i, d := range drafts {
			tcID := createdIDs[i]
			if err := tx.Model(&models.AIGeneratedDraft{}).Where("id = ?", d.ID).
				Updates(map[string]interface{}{
					"status":                models.AIDraftStatusAccepted,
					"accepted_test_case_id": tcID,
				}).Error; err != nil {
				return err
			}
			meta, _ := json.Marshal(map[string]string{"test_case_id": tcID})
			if err := createGenerationEventTx(tx, &models.AIGenerationEvent{
				RunID: runID, DraftID: &d.ID, EventType: models.AIGenEventAccepted,
				ActorID: actorID, MetadataJSON: string(meta),
			}); err != nil {
				return err
			}
			res.CreatedTestCaseIDs = append(res.CreatedTestCaseIDs, tcID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// materializeDraftContentsTx creates test cases + steps + requirement links
// for validated draft contents inside tx. Shared by lifecycle acceptance
// (AcceptGenerationDrafts) and the legacy accept adapter
// (AcceptLegacyGeneratedTests). createdIDs[i] is the test case materialized
// from contents[i] — order is preserved so callers can zip it back against
// their own per-draft bookkeeping.
func (s *Store) materializeDraftContentsTx(tx *gorm.DB, requirementID, folderID string, groupByCategory bool, contents []models.DraftContent) (createdIDs []string, subfolders int, err error) {
	subfolderCache := map[string]string{}
	now := time.Now()
	createdIDs = make([]string, 0, len(contents))
	for _, c := range contents {
		targetFolderID := folderID
		if groupByCategory && c.Category != "" {
			if cached, ok := subfolderCache[c.Category]; ok {
				targetFolderID = cached
			} else {
				sub, created, serr := s.findOrCreateSubfolderTx(tx, folderID, c.Category)
				if serr != nil {
					return nil, 0, fmt.Errorf("subfolder %q: %w", c.Category, serr)
				}
				if created {
					subfolders++
				}
				subfolderCache[c.Category] = sub.ID
				targetFolderID = sub.ID
			}
		}

		steps := make([]*models.TestStep, len(c.Steps))
		for j, st := range c.Steps {
			steps[j] = &models.TestStep{Action: st.Action, ExpectedResult: st.ExpectedResult, OrderIndex: j}
		}
		tc := &models.TestCase{
			FolderID:    targetFolderID,
			Name:        c.Name,
			Description: c.Description,
			Steps:       steps,
		}
		if err := s.createTestCaseTx(tx, tc); err != nil {
			return nil, 0, fmt.Errorf("test case %q: %w", c.Name, err)
		}
		if requirementID != "" {
			if err := tx.Create(&models.RequirementTestCaseLink{
				ID: uuid.New().String(), RequirementID: requirementID,
				TestCaseID: tc.ID, CreatedAt: now,
			}).Error; err != nil {
				return nil, 0, fmt.Errorf("link for %q: %w", c.Name, err)
			}
		}
		createdIDs = append(createdIDs, tc.ID)
	}
	return createdIDs, subfolders, nil
}

// AcceptLegacyGeneratedTests transactionally materializes transient legacy
// drafts (no lifecycle records). Fixes the historic partial-batch bug for
// legacy API clients: any failure rolls back the whole batch.
func (s *Store) AcceptLegacyGeneratedTests(requirementID, folderID string, tests []models.GeneratedTestCase, groupByCategory bool) (*AcceptGenerationResult, error) {
	res := &AcceptGenerationResult{CreatedTestCaseIDs: []string{}}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&models.Folder{}, "id = ?", folderID).Error; err != nil {
			return fmt.Errorf("target folder: %w", err)
		}
		if err := tx.First(&models.Requirement{}, "id = ?", requirementID).Error; err != nil {
			return fmt.Errorf("linked requirement: %w", err)
		}
		contents := make([]models.DraftContent, len(tests))
		for i, t := range tests {
			contents[i] = models.DraftContent{
				Name: t.Name, Category: t.Category, Description: t.Description,
				SourceRefs: t.SourceRefs, Steps: t.Steps,
			}
		}
		ids, subfolders, err := s.materializeDraftContentsTx(tx, requirementID, folderID, groupByCategory, contents)
		if err != nil {
			return err
		}
		res.CreatedTestCaseIDs = ids
		res.SubfoldersCreated = subfolders
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// GetDraftsByIDs loads drafts by ID (legacy temp_id -> lifecycle matching).
func (s *Store) GetDraftsByIDs(ids []string) ([]*models.AIGeneratedDraft, error) {
	var drafts []*models.AIGeneratedDraft
	err := s.db.Where("id IN ?", ids).Find(&drafts).Error
	return drafts, err
}

// RegenMeta is the safe metadata recorded on a `regenerated` event.
// AlternativeID is filled by the store. No prompt text belongs here.
type RegenMeta struct {
	Instruction      string `json:"instruction,omitempty"`
	Action           string `json:"action,omitempty"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	DurationMs       int64  `json:"duration_ms"`
	AlternativeID    string `json:"alternative_id"`
}

// CreateDraftAlternative persists a regenerated version of a pending draft as
// a NEW pending row in the same position family (Version+1, ParentDraftID set)
// and appends a `regenerated` event on the original. The original is untouched
// — reviewers choose between them explicitly (ChooseDraftVersion).
func (s *Store) CreateDraftAlternative(originalDraftID string, content models.DraftContent, validationJSON, qualityJSON, duplicatesJSON string, meta RegenMeta, actorID *string) (*models.AIGeneratedDraft, error) {
	var alt *models.AIGeneratedDraft
	err := s.db.Transaction(func(tx *gorm.DB) error {
		original, err := getPendingDraftTx(tx, originalDraftID)
		if err != nil {
			return err
		}
		originalSnapshot, err := json.Marshal(content)
		if err != nil {
			return err
		}
		alt = &models.AIGeneratedDraft{
			ID:             uuid.New().String(),
			RunID:          original.RunID,
			Position:       original.Position,
			Version:        original.Version + 1,
			ParentDraftID:  &original.ID,
			Status:         models.AIDraftStatusPending,
			ValidationJSON: validationJSON,
			QualityJSON:    qualityJSON,
			DuplicatesJSON: duplicatesJSON,
			OriginalJSON:   string(originalSnapshot),
		}
		if err := alt.ApplyContent(content); err != nil {
			return err
		}
		if err := tx.Create(alt).Error; err != nil {
			return err
		}
		if len(meta.Instruction) > 500 {
			meta.Instruction = meta.Instruction[:500]
		}
		meta.AlternativeID = alt.ID
		metaJSON, err := json.Marshal(meta)
		if err != nil {
			return err
		}
		return createGenerationEventTx(tx, &models.AIGenerationEvent{
			RunID: original.RunID, DraftID: &original.ID,
			EventType: models.AIGenEventRegenerated, ActorID: actorID,
			MetadataJSON: string(metaJSON),
		})
	})
	if err != nil {
		return nil, err
	}
	return alt, nil
}

// ChooseDraftVersion keeps one pending draft of a position family and marks
// every other pending draft at the same position superseded. Chain-proof:
// alternatives of alternatives share the position too.
func (s *Store) ChooseDraftVersion(draftID string, actorID *string) (*models.AIGeneratedDraft, []string, error) {
	var chosen models.AIGeneratedDraft
	var supersededIDs []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&chosen, "id = ?", draftID).Error; err != nil {
			return err
		}
		if chosen.Status != models.AIDraftStatusPending {
			return fmt.Errorf("%w: draft %s is %s", ErrDraftNotPending, draftID, chosen.Status)
		}
		var family []*models.AIGeneratedDraft
		if err := tx.Where("run_id = ? AND position = ? AND id <> ? AND status = ?",
			chosen.RunID, chosen.Position, chosen.ID, models.AIDraftStatusPending).
			Find(&family).Error; err != nil {
			return err
		}
		metaJSON := fmt.Sprintf(`{"chosen_id":%q}`, chosen.ID)
		for _, d := range family {
			res := tx.Model(&models.AIGeneratedDraft{}).
				Where("id = ? AND status = ?", d.ID, models.AIDraftStatusPending).
				Update("status", models.AIDraftStatusSuperseded)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				continue // lost a race to accept/reject — leave it be
			}
			supersededIDs = append(supersededIDs, d.ID)
			if err := createGenerationEventTx(tx, &models.AIGenerationEvent{
				RunID: chosen.RunID, DraftID: &d.ID,
				EventType: models.AIGenEventSuperseded, ActorID: actorID,
				MetadataJSON: metaJSON,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &chosen, supersededIDs, nil
}

// UpdateDraftQuality replaces a pending draft's quality findings (critic pass).
func (s *Store) UpdateDraftQuality(draftID, qualityJSON string) error {
	res := s.db.Model(&models.AIGeneratedDraft{}).
		Where("id = ? AND status = ?", draftID, models.AIDraftStatusPending).
		Update("quality_json", qualityJSON)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrDraftNotPending
	}
	return nil
}

// AppendGenerationEvent appends one lifecycle event outside a larger
// transaction (critic pass, cancellation stamps).
func (s *Store) AppendGenerationEvent(ev *models.AIGenerationEvent) error {
	return createGenerationEventTx(s.db, ev)
}
