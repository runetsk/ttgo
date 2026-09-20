package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
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
	resolve  failureanalysis.DepsResolver
	bc       Broadcaster
	interval time.Duration
}

// NewWorker builds a worker. resolve is called once per job so admin changes apply without a restart.
func NewWorker(s *store.Store, resolve failureanalysis.DepsResolver, bc Broadcaster, interval time.Duration) *Worker {
	return &Worker{store: s, resolve: resolve, bc: bc, interval: interval}
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

// ProcessOnceForTest is an exported alias of processOnce for tests in other packages.
func (w *Worker) ProcessOnceForTest(ctx context.Context) error { return w.processOnce(ctx) }

func (w *Worker) failJob(id, msg string) {
	if _, err := w.store.UpdateAnalysisJobStatus(id, models.RunAnalysisJobStatusFailed, msg); err != nil {
		slog.Error("failure-analysis: could not mark job failed", "job_id", id, "error", err)
	}
}

func (w *Worker) isCancelled(jobID string) bool {
	cur, err := w.store.GetAnalysisJob(jobID)
	return err == nil && cur != nil && cur.Status == models.RunAnalysisJobStatusCancelled
}

// processOnce picks up at most one queued job and runs it to completion.
func (w *Worker) processOnce(ctx context.Context) error {
	job, err := w.store.NextQueuedAnalysisJob()
	if err != nil || job == nil {
		return err
	}
	claimed, err := w.store.MarkAnalysisJobRunning(job.ID)
	if err != nil {
		return fmt.Errorf("mark running: %w", err)
	}
	if !claimed {
		return nil // cancelled between pick-up and claim; the cancel wins
	}

	deps, err := w.resolve(job.Trigger)
	if err != nil {
		w.failJob(job.ID, "resolve dependencies: "+err.Error())
		return err
	}
	if deps.Narrative == nil {
		w.failJob(job.ID, "no LLM provider configured")
		return nil
	}

	settings, err := w.store.GetFailureAnalysisSettings()
	if err != nil {
		w.failJob(job.ID, "load settings: "+err.Error())
		return err
	}
	failures, err := w.store.ListLatestFailingResults(job.TestRunID)
	if err != nil {
		w.failJob(job.ID, "load failures: "+err.Error())
		return err
	}
	total := len(failures)

	var groups []*failureanalysis.FailureGroup
	if settings.DedupEnabled {
		groups = failureanalysis.GroupFailures(failures)
	} else {
		for _, r := range failures {
			groups = append(groups, &failureanalysis.FailureGroup{
				Key: failureanalysis.Signature(r.FailureType, r.ErrorMessage), Representative: r, Members: []*models.RunResult{r},
			})
		}
	}

	semanticReport := failureanalysis.SemanticReport{}
	if settings.DedupEnabled && deps.Semantic != nil {
		sd := *deps.Semantic
		sd.Redact = settings.RedactionEnabled
		merged, rep, serr := failureanalysis.MergeGroupsSemantically(ctx, sd, groups, func() bool { return w.isCancelled(job.ID) })
		semanticReport = rep
		switch {
		case errors.Is(serr, failureanalysis.ErrCancelled):
			slog.Info("failure-analysis: cancelled during semantic grouping", "job_id", job.ID)
			return nil
		case serr != nil && ctx.Err() != nil:
			return ctx.Err()
		case serr != nil:
			slog.Warn("failure-analysis: semantic grouping failed, using signature groups", "job_id", job.ID, "err", serr)
		default:
			groups = merged
			slog.Info("failure-analysis: semantic grouping", "job_id", job.ID, "blocks", rep.Blocks, "candidates", rep.Candidates,
				"asked", rep.Asked, "requests", rep.Requests, "merged", rep.Merged, "skipped", rep.Skipped, "tokens", rep.InputTokens)
		}
		if err := w.store.SetAnalysisJobSemanticTokens(job.ID, rep.InputTokens); err != nil {
			slog.Warn("failure-analysis: semantic token update failed", "err", err)
		}
	}
	unique := len(groups)

	cap := settings.MaxAnalysesPerRun
	if unique < cap {
		cap = unique
	}
	groups = groups[:cap]

	covered := 0
	for i, g := range groups {
		if w.isCancelled(job.ID) {
			slog.Info("failure-analysis: cancelled mid-job", "job_id", job.ID, "after_group", i)
			return nil
		}
		rep := g.Representative
		actx := failureanalysis.BuildContext(w.store, rep, time.Now())
		actx.RedactionEnabled = settings.RedactionEnabled
		actx.PromptTemplate = settings.PromptTemplate
		actx.ProviderModel = deps.NarrativeModel
		res, err := failureanalysis.Analyze(ctx, deps.Analyze(), actx)
		if err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			slog.Warn("failure-analysis: analyze failed — recording unknown verdict", "err", err, "result_id", rep.ID)
			res = &failureanalysis.AnalyzeResult{
				Engine: models.AnalysisEngineGenerative, NarrativeStatus: models.NarrativeStatusUnavailable,
				Verdict: models.VerdictUnknown, Confidence: models.ConfidenceLow, Summary: "analysis failed: " + err.Error(),
			}
		}

		repRow, err := w.store.CreateAnalysis(failureanalysis.AnalysisRowFrom(res, rep.ID))
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
			groupKey, sourceID := g.Key, repRow.ID
			clone := failureanalysis.AnalysisRowFrom(res, sib.ID)
			clone.RawResponse = ""
			clone.TypeSafeInputTokens = 0
			clone.TokenUsagePrompt, clone.TokenUsageCompletion = 0, 0
			clone.DedupGroupKey, clone.SourceAnalysisID = &groupKey, &sourceID
			if p, ok := g.SemanticMembers[sib.ID]; ok {
				pp := p
				clone.DedupMethod, clone.DedupPSame = models.DedupMethodSemantic, &pp
				clone.DedupModel, clone.DedupPolicyVersion = semanticReport.Model, semanticReport.PolicyVersion
				clone.Rationale = "[Grouped semantically with representative analysis] " + res.Rationale
			} else {
				clone.DedupMethod = models.DedupMethodSignature
				clone.Rationale = "[Grouped from representative analysis] " + res.Rationale
			}
			cloneRow, err := w.store.CreateAnalysis(clone)
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
			if current, _ := w.store.GetAnalysisJob(job.ID); current != nil {
				w.bc.BroadcastRunAnalysisProgress(current, covered)
			}
		}
	}

	changed, err := w.store.UpdateAnalysisJobStatus(job.ID, models.RunAnalysisJobStatusCompleted, "")
	if err != nil {
		return err
	}
	if !changed {
		slog.Info("failure-analysis: job was cancelled before completion", "job_id", job.ID)
		return nil
	}
	if w.bc != nil {
		if final, _ := w.store.GetAnalysisJob(job.ID); final != nil {
			w.bc.BroadcastRunAnalysisCompleted(final, covered)
		}
	}
	return nil
}
