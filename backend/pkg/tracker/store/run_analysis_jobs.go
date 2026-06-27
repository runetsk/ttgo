package store

import (
	"errors"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MaybeEnqueueForRun inserts a new queued RunAnalysisJob for runID, OR
// returns the existing active (queued/running) job if one is already in
// flight. created==true only when a new row was written.
func (s *Store) MaybeEnqueueForRun(runID, trigger, createdBy string) (*models.RunAnalysisJob, bool, error) {
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

// MarkAnalysisJobRunning transitions status to running and sets StartedAt.
func (s *Store) MarkAnalysisJobRunning(id string) error {
	now := time.Now()
	return s.db.Model(&models.RunAnalysisJob{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     models.RunAnalysisJobStatusRunning,
			"started_at": &now,
		}).Error
}

// UpdateAnalysisJobStatus moves a job to a terminal state (completed/failed/cancelled).
func (s *Store) UpdateAnalysisJobStatus(id, status, errorMessage string) error {
	updates := map[string]interface{}{"status": status}
	if status == models.RunAnalysisJobStatusCompleted ||
		status == models.RunAnalysisJobStatusFailed ||
		status == models.RunAnalysisJobStatusCancelled {
		now := time.Now()
		updates["completed_at"] = &now
	}
	if errorMessage != "" {
		updates["error_message"] = errorMessage
	}
	return s.db.Model(&models.RunAnalysisJob{}).Where("id = ?", id).Updates(updates).Error
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
