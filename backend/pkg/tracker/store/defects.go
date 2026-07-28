package store

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"ttgo/pkg/tracker/models"
)

// DismissReverification clears the reverification flag on a test case (kept).
func (s *Store) DismissReverification(testCaseID string) error {
	if err := s.db.Model(&models.TestCase{}).Where("id = ?", testCaseID).
		Update("reverification_flagged", false).Error; err != nil {
		return err
	}
	return s.CreateAuditLog(&models.AuditLog{
		ID:        uuid.New().String(),
		Action:    fmt.Sprintf("defect:reverification_dismissed:%s", testCaseID),
		Timestamp: time.Now(),
	})
}

// CountDefectLinksByRunResults returns open/closed defect counts per run-result ID.
func (s *Store) CountDefectLinksByRunResults(runResultIDs []string) (open map[string]int, closed map[string]int, err error) {
	type row struct {
		RunResultID string
		Status      string
		N           int
	}
	var rows []row
	err = s.db.Raw(`
		SELECT dl.run_result_id, d.status, COUNT(DISTINCT d.id) as n
		FROM defect_links dl JOIN defects d ON d.id = dl.defect_id
		WHERE dl.run_result_id IN ? GROUP BY dl.run_result_id, d.status`, runResultIDs).Scan(&rows).Error
	open, closed = map[string]int{}, map[string]int{}
	for _, r := range rows {
		if r.Status == "closed" {
			closed[r.RunResultID] += r.N
		} else {
			open[r.RunResultID] += r.N
		}
	}
	return open, closed, err
}

// ListDefectsByRun returns all defects linked (via run results) to the given run.
func (s *Store) ListDefectsByRun(runID string) ([]models.RunDefectRow, error) {
	var rows []models.RunDefectRow
	err := s.db.Raw(`
		SELECT d.*, rr.test_case_id, rr.test_name_snapshot, rr.status AS result_status
		FROM defect_links dl JOIN run_results rr ON rr.id = dl.run_result_id JOIN defects d ON d.id = dl.defect_id
		WHERE rr.test_run_id = ? ORDER BY rr.test_name_snapshot ASC, d.created_at DESC`, runID).Scan(&rows).Error
	return rows, err
}

func (s *Store) CreateDefect(d *models.Defect) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	d.Title = strings.TrimSpace(d.Title)
	if d.Status == "" {
		d.Status = "open"
	}
	if d.Severity == "" {
		d.Severity = "minor"
	}
	if d.AssigneeID != nil && *d.AssigneeID == "" {
		d.AssigneeID = nil // on create "" and nil both mean unassigned -> store NULL
	}
	now := time.Now()
	d.CreatedAt = now
	d.UpdatedAt = now
	if err := s.db.Create(d).Error; err != nil {
		return err
	}
	// Resolved here rather than in the handlers so BOTH create paths (POST /defects and
	// the runs package's create-and-link) hand back the same shape ListDefects and
	// GetDefect do. Without it a freshly created defect carries assignee_id with an empty
	// assignee_name, which the register renders as "Unknown user" — the label reserved
	// for an owner whose account is actually gone.
	s.populateDefectAssigneeNames([]*models.Defect{d})
	return nil
}

func (s *Store) GetDefect(id string) (*models.Defect, error) {
	var d models.Defect
	err := s.db.First(&d, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.populateDefectAssigneeNames([]*models.Defect{&d})
	return &d, nil
}

// populateDefectAssigneeNames fills AssigneeName on defects that carry an AssigneeID via a
// single batch users lookup, mirroring populateAssigneeNames for runs (runs.go). DisplayName
// falls back to Email, and names resolve even for users who are no longer active.
//
// Deliberately a second query rather than a LEFT JOIN in the row scan: AssigneeName is
// gorm:"-", so a joined column would be silently discarded, and joining users also makes
// ListDefects' ORDER BY created_at ambiguous.
func (s *Store) populateDefectAssigneeNames(defects []*models.Defect) {
	seen := map[string]bool{}
	var ids []string
	for _, d := range defects {
		if d.AssigneeID != nil && *d.AssigneeID != "" && !seen[*d.AssigneeID] {
			seen[*d.AssigneeID] = true
			ids = append(ids, *d.AssigneeID)
		}
	}
	if len(ids) == 0 {
		return
	}
	type urow struct {
		ID          string
		DisplayName string
		Email       string
	}
	var rows []urow
	if err := s.db.Model(&models.User{}).Select("id, display_name, email").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		// Best-effort by design — a name is decoration, and failing a whole defect list
		// over it would be worse. But it must not be silent: every row would come back
		// with a blank assignee_name, which the register displays as "Unknown user".
		slog.Warn("defects: could not resolve assignee names", "err", err, "ids", len(ids))
		return
	}
	name := make(map[string]string, len(rows))
	for _, u := range rows {
		if u.DisplayName != "" {
			name[u.ID] = u.DisplayName
		} else {
			name[u.ID] = u.Email
		}
	}
	for _, d := range defects {
		if d.AssigneeID != nil {
			d.AssigneeName = name[*d.AssigneeID]
		}
	}
}

func (s *Store) ListDefects(status, severity, q string) ([]models.Defect, error) {
	tx := s.db.Model(&models.Defect{})
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	if severity != "" {
		tx = tx.Where("severity = ?", severity)
	}
	if q = strings.TrimSpace(q); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		tx = tx.Where("lower(title) LIKE ? OR lower(external_key) LIKE ?", like, like)
	}
	var defects []models.Defect
	if err := tx.Order("created_at DESC").Find(&defects).Error; err != nil {
		return nil, err
	}
	if err := s.enrichDefects(defects); err != nil {
		return nil, err
	}
	return defects, nil
}

// enrichDefects fills the computed, non-persisted fields on a defect slice: LinkedTestCount from
// one grouped defect_links query and AssigneeName from one batch users lookup. Shared by
// ListDefects and BulkUpdateDefects so both hand back identically shaped rows — a caller that
// patches a list in place from a bulk response must not have to re-merge either field by hand.
func (s *Store) enrichDefects(defects []models.Defect) error {
	if len(defects) == 0 {
		return nil
	}
	ids := make([]string, len(defects))
	for i := range defects {
		ids[i] = defects[i].ID
	}
	type cnt struct {
		DefectID string
		N        int
	}
	var counts []cnt
	if err := s.db.Model(&models.DefectLink{}).
		Select("defect_id, COUNT(DISTINCT test_case_id) as n").
		Where("defect_id IN ? AND test_case_id IS NOT NULL", ids).
		Group("defect_id").Scan(&counts).Error; err != nil {
		return err
	}
	byID := make(map[string]int, len(counts))
	for _, c := range counts {
		byID[c.DefectID] = c.N
	}
	ptrs := make([]*models.Defect, len(defects))
	for i := range defects {
		defects[i].LinkedTestCount = byID[defects[i].ID]
		ptrs[i] = &defects[i]
	}
	s.populateDefectAssigneeNames(ptrs)
	return nil
}

func (s *Store) UpdateDefect(id string, req models.UpdateDefectRequest) (*models.Defect, error) {
	updates := map[string]interface{}{"updated_at": time.Now()}
	if req.Title != nil {
		updates["title"] = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Severity != nil {
		updates["severity"] = *req.Severity
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.AssigneeID != nil {
		// "" clears the assignee; an absent (nil) field leaves it unchanged.
		if *req.AssigneeID == "" {
			updates["assignee_id"] = nil
		} else {
			updates["assignee_id"] = *req.AssigneeID
		}
	}
	if req.ExternalProvider != nil {
		updates["external_provider"] = *req.ExternalProvider
	}
	if req.ExternalKey != nil {
		updates["external_key"] = *req.ExternalKey
	}
	if req.ExternalURL != nil {
		updates["external_url"] = *req.ExternalURL
	}
	if err := s.db.Model(&models.Defect{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	d, err := s.GetDefect(id)
	if err != nil {
		return nil, err
	}
	if d != nil && req.Status != nil {
		if err := recomputeReverification(s.db, affectedTestCaseIDs(s.db, id)); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// BulkUpdateDefects applies the non-nil fields of req to every defect in ids with one grouped
// UPDATE inside a transaction, and returns the resulting rows enriched exactly like ListDefects'.
//
// Unknown ids are tolerated rather than rejected: the UPDATE simply matches fewer rows, and the
// returned slice reports the defects that really exist. An empty id list, or a request that sets
// no field at all, is a no-op — notably it does NOT bump updated_at, since that column is the
// staleness signal and a write with nothing to write would silently reset it.
//
// A status change recomputes reverification over the union of affected test cases, mirroring
// UpdateDefect (:181). "Mark verified & close" is the primary path to closed, so skipping it here
// would let bulk and single update diverge on reverification_flagged without any visible error.
func (s *Store) BulkUpdateDefects(ids []string, req models.BulkUpdateDefectsRequest) ([]models.Defect, error) {
	if len(ids) == 0 {
		return []models.Defect{}, nil
	}
	updates := map[string]interface{}{}
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Severity != nil {
		updates["severity"] = *req.Severity
	}
	if req.AssigneeID != nil {
		// "" clears the assignee across the selection; an absent (nil) field leaves it unchanged.
		if *req.AssigneeID == "" {
			updates["assignee_id"] = nil
		} else {
			updates["assignee_id"] = *req.AssigneeID
		}
	}
	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&models.Defect{}).Where("id IN ?", ids).Updates(updates).Error; err != nil {
				return err
			}
			if req.Status == nil {
				return nil
			}
			return recomputeReverification(tx, affectedTestCaseIDs(tx, ids...))
		})
		if err != nil {
			return nil, err
		}
	}

	var updated []models.Defect
	if err := s.db.Where("id IN ?", ids).Order("created_at DESC").Find(&updated).Error; err != nil {
		return nil, err
	}
	if err := s.enrichDefects(updated); err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Store) DeleteDefect(id string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		tcIDs := affectedTestCaseIDs(tx, id)
		if err := tx.Where("defect_id = ?", id).Delete(&models.DefectLink{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.Defect{}, "id = ?", id).Error; err != nil {
			return err
		}
		return recomputeReverification(tx, tcIDs)
	})
}

func (s *Store) LinkDefectToResult(defectID, runResultID, testCaseID string) (*models.DefectLink, error) {
	link := &models.DefectLink{ID: uuid.New().String(), DefectID: defectID, RunResultID: &runResultID, CreatedAt: time.Now()}
	if testCaseID != "" {
		link.TestCaseID = &testCaseID
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(link).Error; err != nil {
			if isUniqueConstraintError(err) {
				return models.ErrDuplicateDefectLink
			}
			return err
		}
		if testCaseID != "" {
			return recomputeReverification(tx, []string{testCaseID})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return link, nil
}

func (s *Store) LinkDefectToTestCase(defectID, testCaseID string) (*models.DefectLink, error) {
	link := &models.DefectLink{ID: uuid.New().String(), DefectID: defectID, TestCaseID: &testCaseID, CreatedAt: time.Now()}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(link).Error; err != nil {
			if isUniqueConstraintError(err) {
				return models.ErrDuplicateDefectLink
			}
			return err
		}
		return recomputeReverification(tx, []string{testCaseID})
	})
	if err != nil {
		return nil, err
	}
	return link, nil
}

func (s *Store) UnlinkDefectFromResult(defectID, runResultID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var link models.DefectLink
		err := tx.Where("defect_id = ? AND run_result_id = ?", defectID, runResultID).First(&link).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("defect link not found")
		}
		if err != nil {
			return err
		}
		if err := tx.Delete(&link).Error; err != nil {
			return err
		}
		if link.TestCaseID != nil {
			return recomputeReverification(tx, []string{*link.TestCaseID})
		}
		return nil
	})
}

func (s *Store) UnlinkDefectFromTestCase(defectID, testCaseID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("defect_id = ? AND test_case_id = ? AND run_result_id IS NULL", defectID, testCaseID).
			Delete(&models.DefectLink{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return fmt.Errorf("defect link not found")
		}
		return recomputeReverification(tx, []string{testCaseID})
	})
}

// AffectedTestCase is a distinct test case linked to a defect, together with the run in which that
// test case last failed.
//
// LastRunID / LastRunName / LastResultStatus describe the MOST RECENT FAILING RUN RESULT FOR THE
// TEST CASE — deliberately not "the run result the defect was linked from". Two reasons: a
// test-case-scoped link carries no run_result_id at all (models.DefectLink), so a link-scoped
// answer would be empty for every defect filed against a test case rather than a result; and the
// question the triage queue asks is "where do I go to see this failing / retest it?", which is
// about the test's latest failure, not about whichever historical result someone happened to file
// the defect against. "Failing" is models.FailureStatuses (FAIL and ERROR).
//
// All three are empty strings when the test case has no failing result — such test cases are still
// returned, they simply carry no run.
type AffectedTestCase struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	LastRunID        string `json:"last_run_id"`
	LastRunName      string `json:"last_run_name"`
	LastResultStatus string `json:"last_result_status"`
}

// ListAffectedTestCases returns the distinct test cases a defect affects, ordered by name, each
// carrying the run it last failed in (see AffectedTestCase). Both link kinds (result-scoped and
// test-case-scoped) carry test_case_id, so this matches the LinkedTestCount basis. Links whose
// test case has been deleted are excluded.
func (s *Store) ListAffectedTestCases(defectID string) ([]AffectedTestCase, error) {
	var out []AffectedTestCase
	// The link set is collapsed to one row per test case by `tc.id IN (…)` rather than by
	// SELECT DISTINCT: a test case linked through several run results used to be deduped by the
	// DISTINCT, but per-run columns differ between those rows, so DISTINCT would stop collapsing
	// them and the defect would list the same test twice.
	//
	// "Last failed" is ordered by when the result RAN (start_time), falling back to updated_at for
	// results recorded without timing — the same rule, and the same '0002-01-01' sentinel, as
	// ListRecentResultsForTestCase (runs.go) — with attempt_number breaking ties between retries
	// recorded inside one run at the same instant.
	err := s.db.Raw(`
		SELECT t.id, t.name,
		       COALESCE(tr.id, '')     AS last_run_id,
		       COALESCE(tr.name, '')   AS last_run_name,
		       COALESCE(rr.status, '') AS last_result_status
		FROM (
			SELECT tc.id AS id, tc.name AS name,
			       (SELECT r2.id FROM run_results r2
			        WHERE r2.test_case_id = tc.id AND r2.status IN ?
			        ORDER BY CASE WHEN r2.start_time > '0002-01-01' THEN r2.start_time ELSE r2.updated_at END DESC,
			                 r2.attempt_number DESC
			        LIMIT 1) AS last_result_id
			FROM test_cases tc
			WHERE tc.id IN (SELECT dl.test_case_id FROM defect_links dl
			                WHERE dl.defect_id = ? AND dl.test_case_id IS NOT NULL)
		) t
		LEFT JOIN run_results rr ON rr.id = t.last_result_id
		LEFT JOIN test_runs tr ON tr.id = rr.test_run_id
		ORDER BY t.name`, models.FailureStatuses, defectID).Scan(&out).Error
	return out, err
}

func (s *Store) ListDefectsByResult(runResultID string) ([]models.Defect, error) {
	var defects []models.Defect
	err := s.db.Raw(`
		SELECT DISTINCT d.* FROM defects d JOIN defect_links dl ON dl.defect_id = d.id
		WHERE dl.run_result_id = ? ORDER BY dl.created_at DESC`, runResultID).Scan(&defects).Error
	return defects, err
}

func (s *Store) ListDefectsByTestCase(testCaseID string) ([]models.Defect, error) {
	var defects []models.Defect
	err := s.db.Raw(`
		SELECT DISTINCT d.* FROM defects d JOIN defect_links dl ON dl.defect_id = d.id
		WHERE dl.test_case_id = ? ORDER BY d.created_at DESC`, testCaseID).Scan(&defects).Error
	return defects, err
}

// affectedTestCaseIDs returns the distinct test cases linked to the given defects — the union, when
// several are passed, so a bulk status change recomputes each test case exactly once.
func affectedTestCaseIDs(tx *gorm.DB, defectIDs ...string) []string {
	if len(defectIDs) == 0 {
		return nil
	}
	var ids []string
	_ = tx.Model(&models.DefectLink{}).
		Where("defect_id IN ? AND test_case_id IS NOT NULL", defectIDs).
		Distinct().Pluck("test_case_id", &ids).Error
	return ids
}

// recomputeReverification sets reverification_flagged per test case: it raises the flag when a
// test case has at least one linked defect and every one of them is fixed-or-closed, i.e. the
// test is ready to be retested. "fixed" counts as resolved alongside "closed" so the signal
// fires when QA actually wants it — before the defect is closed, not after.
//
// Grouped, not per test case: bulk-closing 500 defects runs this inside ONE SQLite write
// transaction, and three statements per affected test case meant holding the database's single
// write lock for 3N round-trips while every other writer queued behind it. It is now three
// statements total, whatever N is. A test case with no linked defects left (the delete and
// unlink paths reach here with exactly that) simply misses the grouped count and lands in
// `cleared`, which is what the per-row loop's total == 0 case did.
func recomputeReverification(tx *gorm.DB, testCaseIDs []string) error {
	ids := make([]string, 0, len(testCaseIDs))
	seen := make(map[string]bool, len(testCaseIDs))
	for _, id := range testCaseIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil
	}

	type tally struct {
		TestCaseID string
		Unresolved int64
	}
	var rows []tally
	// The statuses are compile-time constants, so they are inlined rather than bound: a
	// placeholder inside a CASE expression in a SELECT list is where GORM's arg handling
	// is least predictable, and there is no user input here to protect.
	if err := tx.Model(&models.DefectLink{}).
		Select("defect_links.test_case_id AS test_case_id, "+
			"COUNT(DISTINCT CASE WHEN d.status NOT IN ('closed','fixed') THEN d.id END) AS unresolved").
		Joins("JOIN defects d ON d.id = defect_links.defect_id").
		Where("defect_links.test_case_id IN ?", ids).
		Group("defect_links.test_case_id").Scan(&rows).Error; err != nil {
		return err
	}

	flagged := make([]string, 0, len(ids))
	ready := make(map[string]bool, len(rows))
	for _, r := range rows {
		// A row exists only when the test case has at least one linked defect, so the
		// "total > 0" half of the old condition is implied by being here at all.
		ready[r.TestCaseID] = r.Unresolved == 0
		if r.Unresolved == 0 {
			flagged = append(flagged, r.TestCaseID)
		}
	}
	cleared := make([]string, 0, len(ids))
	for _, id := range ids {
		if !ready[id] {
			cleared = append(cleared, id)
		}
	}

	if len(flagged) > 0 {
		if err := tx.Model(&models.TestCase{}).Where("id IN ?", flagged).
			Update("reverification_flagged", true).Error; err != nil {
			return err
		}
	}
	if len(cleared) > 0 {
		if err := tx.Model(&models.TestCase{}).Where("id IN ?", cleared).
			Update("reverification_flagged", false).Error; err != nil {
			return err
		}
	}
	return nil
}
