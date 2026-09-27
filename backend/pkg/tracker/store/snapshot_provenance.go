package store

import (
	"time"

	"ttgo/pkg/tracker/models"
)

// SnapshotMatchesAnalysis reports whether a triage snapshot (the suggested_* columns of a
// run_results row) is exactly what the triage handlers' snapshotValues would write for analysis
// a. It mirrors that function column by column; internal/api/runs pins the two together with a
// test, so neither can change alone.
func SnapshotMatchesAnalysis(a *models.RunResultAnalysis, verdict, defectType, confidence, engine string, score *float64) bool {
	wantEngine := a.Engine
	if wantEngine == "" {
		wantEngine = models.AnalysisEngineGenerative
	}
	if wantEngine == models.AnalysisEngineTypeSafe && a.SuggestionSource == models.SuggestionSourceVerdict {
		wantEngine = models.AnalysisEngineTypeSafeDerived
	}
	wantConfidence, wantScore := a.Confidence, (*float64)(nil)
	if a.Engine == models.AnalysisEngineTypeSafe && a.SuggestedDefectTypeConfidence != nil {
		wantScore = a.SuggestedDefectTypeConfidence
		wantConfidence = snapshotConfidenceBucket(*wantScore)
	}
	if a.Verdict != verdict || a.SuggestedDefectType != defectType || wantConfidence != confidence || wantEngine != engine {
		return false
	}
	if (wantScore == nil) != (score == nil) {
		return false
	}
	return wantScore == nil || *wantScore == *score
}

// snapshotConfidenceBucket mirrors the triage snapshot's 0.9 / 0.5 buckets.
func snapshotConfidenceBucket(score float64) string {
	switch {
	case score >= 0.9:
		return models.ConfidenceHigh
	case score >= 0.5:
		return models.ConfidenceMedium
	default:
		return models.ConfidenceLow
	}
}

// backfillSnapshotProvenance fills suggested_policy_version and suggested_is_clone on triage
// snapshots recorded before those columns existed. bootstrapDB runs it only on the boot that
// adds suggested_is_clone, so it runs once per database.
//
// Conservative by design (spec B1): a row is filled only when exactly one analysis of its result
// was created at or before decided_at and reproduces every snapshot dimension (verdict, defect
// type, confidence and score, engine and suggestion source), and that analysis is the newest
// version at the decision, the one the snapshot was read from. No candidate, two candidates or a
// newer non-matching version all leave the row NULL. The accuracy report counts NULL rows in its
// totals but not in the direct/clone split.
//
// Times are compared as instants in Go, not as TEXT in SQL: decided_at is written in UTC and
// analysis created_at in server-local time, so a TEXT comparison would be skewed by the offset.
func (s *Store) backfillSnapshotProvenance() error {
	type snapshot struct {
		ID                       string
		SuggestedVerdict         string
		SuggestedDefectType      string
		SuggestedConfidence      string
		SuggestedEngine          string
		SuggestedConfidenceScore *float64
		DecidedAt                time.Time
	}
	var snaps []snapshot
	if err := s.db.Raw(`
		SELECT id, suggested_verdict, suggested_defect_type, suggested_confidence,
		       suggested_engine, suggested_confidence_score, decided_at
		  FROM run_results
		 WHERE decided_at IS NOT NULL AND suggested_verdict != '' AND suggested_is_clone IS NULL`).Scan(&snaps).Error; err != nil {
		return err
	}
	for _, r := range snaps {
		var analyses []models.RunResultAnalysis
		if err := s.db.Select("id", "version", "verdict", "confidence", "engine", "suggested_defect_type",
			"suggested_defect_type_confidence", "suggestion_source", "policy_version", "source_analysis_id", "created_at").
			Where("run_result_id = ?", r.ID).Order("version DESC").Find(&analyses).Error; err != nil {
			return err
		}
		var match *models.RunResultAnalysis
		matches, newest := 0, true
		for i := range analyses {
			a := &analyses[i]
			if a.CreatedAt.After(r.DecidedAt) {
				continue // written after the decision: cannot be what the human saw
			}
			current := newest
			newest = false
			if SnapshotMatchesAnalysis(a, r.SuggestedVerdict, r.SuggestedDefectType, r.SuggestedConfidence, r.SuggestedEngine, r.SuggestedConfidenceScore) {
				matches++
				if current {
					match = a
				}
			}
		}
		if matches != 1 || match == nil {
			continue
		}
		if err := s.db.Exec(`UPDATE run_results SET suggested_policy_version = ?, suggested_is_clone = ? WHERE id = ?`,
			match.PolicyVersion, match.SourceAnalysisID != nil, r.ID).Error; err != nil {
			return err
		}
	}
	return nil
}
