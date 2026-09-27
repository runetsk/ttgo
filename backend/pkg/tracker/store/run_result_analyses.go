package store

import (
	"errors"
	"fmt"
	"math"
	"sort"
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
	// Write-time defaults (spec §5 resolution rule). A typesafe row keeps an empty
	// suggestion: that means "abstained" and must never be derived.
	if a.Engine == "" {
		a.Engine = models.AnalysisEngineGenerative
	}
	if a.NarrativeStatus == "" {
		a.NarrativeStatus = models.NarrativeStatusOK
	}
	if a.DecisionStatus == "" {
		a.DecisionStatus = models.DecisionStatusOK
	}
	if a.Engine == models.AnalysisEngineGenerative && a.SuggestedDefectType == "" {
		a.SuggestedDefectType = models.SuggestedDefectType(a.Verdict)
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

// GetCurrentAnalysisForResult returns the newest-version analysis for a single
// result, or (nil, nil) when the result has no analysis at all.
func (s *Store) GetCurrentAnalysisForResult(runResultID string) (*models.RunResultAnalysis, error) {
	// LIMIT 1 rather than reusing ListAnalysesForResult: this runs on the triage write path, and
	// every row carries a full RawResponse LLM blob, so loading the whole version history to
	// keep the first record would read (and discard) kilobytes per re-analysis.
	var out models.RunResultAnalysis
	err := s.db.
		Where("run_result_id = ?", runResultID).
		Order("version DESC").
		Limit(1).
		Take(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
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

// GetAnalysisByID returns one analysis version, or (nil, nil) when it does not exist.
func (s *Store) GetAnalysisByID(id string) (*models.RunResultAnalysis, error) {
	var out models.RunResultAnalysis
	err := s.db.Where("id = ?", id).Take(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// NarrativeUpdate is an explanation written after the decision was stored (on-demand
// "Explain"). Token and timing fields are added to what the analysis already recorded.
type NarrativeUpdate struct {
	Summary, NextAction, Rationale, NarrativeStatus string
	AddPrompt, AddCompletion, AddLLMMs, AddLLMCalls int
	FinishReason                                    string
}

// UpdateAnalysisNarrative fills in the explanation of an existing analysis without touching
// its decision. The decision stays immutable; only the narrative fields change.
func (s *Store) UpdateAnalysisNarrative(id string, u NarrativeUpdate) (*models.RunResultAnalysis, error) {
	err := s.db.Model(&models.RunResultAnalysis{}).Where("id = ?", id).Updates(map[string]interface{}{
		"summary":                u.Summary,
		"next_action":            u.NextAction,
		"rationale":              u.Rationale,
		"narrative_status":       u.NarrativeStatus,
		"token_usage_prompt":     gorm.Expr("token_usage_prompt + ?", u.AddPrompt),
		"token_usage_completion": gorm.Expr("token_usage_completion + ?", u.AddCompletion),
		"llm_ms":                 gorm.Expr("llm_ms + ?", u.AddLLMMs),
		"llm_calls":              gorm.Expr("llm_calls + ?", u.AddLLMCalls),
		"finish_reason":          u.FinishReason,
	}).Error
	if err != nil {
		return nil, err
	}
	return s.GetAnalysisByID(id)
}

// FailedResultIDsForRun returns the failing results of a run whose current analysis is a
// failed attempt, so a retry can re-analyze just those.
func (s *Store) FailedResultIDsForRun(runID string) (map[string]bool, error) {
	current, err := s.GetCurrentAnalysesByRun(runID)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for id, a := range current {
		if a.Failed() {
			out[id] = true
		}
	}
	return out, nil
}

// AnalysisJobOutcomes counts what a job's analyses produced. Representatives are the rows
// the job analyzed itself (no source analysis); FailedRows counts clones too.
func (s *Store) AnalysisJobOutcomes(jobID string) (models.RunAnalysisJobOutcomes, error) {
	var o models.RunAnalysisJobOutcomes
	err := s.db.Raw(`
		SELECT
		  COUNT(*) AS groups,
		  COALESCE(SUM(CASE WHEN decision_status = 'ok' THEN 1 ELSE 0 END), 0) AS decided,
		  COALESCE(SUM(CASE WHEN decision_status = 'failed' THEN 1 ELSE 0 END), 0) AS failed,
		  COALESCE(SUM(CASE WHEN decision_status = 'ok' AND verdict = 'unknown' THEN 1 ELSE 0 END), 0) AS unknown,
		  COALESCE(SUM(CASE WHEN decision_status = 'ok' AND narrative_status IN ('unavailable', 'unparseable') THEN 1 ELSE 0 END), 0) AS no_explanation,
		  COALESCE(SUM(CASE WHEN decision_status = 'ok' AND narrative_status = 'skipped' THEN 1 ELSE 0 END), 0) AS explanation_skipped,
		  COALESCE(SUM(CASE WHEN decision_status = 'ok' AND takeover_from_verdict != '' THEN 1 ELSE 0 END), 0) AS taken_over
		FROM run_result_analyses WHERE job_id = ? AND source_analysis_id IS NULL`, jobID).Scan(&o).Error
	if err != nil {
		return o, err
	}
	err = s.db.Raw(`SELECT COUNT(*) FROM run_result_analyses WHERE job_id = ? AND source_analysis_id IS NULL
		AND decision_status = 'failed' AND error_category = 'configuration'`, jobID).Scan(&o.FailedConfiguration).Error
	if err != nil {
		return o, err
	}
	err = s.db.Raw(`SELECT COUNT(*) FROM run_result_analyses WHERE job_id = ? AND decision_status = 'failed'`, jobID).
		Scan(&o.FailedRows).Error
	if err != nil {
		return o, err
	}
	var timings []struct{ D, L int }
	if err := s.db.Raw(`SELECT decision_ms AS d, llm_ms AS l FROM run_result_analyses
		WHERE job_id = ? AND source_analysis_id IS NULL`, jobID).Scan(&timings).Error; err != nil {
		return o, err
	}
	var decision, llmMs []int
	for _, t := range timings {
		if t.D > 0 {
			decision = append(decision, t.D)
		}
		if t.L > 0 {
			llmMs = append(llmMs, t.L)
		}
	}
	o.DecisionMsAvg, o.DecisionMsP50, o.DecisionMsMax = msStats(decision)
	o.LLMMsAvg, o.LLMMsP50, o.LLMMsMax = msStats(llmMs)
	err = s.db.Raw(`SELECT COALESCE(MAX(rate_limit_hits), 0) FROM run_analysis_jobs WHERE id = ?`, jobID).
		Scan(&o.RateLimitHits).Error
	return o, err
}

// msStats is the rounded mean, nearest-rank median and maximum of millisecond samples.
func msStats(v []int) (avg, p50, mx int) {
	if len(v) == 0 {
		return 0, 0, 0
	}
	sorted := append([]int(nil), v...)
	sort.Ints(sorted)
	sum := 0
	for _, x := range sorted {
		sum += x
	}
	return int(math.Round(float64(sum) / float64(len(sorted)))), sorted[(len(sorted)+1)/2-1], sorted[len(sorted)-1]
}
