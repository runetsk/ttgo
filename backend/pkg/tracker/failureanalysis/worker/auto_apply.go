package worker

import (
	"log/slog"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
)

// ResultBroadcaster is implemented by broadcasters that can republish result rows. The worker
// uses it, when the broadcaster has it, after auto-apply changes a label, so the run grid shows
// the AI badge (or loses it) live.
type ResultBroadcaster interface {
	BroadcastRunResultsUpdated(run *models.TestRun, rows []*models.RunResult)
}

// publishResults republishes the given results as full rows. Best effort, like every broadcast.
func (w *Worker) publishResults(runID string, ids []string) {
	rb, ok := w.bc.(ResultBroadcaster)
	if !ok || len(ids) == 0 {
		return
	}
	run, err := w.store.GetTestRunSummary(runID)
	if err != nil || run == nil {
		return
	}
	rows, err := w.store.GetRunResultsByIDs(runID, ids)
	if err != nil || len(rows) == 0 {
		return
	}
	rb.BroadcastRunResultsUpdated(run, rows)
}

// autoApplyDecided keeps AI-set labels in step with a group's newly stored decision (spec §3.3,
// §3.5, R3): the representative and its signature clones take the suggested defect type when the
// decision qualifies for this job, otherwise any AI label on them resets. Semantic clones always
// reset here; they are labelled in the narrated phase once the transfer check says the
// explanation fits them. Human labels are never touched — the store predicates exclude any row a
// person decided or labelled.
func (jw *jobWriter) autoApplyDecided(g *failureanalysis.FailureGroup, res *failureanalysis.AnalyzeResult, repID string) {
	var direct, semantic []string
	for _, m := range g.Members {
		if _, ok := g.SemanticMembers[m.ID]; ok {
			semantic = append(semantic, m.ID)
			continue
		}
		direct = append(direct, m.ID)
	}
	var changed []string
	if value, ok := jw.deps.AutoApplyTarget(res); ok {
		if jw.autoValues == nil {
			jw.autoValues = map[string]string{}
		}
		jw.autoValues[repID] = value
		changed = append(changed, jw.applyAuto(direct, value)...)
	} else {
		changed = append(changed, jw.resetAuto(direct)...)
	}
	changed = append(changed, jw.resetAuto(semantic)...)
	jw.w.publishResults(jw.runID, changed)
}

// autoApplyNarrated labels a qualifying group's semantic clones whose stored narrative fit is at
// least TransferFitMin, after the narration (and its transfer check) landed on the rows.
func (jw *jobWriter) autoApplyNarrated(repID string, changed []*models.RunResultAnalysis) {
	value, ok := jw.autoValues[repID]
	if !ok {
		return
	}
	var fit []string
	for _, a := range changed {
		if a.DedupMethod == models.DedupMethodSemantic && a.NarrativeFit != nil && *a.NarrativeFit >= failureanalysis.TransferFitMin {
			fit = append(fit, a.RunResultID)
		}
	}
	jw.w.publishResults(jw.runID, jw.applyAuto(fit, value))
}

// applyAuto labels ids and returns them when any row changed (they are republished whole).
func (jw *jobWriter) applyAuto(ids []string, value string) []string {
	if len(ids) == 0 {
		return nil
	}
	n, err := jw.w.store.ApplyAutoDefectType(ids, value)
	if err != nil {
		slog.Warn("failure-analysis: auto-apply failed", "job_id", jw.jobID, "err", err)
		return nil
	}
	if n == 0 {
		return nil
	}
	if err := jw.w.store.AddAnalysisJobAutoApplied(jw.jobID, n); err != nil {
		slog.Warn("failure-analysis: auto-applied count not recorded", "job_id", jw.jobID, "err", err)
	}
	return ids
}

// resetAuto resets AI labels on ids and returns them when any row changed.
func (jw *jobWriter) resetAuto(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	n, err := jw.w.store.ResetAutoDefectType(ids)
	if err != nil {
		slog.Warn("failure-analysis: AI label reset failed", "job_id", jw.jobID, "err", err)
		return nil
	}
	if n == 0 {
		return nil
	}
	return ids
}
