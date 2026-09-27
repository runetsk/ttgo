package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
	"ttgo/pkg/tracker/callstats"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
)

// GroupDeadline bounds one group's whole analysis: the TypeSafe call, the LLM call with its
// transient retry and JSON repair, and the explanation. A group that runs past it is recorded
// as a failed attempt (category "timeout") and the job moves on.
var GroupDeadline = 5 * time.Minute

// cancelPoll is how often a running job checks whether it was cancelled, so an in-flight
// provider request is abandoned instead of finishing first.
var cancelPoll = time.Second

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

// recordCosts appends cost events to the ledger. A failed write is logged, never fatal: the
// analysis it describes is already stored.
func (w *Worker) recordCosts(events []*models.AIAnalysisCostEvent) {
	for _, ev := range events {
		if err := w.store.RecordAnalysisCostEvent(ev); err != nil {
			slog.Warn("failure-analysis: cost event not recorded", "kind", ev.Kind, "engine", ev.Engine, "err", err)
		}
	}
}

func (w *Worker) isCancelled(jobID string) bool {
	cur, err := w.store.GetAnalysisJob(jobID)
	return err == nil && cur != nil && cur.Status == models.RunAnalysisJobStatusCancelled
}

// watchCancel cancels the job context as soon as the job is marked cancelled, so a provider
// request in flight is abandoned. It returns when ctx ends.
func (w *Worker) watchCancel(ctx context.Context, cancel context.CancelFunc, jobID string) {
	t := time.NewTicker(cancelPoll)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if w.isCancelled(jobID) {
				cancel()
				return
			}
		}
	}
}

// groupOutcome is one group's finished analysis, handed from its goroutine to the writer.
type groupOutcome struct {
	group *failureanalysis.FailureGroup
	res   *failureanalysis.AnalyzeResult
	err   error
}

// parallelGroups bounds the stored setting; a row from before the setting reads as 0.
func parallelGroups(n int) int {
	switch {
	case n < 1:
		return 1
	case n > models.MaxParallelGroups:
		return models.MaxParallelGroups
	}
	return n
}

// analyzeGroups runs analyze for up to parallel groups at once, each under its own
// GroupDeadline, and delivers every outcome on the returned channel, which is closed once
// all started groups have finished. No group starts after the job is cancelled: the
// feeder checks the job context and the stored status before each one. The channel is
// buffered for every group, so a writer that stops reading early never blocks a sender.
func (w *Worker) analyzeGroups(jobCtx context.Context, cancelJob context.CancelFunc, jobID string,
	groups []*failureanalysis.FailureGroup, parallel int,
	analyze func(context.Context, *failureanalysis.FailureGroup) (*failureanalysis.AnalyzeResult, error),
) <-chan groupOutcome {
	out := make(chan groupOutcome, len(groups))
	go func() {
		defer close(out)
		slots := make(chan struct{}, parallel)
		var wg sync.WaitGroup
		defer wg.Wait()
		for _, g := range groups {
			select {
			case slots <- struct{}{}:
			case <-jobCtx.Done():
				return
			}
			if jobCtx.Err() != nil || w.isCancelled(jobID) {
				cancelJob()
				return
			}
			wg.Add(1)
			go func(g *failureanalysis.FailureGroup) {
				defer wg.Done()
				defer func() { <-slots }()
				gctx, cancel := context.WithTimeout(jobCtx, GroupDeadline)
				defer cancel()
				res, err := analyze(gctx, g)
				out <- groupOutcome{group: g, res: res, err: err}
			}(g)
		}
	}()
	return out
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
	if errors.Is(err, failureanalysis.ErrAIDisabled) {
		w.failJob(job.ID, "AI features are switched off")
		return nil
	}
	if err != nil {
		w.failJob(job.ID, "resolve dependencies: "+err.Error())
		return err
	}
	if !deps.CanAnalyze() {
		msg := "no LLM provider configured"
		if deps.LLMUnavailableReason != "" {
			msg = "cannot analyze: " + deps.LLMUnavailableReason
		}
		w.failJob(job.ID, msg)
		return nil
	}
	pipeline := deps.Pipeline()
	if b, err := json.Marshal(pipeline); err == nil {
		if err := w.store.SetAnalysisJobPipeline(job.ID, string(b), pipeline.Label()); err != nil {
			slog.Warn("failure-analysis: pipeline record failed", "err", err)
		}
	}

	jobCtx, cancelJob := context.WithCancel(ctx)
	defer cancelJob()
	// Every TypeSafe and LLM call of the job reports its 429s here (semantic pass included).
	jobCtx, calls := callstats.WithCounter(jobCtx)
	saveRateLimits := func() {
		if err := w.store.SetAnalysisJobRateLimitHits(job.ID, calls.RateLimitHits()); err != nil {
			slog.Warn("failure-analysis: rate-limit count not recorded", "job_id", job.ID, "err", err)
		}
	}
	defer saveRateLimits() // also for cancelled and interrupted jobs
	go w.watchCancel(jobCtx, cancelJob, job.ID)
	cancelled := func() bool { return jobCtx.Err() != nil && ctx.Err() == nil }

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
		merged, rep, serr := failureanalysis.MergeGroupsSemantically(jobCtx, sd, groups, func() bool { return w.isCancelled(job.ID) })
		semanticReport = rep
		// The pass was billed whether or not its merges are used.
		w.recordCosts(failureanalysis.SemanticCostEvents(rep, deps, failureanalysis.CostRefs{RunID: job.TestRunID, JobID: &job.ID}))
		switch {
		case errors.Is(serr, failureanalysis.ErrCancelled) || cancelled():
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

	if job.RetryFailedOnly {
		failedIDs, err := w.store.FailedResultIDsForRun(job.TestRunID)
		if err != nil {
			w.failJob(job.ID, "load failed analyses: "+err.Error())
			return err
		}
		kept := groups[:0]
		for _, g := range groups {
			for _, m := range g.Members {
				if failedIDs[m.ID] {
					kept = append(kept, g)
					break
				}
			}
		}
		groups = kept
	}
	unique := len(groups)

	cap := settings.MaxAnalysesPerRun
	if unique < cap {
		cap = unique
	}
	groups = groups[:cap]

	// Groups are analyzed ParallelGroups at a time: the TypeSafe and LLM calls, which are
	// nearly all of a job's time, overlap. Results are written here, on this goroutine only,
	// in the order they finish, so the store sees one writer as before.
	analyzeDeps := deps.Analyze()
	outcomes := w.analyzeGroups(jobCtx, cancelJob, job.ID, groups, parallelGroups(settings.ParallelGroups),
		func(gctx context.Context, g *failureanalysis.FailureGroup) (*failureanalysis.AnalyzeResult, error) {
			actx := failureanalysis.BuildContext(w.store, g.Representative, time.Now())
			actx.RedactionEnabled = settings.RedactionEnabled
			actx.PromptTemplate = settings.PromptTemplate
			actx.ProviderModel = deps.NarrativeModel
			return failureanalysis.Analyze(gctx, analyzeDeps, actx)
		})

	jobID := job.ID
	covered, done := 0, 0
	for out := range outcomes {
		g, res, err := out.group, out.res, out.err
		rep := g.Representative
		if err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil && cancelled() {
			continue // abandoned by the cancel; a result that came back before it is still stored
		}
		if err != nil {
			slog.Warn("failure-analysis: analysis attempt failed", "err", err, "result_id", rep.ID)
			res = failureanalysis.FailedResult(err, analyzeDeps)
		}

		repRowIn := failureanalysis.AnalysisRowFrom(res, rep.ID)
		repRowIn.JobID = &jobID
		repRow, err := w.store.CreateAnalysis(repRowIn)
		// Clones below copy this answer and make no call, so only the representative bills.
		w.recordCosts(failureanalysis.CostEvents(models.AnalysisCostKindAnalysis, res, deps,
			failureanalysis.RefsFor(job.TestRunID, &jobID, repRow)))
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
			clone.JobID = &jobID
			clone.RawResponse = ""
			clone.TypeSafeInputTokens = 0
			clone.TokenUsagePrompt, clone.TokenUsageCompletion = 0, 0
			clone.DecisionMs, clone.LLMMs, clone.LLMCalls, clone.FinishReason = 0, 0, 0, ""
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
		done++
		if err := w.store.UpdateAnalysisJobProgress(job.ID, done, unique, cap, total); err != nil {
			slog.Warn("failure-analysis: progress update failed", "err", err)
		}
		if w.bc != nil {
			if current, _ := w.store.GetAnalysisJob(job.ID); current != nil {
				w.bc.BroadcastRunAnalysisProgress(current, covered)
			}
		}
	}
	if ctx.Err() != nil {
		// The server is stopping: groups may never have started, so the job is not complete.
		// It stays running, and the restart sweep marks it failed on the next start.
		return ctx.Err()
	}
	if cancelled() {
		slog.Info("failure-analysis: cancelled mid-job", "job_id", job.ID, "groups_stored", done)
		return nil
	}

	saveRateLimits() // before completion, so the completed job already carries it
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
