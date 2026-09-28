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

// GroupDeadline is the floor of a group's deadline. Each job derives its own bound from its
// timeouts (failureanalysis.GroupDeadlineFor: the TypeSafe call, one LLM stage with its retries
// and JSON repair, backoffs); a group that runs past it is recorded as a failed attempt
// (category "timeout") and the job moves on. Lowering GroupDeadline below
// failureanalysis.MinGroupDeadline (tests) caps every group at that value instead.
var GroupDeadline = failureanalysis.MinGroupDeadline

// jobGroupDeadline is the bound for every group of a job resolved with deps.
func jobGroupDeadline(deps failureanalysis.JobDeps) time.Duration {
	if GroupDeadline < failureanalysis.MinGroupDeadline {
		return GroupDeadline
	}
	return failureanalysis.GroupDeadlineFor(deps.TypeSafeTimeout, deps.LLMCallTimeout)
}

// cancelPoll is how often a running job checks whether it was cancelled, so an in-flight
// provider request is abandoned instead of finishing first.
var cancelPoll = time.Second

// Broadcaster is the narrow subset of the ws.Hub the worker needs.
type Broadcaster interface {
	BroadcastRunAnalysisProgress(job *models.RunAnalysisJob, coveredFailures int)
	BroadcastRunAnalysisCompleted(job *models.RunAnalysisJob, coveredFailures int)
	BroadcastRunResultAnalysisCreated(a *models.RunResultAnalysis, testRunID string)
	BroadcastRunResultAnalysisUpdated(a *models.RunResultAnalysis, testRunID string)
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
	// Single process (spec deployment assumption): nothing can be narrating at startup, so every
	// pending explanation — a job's or an interrupted Explain's — will never be written.
	if n, err := w.store.SweepPendingNarratives(""); err != nil {
		slog.Warn("failure-analysis: restart sweep of pending explanations failed", "err", err)
	} else if n > 0 {
		slog.Info("failure-analysis: restart sweep settled pending explanations", "count", n)
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

// settlePending marks the job's explanations that will never be written unavailable and
// republishes those rows, so no view keeps showing "Explanation being written…". Runs whenever
// a claimed job ends — completed, cancelled, failed or stopped.
func (w *Worker) settlePending(jobID, runID string) {
	ids, err := w.store.PendingNarrativeIDs(jobID)
	if err != nil {
		slog.Warn("failure-analysis: pending explanations not listed", "job_id", jobID, "err", err)
		return
	}
	if len(ids) == 0 {
		return
	}
	n, err := w.store.SweepPendingNarratives(jobID)
	if err != nil {
		slog.Warn("failure-analysis: pending explanations not settled", "job_id", jobID, "err", err)
		return
	}
	slog.Info("failure-analysis: settled explanations that were never written", "job_id", jobID, "count", n)
	if w.bc == nil {
		return
	}
	for _, id := range ids {
		if a, _ := w.store.GetAnalysisByID(id); a != nil && a.NarrativeStatus != models.NarrativeStatusPending {
			w.bc.BroadcastRunResultAnalysisUpdated(a, runID)
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

// groupPhase tags what a group goroutine hands the writer.
type groupPhase string

const (
	phaseDecided  groupPhase = "decided"  // the decision (or a failed attempt), to be stored
	phaseNarrated groupPhase = "narrated" // the explanation of a stored pending decision
)

// decidedAck is the writer's one answer to a decided outcome: the stored representative's id,
// or why it could not be stored (the group then does not narrate).
type decidedAck struct {
	repID string
	err   error
}

// groupOutcome is what a group goroutine hands the single writer.
type groupOutcome struct {
	phase groupPhase
	group *failureanalysis.FailureGroup

	// decided
	res *failureanalysis.AnalyzeResult
	err error
	ack chan decidedAck // buffered (1); the writer answers every decided outcome exactly once

	// narrated
	repAnalysisID string
	delta         failureanalysis.NarrationDelta
}

// groupFuncs are the two phases of one group's analysis.
type groupFuncs struct {
	decide  func(context.Context, *failureanalysis.FailureGroup) (*failureanalysis.AnalyzeResult, failureanalysis.AnalyzeContext, error)
	narrate func(context.Context, failureanalysis.AnalyzeContext, *failureanalysis.AnalyzeResult) (failureanalysis.NarrationDelta, bool)
}

// errAbandoned acks a decided outcome the writer dropped because the job was cancelled.
var errAbandoned = errors.New("abandoned: the job was cancelled")

// cancelledNarrationReason is why a pending explanation was given up between or during phases.
const cancelledNarrationReason = "the analysis was cancelled or timed out before the explanation was written"

// narrationCancelled is the internal NarrationDelta.Reason P2's Narrate sets when its context
// was cancelled mid-call (Analyze turns it into ctx.Err()). If P2 exported a constant for it,
// use that instead of this literal.
const narrationCancelled = "cancelled"

// cancelledDelta settles a pending group whose narration was cut off, keeping what any call
// it made cost (it was billed).
func cancelledDelta(spent failureanalysis.NarrationDelta) failureanalysis.NarrationDelta {
	return failureanalysis.NarrationDelta{
		NarrativeStatus: models.NarrativeStatusUnavailable, Reason: cancelledNarrationReason,
		Summary:      "AI narrative unavailable: " + cancelledNarrationReason,
		PromptTokens: spent.PromptTokens, CompletionTokens: spent.CompletionTokens,
		LLMMs: spent.LLMMs, LLMCalls: spent.LLMCalls,
	}
}

// analyzeGroups runs up to parallel groups at once, each under its own deadline, in two phases:
// decide, hand the decision to the writer, and — when it is pending an explanation and the
// writer acked the stored representative — narrate and hand the explanation over too. No group
// starts after the job is cancelled. The channel holds two outcomes per group and every send
// also watches the job context, so no goroutine blocks after the writer stops; it is closed
// once every started group has ended.
func (w *Worker) analyzeGroups(jobCtx context.Context, cancelJob context.CancelFunc, jobID string,
	groups []*failureanalysis.FailureGroup, parallel int, deadline time.Duration, fns groupFuncs,
) <-chan groupOutcome {
	out := make(chan groupOutcome, 2*len(groups))
	send := func(o groupOutcome) bool {
		select { // room in the buffer: deliver even after a cancel, so a finished result is kept
		case out <- o:
			return true
		default:
		}
		select {
		case out <- o:
			return true
		case <-jobCtx.Done():
			return false
		}
	}
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
				gctx, cancel := context.WithTimeout(jobCtx, deadline)
				defer cancel()

				res, actx, err := fns.decide(gctx, g)
				ack := make(chan decidedAck, 1)
				if !send(groupOutcome{phase: phaseDecided, group: g, res: res, err: err, ack: ack}) {
					return
				}
				if err != nil || res == nil || res.NarrativeStatus != models.NarrativeStatusPending {
					return
				}
				var reply decidedAck
				select {
				case reply = <-ack:
				case <-gctx.Done():
					return // the writer stopped or the group ran out of time; the job-end sweep settles the rows
				}
				if reply.err != nil {
					return // the representative was not stored: nothing to explain
				}
				var delta failureanalysis.NarrationDelta
				if gctx.Err() == nil {
					d, ok := fns.narrate(gctx, actx, res)
					if !ok {
						return
					}
					delta = d
				}
				// Narrate reports a cut-off call with the internal Reason "cancelled"; it (and any
				// non-ok delta once the group context ended) becomes a readable unavailable delta,
				// applied now rather than left for the sweep, so live views settle immediately.
				if delta.Reason == narrationCancelled || (gctx.Err() != nil && delta.NarrativeStatus != models.NarrativeStatusOK) {
					delta = cancelledDelta(delta)
				}
				send(groupOutcome{phase: phaseNarrated, group: g, repAnalysisID: reply.repID, delta: delta})
			}(g)
		}
	}()
	return out
}

// jobWriter stores one job's outcomes. Only processOnce's goroutine uses it, so the store sees
// a single writer.
type jobWriter struct {
	w        *Worker
	deps     failureanalysis.JobDeps
	runID    string
	jobID    string
	semantic failureanalysis.SemanticReport
}

// writeDecided stores a group's decision — the representative, then one clone per other
// member, all with the decision's narrative status (pending while the explanation is written) —
// records the decision's cost events and publishes each row. It returns the representative's
// id; an error means it could not be stored and the group must not narrate.
func (jw *jobWriter) writeDecided(g *failureanalysis.FailureGroup, res *failureanalysis.AnalyzeResult) (string, error) {
	rep := g.Representative
	jobID := jw.jobID
	repRowIn := failureanalysis.AnalysisRowFrom(res, rep.ID)
	repRowIn.JobID = &jobID
	repRow, err := jw.w.store.CreateAnalysis(repRowIn)
	// Clones below copy this answer and make no call, so only the representative bills.
	jw.w.recordCosts(failureanalysis.DecisionCostEvents(res, jw.deps, failureanalysis.RefsFor(jw.runID, &jobID, repRow)))
	if err != nil {
		slog.Warn("failure-analysis: persist representative failed", "err", err, "result_id", rep.ID)
		return "", err
	}
	if jw.w.bc != nil {
		jw.w.bc.BroadcastRunResultAnalysisCreated(repRow, jw.runID)
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
			clone.DedupModel, clone.DedupPolicyVersion = jw.semantic.Model, jw.semantic.PolicyVersion
			clone.Rationale = "[Grouped semantically with representative analysis] " + res.Rationale
		} else {
			clone.DedupMethod = models.DedupMethodSignature
			clone.Rationale = "[Grouped from representative analysis] " + res.Rationale
		}
		cloneRow, err := jw.w.store.CreateAnalysis(clone)
		if err != nil {
			slog.Warn("failure-analysis: persist clone failed", "err", err) // a clone never fails the ack
			continue
		}
		if jw.w.bc != nil {
			jw.w.bc.BroadcastRunResultAnalysisCreated(cloneRow, jw.runID)
		}
	}
	return repRow.ID, nil
}

// writeNarrated applies a group's explanation to the representative and its clones in one
// transaction and republishes every changed row. The call's cost is recorded whether or not
// the apply wins: it was billed.
func (jw *jobWriter) writeNarrated(repID string, d failureanalysis.NarrationDelta) {
	jobID := jw.jobID
	jw.w.recordCosts(failureanalysis.NarrationCostEvents(d, jw.deps,
		failureanalysis.CostRefs{RunID: jw.runID, JobID: &jobID, AnalysisID: &repID}, models.AnalysisCostKindAnalysis))
	changed, err := jw.w.store.ApplyNarration(repID, d)
	if err != nil {
		slog.Warn("failure-analysis: explanation not stored", "analysis_id", repID, "err", err)
		return
	}
	if len(changed) == 0 {
		slog.Info("failure-analysis: explanation arrived after its group was settled; not applied", "analysis_id", repID)
		return
	}
	if jw.w.bc != nil {
		for _, a := range changed {
			jw.w.bc.BroadcastRunResultAnalysisUpdated(a, jw.runID)
		}
	}
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
	defer w.settlePending(job.ID, job.TestRunID) // every exit after the claim: cancel, failure, stop

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
	// Every TypeSafe and LLM call of the job reports its 429s, call timeouts and hedges here
	// (semantic pass included).
	jobCtx, calls := callstats.WithCounter(jobCtx)
	saveCallStats := func() {
		if err := w.store.SetAnalysisJobCallStats(job.ID, calls.RateLimitHits(), calls.CallTimeouts(),
			calls.HedgesFired(), calls.HedgesWon()); err != nil {
			slog.Warn("failure-analysis: call stats not recorded", "job_id", job.ID, "err", err)
		}
	}
	defer saveCallStats() // also for cancelled and interrupted jobs
	// Fired hedges are billed once per job, when it ends however it ends: the counter is
	// shared by the job's parallel groups, so they are tied to the job, not to one analysis.
	defer func() {
		w.recordCosts(failureanalysis.HedgeCostEvents(calls.HedgePromptTokens(), deps,
			failureanalysis.CostRefs{RunID: job.TestRunID, JobID: &job.ID}))
	}()
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

	// Groups are analyzed ParallelGroups at a time in two phases: the decision is stored and
	// published as soon as it exists, and a TypeSafe decision's explanation lands on the same
	// rows afterwards. Outcomes are written here, on this goroutine only, so the store sees one
	// writer as before.
	analyzeDeps := deps.Analyze()
	jobID := job.ID
	outcomes := w.analyzeGroups(jobCtx, cancelJob, jobID, groups, parallelGroups(settings.ParallelGroups), jobGroupDeadline(deps), groupFuncs{
		decide: func(gctx context.Context, g *failureanalysis.FailureGroup) (*failureanalysis.AnalyzeResult, failureanalysis.AnalyzeContext, error) {
			actx := failureanalysis.BuildContext(w.store, g.Representative, time.Now(), deps.FewShotExamples)
			actx.RedactionEnabled = settings.RedactionEnabled
			actx.PromptTemplate = settings.PromptTemplate
			actx.ProviderModel = deps.NarrativeModel
			actx.GroupMembers = failureanalysis.GroupMemberErrors(g.Representative, g.Members)
			res, err := failureanalysis.Decide(gctx, analyzeDeps, actx)
			if err != nil && res == nil {
				res = failureanalysis.FailedResult(err, analyzeDeps, actx)
			}
			return res, actx, err
		},
		narrate: func(gctx context.Context, actx failureanalysis.AnalyzeContext, decided *failureanalysis.AnalyzeResult) (failureanalysis.NarrationDelta, bool) {
			return failureanalysis.Narrate(gctx, analyzeDeps, actx, decided)
		},
	})

	jw := &jobWriter{w: w, deps: deps, runID: job.TestRunID, jobID: jobID, semantic: semanticReport}
	covered, done := 0, 0
	for out := range outcomes {
		if out.phase == phaseNarrated {
			jw.writeNarrated(out.repAnalysisID, out.delta)
			continue
		}
		g := out.group
		if out.err != nil && ctx.Err() != nil {
			out.ack <- decidedAck{err: ctx.Err()}
			return ctx.Err()
		}
		if out.err != nil && cancelled() {
			out.ack <- decidedAck{err: errAbandoned}
			continue // abandoned by the cancel; a result that came back before it is still stored
		}
		if out.err != nil {
			slog.Warn("failure-analysis: analysis attempt failed", "err", out.err, "result_id", g.Representative.ID)
		}
		repID, err := jw.writeDecided(g, out.res)
		out.ack <- decidedAck{repID: repID, err: err}
		if err != nil {
			continue
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

	saveCallStats()                        // before completion, so the completed job already carries it
	w.settlePending(job.ID, job.TestRunID) // before completion, so the completed job has none in progress
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
