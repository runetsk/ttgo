package store

import (
	"errors"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// orderedSigs returns a pair of signatures in the stored orientation (SigA < SigB).
func orderedSigs(a, b string) (string, string) {
	if a > b {
		return b, a
	}
	return a, b
}

// SaveSemanticPairs records every pair a completed semantic pass decided (spec Wave 4 §1).
// model is the configured TypeSafe model, the memory key.
func (s *Store) SaveSemanticPairs(jobID, runID, model, policy string, pairs []failureanalysis.SemanticPairOutcome) error {
	if len(pairs) == 0 {
		return nil
	}
	now := time.Now().UTC()
	rows := make([]*models.SemanticPair, 0, len(pairs))
	for _, p := range pairs {
		a, b := orderedSigs(p.SigA, p.SigB)
		ra, rb := p.ResultA, p.ResultB
		if a != p.SigA {
			ra, rb = rb, ra
		}
		var ps *float64
		if p.P != nil {
			v := *p.P
			ps = &v
		}
		rows = append(rows, &models.SemanticPair{ID: uuid.New().String(), JobID: jobID, RunID: runID, SigA: a, SigB: b,
			ResultAID: ra, ResultBID: rb, PSame: ps, Model: model, AnsweredModel: p.AnsweredModel, PolicyVersion: policy,
			Source: p.Source, SourcePairID: p.SourcePairID, Merged: p.Merged, CreatedAt: now})
	}
	return s.db.CreateInBatches(rows, 200).Error
}

// SemanticMemory returns the lookup the semantic pass consults before asking (spec Wave 4 §2): a
// human row for the pair wins at any age and for any model; otherwise the newest typesafe row for
// the same configured model and policy, created within failureanalysis.SemanticMemoryDays of now.
// memory rows are never a source, so a remembered answer cannot outlive its original. A failed
// read is treated as "not remembered" (the pair is asked).
func (s *Store) SemanticMemory(model, policy string, now time.Time) func(a, b string) (failureanalysis.Remembered, bool) {
	since := now.UTC().AddDate(0, 0, -failureanalysis.SemanticMemoryDays)
	return func(x, y string) (failureanalysis.Remembered, bool) {
		a, b := orderedSigs(x, y)
		var human models.SemanticPair
		err := s.db.Where("sig_a = ? AND sig_b = ? AND source = ?", a, b, failureanalysis.SemanticSourceHuman).
			Order("created_at DESC").Take(&human).Error
		if err == nil {
			return failureanalysis.Remembered{ID: human.ID, Source: failureanalysis.SemanticSourceHuman, CreatedAt: human.CreatedAt}, true
		}
		var row models.SemanticPair
		err = s.db.Where("sig_a = ? AND sig_b = ? AND source = ? AND model = ? AND policy_version = ? AND p_same IS NOT NULL AND created_at >= ?",
			a, b, failureanalysis.SemanticSourceTypeSafe, model, policy, since).
			Order("created_at DESC").Take(&row).Error
		if err != nil {
			return failureanalysis.Remembered{}, false
		}
		return failureanalysis.Remembered{ID: row.ID, P: *row.PSame, Source: failureanalysis.SemanticSourceTypeSafe,
			AnsweredModel: row.AnsweredModel, CreatedAt: row.CreatedAt}, true
	}
}

// SetAnalysisJobSemanticReport stores the semantic pass's counts (JSON text) on the job.
func (s *Store) SetAnalysisJobSemanticReport(id, report string) error {
	return s.db.Model(&models.RunAnalysisJob{}).Where("id = ?", id).Update("semantic_report", report).Error
}

// ListSemanticPairsForJob returns the job's pair rows, oldest first.
func (s *Store) ListSemanticPairsForJob(jobID string) ([]*models.SemanticPair, error) {
	var out []*models.SemanticPair
	err := s.db.Where("job_id = ?", jobID).Order("created_at, sig_a, sig_b").Find(&out).Error
	return out, err
}

// SemanticPairFor returns the job's non-human row for a signature pair, or nil.
func (s *Store) SemanticPairFor(jobID, x, y string) (*models.SemanticPair, error) {
	a, b := orderedSigs(x, y)
	var row models.SemanticPair
	err := s.db.Where("job_id = ? AND sig_a = ? AND sig_b = ? AND source != ?", jobID, a, b, failureanalysis.SemanticSourceHuman).
		Order("created_at DESC").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// RememberedSource returns the row a memory row reused (source_pair_id): a TypeSafe answer or a
// person's split. nil when row is not a memory row or the source is gone.
func (s *Store) RememberedSource(row *models.SemanticPair) (*models.SemanticPair, error) {
	if row == nil || row.Source != failureanalysis.SemanticSourceMemory || row.SourcePairID == "" {
		return nil, nil
	}
	var src models.SemanticPair
	err := s.db.Where("id = ?", row.SourcePairID).Take(&src).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &src, nil
}
