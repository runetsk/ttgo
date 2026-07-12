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
	ErrDraftNotPending = errors.New("draft is not in pending status")
	ErrDraftInvalid    = errors.New("draft has validation errors")
	ErrUnknownDrafts   = errors.New("one or more drafts do not belong to this run")
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

// SaveDraftEdit overwrites a pending draft's editable content, bumps its
// version, marks it edited, stores fresh validation findings, and appends an
// edited event — all in one transaction.
func (s *Store) SaveDraftEdit(draftID string, content models.DraftContent, validationJSON string, actorID *string) (*models.AIGeneratedDraft, error) {
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
		d.Edited = true
		d.Version++
		if err := tx.Save(d).Error; err != nil {
			return err
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
		d.Status = models.AIDraftStatusRejected
		if err := tx.Save(d).Error; err != nil {
			return err
		}
		out = d
		return createGenerationEventTx(tx, &models.AIGenerationEvent{
			RunID: d.RunID, DraftID: &d.ID, EventType: models.AIGenEventRejected,
			ActorID: actorID, Reason: reason, Note: note,
		})
	})
	return out, err
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
		if err := tx.First(&models.Requirement{}, "id = ?", run.RequirementID).Error; err != nil {
			return fmt.Errorf("linked requirement: %w", err)
		}
		contents := make([]models.DraftContent, len(drafts))
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
		}

		// ── Materialize ──
		subfolderCache := map[string]string{}
		now := time.Now()
		for i, d := range drafts {
			c := contents[i]
			targetFolderID := folderID
			if groupByCategory && c.Category != "" {
				if cached, ok := subfolderCache[c.Category]; ok {
					targetFolderID = cached
				} else {
					sub, created, err := s.findOrCreateSubfolderTx(tx, folderID, c.Category)
					if err != nil {
						return fmt.Errorf("subfolder %q: %w", c.Category, err)
					}
					if created {
						res.SubfoldersCreated++
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
				return fmt.Errorf("test case %q: %w", c.Name, err)
			}
			if err := tx.Create(&models.RequirementTestCaseLink{
				ID: uuid.New().String(), RequirementID: run.RequirementID,
				TestCaseID: tc.ID, CreatedAt: now,
			}).Error; err != nil {
				return fmt.Errorf("link for %q: %w", c.Name, err)
			}
			if err := tx.Model(&models.AIGeneratedDraft{}).Where("id = ?", d.ID).
				Updates(map[string]interface{}{
					"status":                models.AIDraftStatusAccepted,
					"accepted_test_case_id": tc.ID,
				}).Error; err != nil {
				return err
			}
			meta, _ := json.Marshal(map[string]string{"test_case_id": tc.ID})
			if err := createGenerationEventTx(tx, &models.AIGenerationEvent{
				RunID: runID, DraftID: &d.ID, EventType: models.AIGenEventAccepted,
				ActorID: actorID, MetadataJSON: string(meta),
			}); err != nil {
				return err
			}
			res.CreatedTestCaseIDs = append(res.CreatedTestCaseIDs, tc.ID)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}
