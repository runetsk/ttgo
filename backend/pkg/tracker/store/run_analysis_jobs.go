package store

import (
	"encoding/json"
	"errors"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MaybeEnqueueForRun inserts a new queued RunAnalysisJob for runID, OR
// returns the existing active (queued/running) job if one is already in
// flight. created==true only when a new row was written.
func (s *Store) MaybeEnqueueForRun(runID, trigger, createdBy string) (*models.RunAnalysisJob, bool, error) {
	return s.maybeEnqueue(runID, trigger, createdBy, false)
}

func (s *Store) maybeEnqueue(runID, trigger, createdBy string, retryFailedOnly bool) (*models.RunAnalysisJob, bool, error) {
	// activeJob returns the current queued/running job for the run, if any.
	activeJob := func() (*models.RunAnalysisJob, error) {
		var j models.RunAnalysisJob
		err := s.db.Where(`test_run_id = ? AND status IN ?`, runID,
			[]string{models.RunAnalysisJobStatusQueued, models.RunAnalysisJobStatusRunning}).
			Order("created_at DESC").First(&j).Error
		if err != nil {
			return nil, err
		}
		return &j, nil
	}

	// Fast path: an active job already exists.
	if existing, err := activeJob(); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}

	job := &models.RunAnalysisJob{
		ID:        uuid.New().String(),
		TestRunID: runID,
		Trigger:   trigger,
		Status:    models.RunAnalysisJobStatusQueued,
		CreatedAt: time.Now(),

		RetryFailedOnly: retryFailedOnly,
	}
	if createdBy != "" {
		job.CreatedBy = &createdBy
	}
	// The partial unique index uq_run_analysis_active enforces at most one active
	// job per run, so two concurrent enqueues cannot both insert. The loser's
	// Create fails on the constraint; re-read and return the winner instead of
	// creating a duplicate that would double the LLM spend (F-008).
	if err := s.db.Create(job).Error; err != nil {
		if existing, e2 := activeJob(); e2 == nil {
			return existing, false, nil
		}
		return nil, false, err
	}
	return job, true, nil
}

// EnqueueScopedAnalysis queues a manual job limited to resultIDs (spec Wave 4 §4) and, in the same
// transaction, records humanPairs (a person's split). Like MaybeEnqueueForRun it returns the
// already active job instead (created false), and then writes nothing.
func (s *Store) EnqueueScopedAnalysis(runID, createdBy string, resultIDs []string, splitFrom string, humanPairs []*models.SemanticPair) (*models.RunAnalysisJob, bool, error) {
	scope, err := json.Marshal(resultIDs)
	if err != nil {
		return nil, false, err
	}
	activeJob := func() (*models.RunAnalysisJob, error) {
		var j models.RunAnalysisJob
		err := s.db.Where(`test_run_id = ? AND status IN ?`, runID,
			[]string{models.RunAnalysisJobStatusQueued, models.RunAnalysisJobStatusRunning}).
			Order("created_at DESC").First(&j).Error
		if err != nil {
			return nil, err
		}
		return &j, nil
	}
	if existing, err := activeJob(); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	job := &models.RunAnalysisJob{
		ID: uuid.New().String(), TestRunID: runID, Trigger: models.RunAnalysisJobTriggerManual,
		Status: models.RunAnalysisJobStatusQueued, CreatedAt: time.Now(), ScopeResultIDs: string(scope),
	}
	if createdBy != "" {
		job.CreatedBy = &createdBy
	}
	if splitFrom != "" {
		job.SplitFromAnalysisID = &splitFrom
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		for _, p := range humanPairs {
			p.SigA, p.SigB = orderedSigs(p.SigA, p.SigB)
			if p.ID == "" {
				p.ID = uuid.New().String()
			}
			p.CreatedAt = now
			// An earlier split already pinned this pair (uq_semantic_pairs_human): keep that row.
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(p).Error; err != nil {
				return err
			}
		}
		return tx.Create(job).Error
	})
	if err != nil {
		// The one-active-job index: a concurrent enqueue won; nothing of this one was written.
		if existing, e2 := activeJob(); e2 == nil {
			return existing, false, nil
		}
		return nil, false, err
	}
	return job, true, nil
}

// ScopeResultIDs decodes a job's result scope; nil = the whole run.
func ScopeResultIDs(job *models.RunAnalysisJob) map[string]bool {
	if job == nil || job.ScopeResultIDs == "" {
		return nil
	}
	var ids []string
	if json.Unmarshal([]byte(job.ScopeResultIDs), &ids) != nil {
		return nil
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out
}

// GetAnalysisJob fetches a single job by ID.
func (s *Store) GetAnalysisJob(id string) (*models.RunAnalysisJob, error) {
	var job models.RunAnalysisJob
	if err := s.db.First(&job, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

// GetLatestAnalysisJobForRun returns the most recent job for a run, or
// (nil, nil) if none.
func (s *Store) GetLatestAnalysisJobForRun(runID string) (*models.RunAnalysisJob, error) {
	var job models.RunAnalysisJob
	err := s.db.Where("test_run_id = ?", runID).Order("created_at DESC").First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// NextQueuedAnalysisJob returns the oldest queued job, or (nil, nil) if none.
func (s *Store) NextQueuedAnalysisJob() (*models.RunAnalysisJob, error) {
	var job models.RunAnalysisJob
	err := s.db.Where("status = ?", models.RunAnalysisJobStatusQueued).
		Order("created_at ASC").First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// MarkAnalysisJobRunning claims a queued job. It is conditional on the job still being
// queued, so a cancel that lands between pick-up and claim wins; returns whether it claimed.
func (s *Store) MarkAnalysisJobRunning(id string) (bool, error) {
	now := time.Now()
	res := s.db.Model(&models.RunAnalysisJob{}).
		Where("id = ? AND status = ?", id, models.RunAnalysisJobStatusQueued).
		Updates(map[string]interface{}{
			"status":     models.RunAnalysisJobStatusRunning,
			"started_at": &now,
		})
	return res.RowsAffected > 0, res.Error
}

// UpdateAnalysisJobStatus moves a job to a terminal state. completed and failed are
// only ever written over a job that is still running, so a worker finishing after a
// cancel can never overwrite "cancelled"; cancelled itself may be written over queued or
// running. Returns whether a row actually changed.
func (s *Store) UpdateAnalysisJobStatus(id, status, errorMessage string) (bool, error) {
	updates := map[string]interface{}{"status": status}
	terminal := status == models.RunAnalysisJobStatusCompleted ||
		status == models.RunAnalysisJobStatusFailed ||
		status == models.RunAnalysisJobStatusCancelled
	if terminal {
		now := time.Now()
		updates["completed_at"] = &now
	}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}
	q := s.db.Model(&models.RunAnalysisJob{}).Where("id = ?", id)
	switch status {
	case models.RunAnalysisJobStatusCompleted, models.RunAnalysisJobStatusFailed:
		q = q.Where("status = ?", models.RunAnalysisJobStatusRunning)
	case models.RunAnalysisJobStatusCancelled:
		q = q.Where("status IN ?", []string{models.RunAnalysisJobStatusQueued, models.RunAnalysisJobStatusRunning})
	}
	res := q.Updates(updates)
	return res.RowsAffected > 0, res.Error
}

// SetAnalysisJobSemanticTokens records the TypeSafe input tokens the semantic pass used.
func (s *Store) SetAnalysisJobSemanticTokens(id string, tokens int) error {
	return s.db.Model(&models.RunAnalysisJob{}).Where("id = ?", id).
		Update("semantic_input_tokens", tokens).Error
}

// UpdateAnalysisJobProgress bumps analyzed_count and sets capped_at / unique_groups / total_failures.
func (s *Store) UpdateAnalysisJobProgress(id string, analyzedCount, uniqueGroups, cappedAt, totalFailures int) error {
	return s.db.Model(&models.RunAnalysisJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"analyzed_count": analyzedCount,
			"unique_groups":  uniqueGroups,
			"capped_at":      cappedAt,
			"total_failures": totalFailures,
		}).Error
}

// SweepRunningAnalysisJobs marks any rows stuck in "running" as "failed".
// Called once at server startup to recover from crashes.
func (s *Store) SweepRunningAnalysisJobs() (int64, error) {
	now := time.Now()
	r := s.db.Model(&models.RunAnalysisJob{}).
		Where("status = ?", models.RunAnalysisJobStatusRunning).
		Updates(map[string]interface{}{
			"status":        models.RunAnalysisJobStatusFailed,
			"error_message": "interrupted by restart",
			"completed_at":  &now,
		})
	return r.RowsAffected, r.Error
}

// EnqueueRetryFailedForRun queues a job that re-analyzes only the groups whose current
// analysis failed. Like MaybeEnqueueForRun it returns an already active job instead.
func (s *Store) EnqueueRetryFailedForRun(runID, createdBy string) (*models.RunAnalysisJob, bool, error) {
	return s.maybeEnqueue(runID, models.RunAnalysisJobTriggerManual, createdBy, true)
}

// SetAnalysisJobPipeline records the route a job runs with.
func (s *Store) SetAnalysisJobPipeline(id, pipelineJSON, label string) error {
	return s.db.Model(&models.RunAnalysisJob{}).Where("id = ?", id).
		Updates(map[string]interface{}{"pipeline": pipelineJSON, "pipeline_label": label}).Error
}

// ListAnalysisJobsForRun returns every analysis job of a run, newest first.
func (s *Store) ListAnalysisJobsForRun(runID string) ([]*models.RunAnalysisJob, error) {
	var out []*models.RunAnalysisJob
	err := s.db.Where("test_run_id = ?", runID).Order("created_at DESC").Find(&out).Error
	return out, err
}

// CreateSkippedAnalysisJob records an automatic analysis that was not queued because spent +
// its estimate would exceed the monthly budget. It is terminal when written, so the
// one-active-job rule (queued/running only) never counts it and a later start is not blocked.
func (s *Store) CreateSkippedAnalysisJob(runID, reason string, estimateUSD, spentUSD, budgetUSD float64) (*models.RunAnalysisJob, error) {
	now := time.Now()
	job := &models.RunAnalysisJob{
		ID:          uuid.New().String(),
		TestRunID:   runID,
		Trigger:     models.RunAnalysisJobTriggerAutoOnDone,
		Status:      models.RunAnalysisJobStatusSkipped,
		CreatedAt:   now,
		CompletedAt: &now,

		SkipReason: reason, SkipEstimateUSD: &estimateUSD, SkipSpentUSD: &spentUSD, SkipBudgetUSD: &budgetUSD,
	}
	if err := s.db.Create(job).Error; err != nil {
		return nil, err
	}
	return job, nil
}

// SetAnalysisJobRateLimitHits records how many rate-limit responses the job has seen.
func (s *Store) SetAnalysisJobRateLimitHits(id string, hits int) error {
	return s.db.Model(&models.RunAnalysisJob{}).Where("id = ?", id).Update("rate_limit_hits", hits).Error
}

// SetAnalysisJobCallStats records the job's call telemetry: rate-limit responses, LLM calls cut
// by the per-call timeout, hedged requests sent and hedges that won.
func (s *Store) SetAnalysisJobCallStats(id string, rateLimits, callTimeouts, hedgesFired, hedgesWon int) error {
	return s.db.Model(&models.RunAnalysisJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"rate_limit_hits": rateLimits,
		"call_timeouts":   callTimeouts,
		"hedges_fired":    hedgesFired,
		"hedges_won":      hedgesWon,
	}).Error
}

// AddAnalysisJobTransferStats adds one narrated group's transfer-check outcome to its job: 1 when
// the check failed, and the semantic clones it left unchecked. Increments, because the job's
// groups report one at a time; zero changes write nothing.
func (s *Store) AddAnalysisJobTransferStats(id string, failed, unchecked int) error {
	if failed == 0 && unchecked == 0 {
		return nil
	}
	return s.db.Model(&models.RunAnalysisJob{}).Where("id = ?", id).Updates(map[string]interface{}{
		"transfer_check_failed": gorm.Expr("transfer_check_failed + ?", failed),
		"transfer_unchecked":    gorm.Expr("transfer_unchecked + ?", unchecked),
	}).Error
}

// SetAnalysisJobAutoApplyState records whether the job may label results by itself, as resolved
// when it started (models.AutoApplyStateOff, On or Paused).
func (s *Store) SetAnalysisJobAutoApplyState(id, state string) error {
	return s.db.Model(&models.RunAnalysisJob{}).Where("id = ?", id).Update("auto_apply_state", state).Error
}

// AddAnalysisJobAutoApplied adds n results labelled by auto-apply to the job's count.
func (s *Store) AddAnalysisJobAutoApplied(id string, n int64) error {
	if n <= 0 {
		return nil
	}
	return s.db.Model(&models.RunAnalysisJob{}).Where("id = ?", id).
		Update("auto_applied", gorm.Expr("auto_applied + ?", n)).Error
}
