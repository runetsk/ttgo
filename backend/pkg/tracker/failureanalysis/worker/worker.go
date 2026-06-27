package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
)

// Broadcaster is the narrow subset of the ws.Hub the worker needs.
type Broadcaster interface {
	BroadcastRunAnalysisProgress(job *models.RunAnalysisJob, coveredFailures int)
	BroadcastRunAnalysisCompleted(job *models.RunAnalysisJob, coveredFailures int)
	BroadcastRunResultAnalysisCreated(a *models.RunResultAnalysis, testRunID string)
}

// Worker is a polling background job runner.
type Worker struct {
	store    *store.Store
	provider llm.Provider
	bc       Broadcaster
	interval time.Duration
}

// NewWorker builds a worker with the given poll interval.
func NewWorker(s *store.Store, p llm.Provider, bc Broadcaster, interval time.Duration) *Worker {
	return &Worker{store: s, provider: p, bc: bc, interval: interval}
}

// Run blocks until ctx is cancelled, polling every interval.
func (w *Worker) Run(ctx context.Context) {
	if n, err := w.store.SweepRunningAnalysisJobs(); err != nil {
		slog.Warn("failure-analysis: restart sweep failed", "err", err)
	} else if n > 0 {
		slog.Info("failure-analysis: restart sweep marked jobs failed", "count", n)
	}

	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := w.processOnce(ctx); err != nil {
				slog.Warn("failure-analysis: process error", "err", err)
			}
		}
	}
}

// ProcessOnceForTest is an exported alias of processOnce, used by tests in
// other packages. Do not call from production code.
func (w *Worker) ProcessOnceForTest(ctx context.Context) error { return w.processOnce(ctx) }

// processOnce picks up at most one queued job and runs it to completion.
func (w *Worker) processOnce(ctx context.Context) error {
	if w.provider == nil {
		return nil
	}
	job, err := w.store.NextQueuedAnalysisJob()
	if err != nil || job == nil {
		return err
	}
	if err := w.store.MarkAnalysisJobRunning(job.ID); err != nil {
		return fmt.Errorf("mark running: %w", err)
	}

	settings, err := w.store.GetFailureAnalysisSettings()
	if err != nil {
		_ = w.store.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusFailed, "load settings: "+err.Error())
		return err
	}

	failures, err := w.store.ListLatestFailingResults(job.TestRunID)
	if err != nil {
		_ = w.store.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusFailed, "load failures: "+err.Error())
		return err
	}
	total := len(failures)

	var groups []*failureanalysis.FailureGroup
	if settings.DedupEnabled {
		groups = failureanalysis.GroupFailures(failures)
	} else {
		for _, r := range failures {
			groups = append(groups, &failureanalysis.FailureGroup{
				Key:            failureanalysis.Signature(r.FailureType, r.ErrorMessage),
				Representative: r,
				Members:        []*models.RunResult{r},
			})
		}
	}
	unique := len(groups)

	cap := settings.MaxAnalysesPerRun
	if unique < cap {
		cap = unique
	}
	if cap > len(groups) {
		cap = len(groups)
	}
	groups = groups[:cap]

	covered := 0
	for i, g := range groups {
		current, err := w.store.GetAnalysisJob(job.ID)
		if err == nil && current.Status == models.RunAnalysisJobStatusCancelled {
			slog.Info("failure-analysis: cancelled mid-job", "job_id", job.ID, "after_group", i)
			return nil
		}

		rep := g.Representative
		res, err := failureanalysis.Analyze(ctx, w.provider, failureanalysis.AnalyzeContext{
			Result:           rep,
			RedactionEnabled: settings.RedactionEnabled,
			PromptTemplate:   settings.PromptTemplate,
			ProviderModel:    "",
		})
		if err != nil {
			slog.Warn("failure-analysis: analyze failed — recording unknown verdict", "err", err, "result_id", rep.ID)
			res = &failureanalysis.AnalyzeResult{
				Verdict: models.VerdictUnknown, Confidence: models.ConfidenceLow,
				Summary: "analysis failed: " + err.Error(),
			}
		}

		repRow, err := w.store.CreateAnalysis(&models.RunResultAnalysis{
			RunResultID:          rep.ID,
			Verdict:              res.Verdict,
			Confidence:           res.Confidence,
			Summary:              res.Summary,
			NextAction:           res.NextAction,
			Rationale:            res.Rationale,
			RawResponse:          res.RawResponse,
			ModelName:            res.ModelName,
			TokenUsagePrompt:     res.TokenUsagePrompt,
			TokenUsageCompletion: res.TokenUsageCompletion,
		})
		if err != nil {
			slog.Warn("failure-analysis: persist representative failed", "err", err)
			continue
		}
		if w.bc != nil {
			w.bc.BroadcastRunResultAnalysisCreated(repRow, job.TestRunID)
		}

		for _, sib := range g.Members {
			if sib.ID == rep.ID {
				continue
			}
			groupKey := g.Key
			sourceID := repRow.ID
			cloneRow, err := w.store.CreateAnalysis(&models.RunResultAnalysis{
				RunResultID:      sib.ID,
				Verdict:          res.Verdict,
				Confidence:       res.Confidence,
				Summary:          res.Summary,
				NextAction:       res.NextAction,
				Rationale:        "[Grouped from representative analysis] " + res.Rationale,
				ModelName:        res.ModelName,
				DedupGroupKey:    &groupKey,
				SourceAnalysisID: &sourceID,
			})
			if err != nil {
				slog.Warn("failure-analysis: persist clone failed", "err", err)
				continue
			}
			if w.bc != nil {
				w.bc.BroadcastRunResultAnalysisCreated(cloneRow, job.TestRunID)
			}
		}

		covered += len(g.Members)
		if err := w.store.UpdateAnalysisJobProgress(job.ID, i+1, unique, cap, total); err != nil {
			slog.Warn("failure-analysis: progress update failed", "err", err)
		}
		if w.bc != nil {
			current, _ := w.store.GetAnalysisJob(job.ID)
			if current != nil {
				w.bc.BroadcastRunAnalysisProgress(current, covered)
			}
		}
	}

	if err := w.store.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusCompleted, ""); err != nil {
		return err
	}
	if w.bc != nil {
		if final, _ := w.store.GetAnalysisJob(job.ID); final != nil {
			w.bc.BroadcastRunAnalysisCompleted(final, covered)
		}
	}
	return nil
}
