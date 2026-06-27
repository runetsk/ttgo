package store

import (
	"fmt"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateAnalysis appends a new analysis row for a RunResult, assigning
// Version = MAX(version)+1 under a process-wide mutex so parallel callers
// always get distinct monotonic versions.
//
// The passed model has ID/Version/CreatedAt filled in by this method; any
// pre-set values are overwritten.
func (s *Store) CreateAnalysis(a *models.RunResultAnalysis) (*models.RunResultAnalysis, error) {
	if a.RunResultID == "" {
		return nil, fmt.Errorf("run_result_id is required")
	}
	a.ID = uuid.New().String()
	a.CreatedAt = time.Now()

	// Serialize the SELECT MAX → INSERT critical section: with GORM's default
	// connection pool, BEGIN IMMEDIATE can't reliably span multiple queries on
	// a shared *gorm.DB. A sync.Mutex is both simpler and strictly correct for
	// the TTGO single-process model.
	s.analysisMu.Lock()
	defer s.analysisMu.Unlock()

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var maxVer int
		if err := tx.Raw(`SELECT COALESCE(MAX(version), 0) FROM run_result_analyses WHERE run_result_id = ?`,
			a.RunResultID).Scan(&maxVer).Error; err != nil {
			return err
		}
		a.Version = maxVer + 1
		return tx.Create(a).Error
	})
	if err != nil {
		return nil, err
	}
	return a, nil
}

// ListAnalysesForResult returns all versions for a single result, newest first.
func (s *Store) ListAnalysesForResult(runResultID string) ([]*models.RunResultAnalysis, error) {
	var out []*models.RunResultAnalysis
	err := s.db.
		Where("run_result_id = ?", runResultID).
		Order("version DESC").
		Find(&out).Error
	return out, err
}

// GetCurrentAnalysesByRun returns the newest-version analysis for every
// RunResult in the given run. The map key is run_result_id. Results with
// no analysis are simply absent from the map.
func (s *Store) GetCurrentAnalysesByRun(runID string) (map[string]*models.RunResultAnalysis, error) {
	var rows []*models.RunResultAnalysis
	err := s.db.Raw(`
		SELECT a.* FROM run_result_analyses a
		JOIN run_results rr ON rr.id = a.run_result_id
		WHERE rr.test_run_id = ?
		  AND a.version = (
		    SELECT MAX(a2.version) FROM run_result_analyses a2
		    WHERE a2.run_result_id = a.run_result_id
		  )
	`, runID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]*models.RunResultAnalysis, len(rows))
	for _, r := range rows {
		out[r.RunResultID] = r
	}
	return out, nil
}
