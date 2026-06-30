package store

import (
	"errors"
	"fmt"
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
	now := time.Now()
	d.CreatedAt = now
	d.UpdatedAt = now
	return s.db.Create(d).Error
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
	return &d, nil
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
	if len(defects) == 0 {
		return defects, nil
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
		return nil, err
	}
	byID := make(map[string]int, len(counts))
	for _, c := range counts {
		byID[c.DefectID] = c.N
	}
	for i := range defects {
		defects[i].LinkedTestCount = byID[defects[i].ID]
	}
	return defects, nil
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

func affectedTestCaseIDs(tx *gorm.DB, defectID string) []string {
	var ids []string
	_ = tx.Model(&models.DefectLink{}).
		Where("defect_id = ? AND test_case_id IS NOT NULL", defectID).
		Distinct().Pluck("test_case_id", &ids).Error
	return ids
}

// recomputeReverification sets reverification_flagged = (>=1 linked defect AND all closed) per test case.
func recomputeReverification(tx *gorm.DB, testCaseIDs []string) error {
	for _, tcID := range testCaseIDs {
		if tcID == "" {
			continue
		}
		var total, open int64
		if err := tx.Model(&models.DefectLink{}).Joins("JOIN defects d ON d.id = defect_links.defect_id").
			Where("defect_links.test_case_id = ?", tcID).Distinct("d.id").Count(&total).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.DefectLink{}).Joins("JOIN defects d ON d.id = defect_links.defect_id").
			Where("defect_links.test_case_id = ? AND d.status != ?", tcID, "closed").Distinct("d.id").Count(&open).Error; err != nil {
			return err
		}
		flagged := total > 0 && open == 0
		if err := tx.Model(&models.TestCase{}).Where("id = ?", tcID).
			Update("reverification_flagged", flagged).Error; err != nil {
			return err
		}
	}
	return nil
}
