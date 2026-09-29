package ai

import (
	"log/slog"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
)

// followAutoLabel keeps an AI-set label in step with the analysis just stored for this result
// (spec §3.5, R3): the label follows a qualifying decision and resets otherwise. Human labels are
// never touched (the store predicates exclude them). A failed write is logged; the analysis stands.
func (h *Handler) followAutoLabel(result *models.RunResult, res *failureanalysis.AnalyzeResult, deps failureanalysis.JobDeps) {
	ids := []string{result.ID}
	var (
		n   int64
		err error
	)
	if value, ok := deps.AutoApplyTarget(res); ok {
		n, err = h.store.ApplyAutoDefectType(ids, value)
	} else {
		n, err = h.store.ResetAutoDefectType(ids)
	}
	if err != nil {
		slog.Warn("failure-analysis: auto-apply for a single analysis failed", "result_id", result.ID, "err", err)
		return
	}
	if n > 0 {
		h.publishResultRows(result.TestRunID, ids)
	}
}

// followSemanticCloneLabels keeps the semantic clones' AI labels in step with the fits a group
// Explain just computed (spec R10): a clone the explanation fits (narrative_fit ≥ TransferFitMin)
// takes the representative's label when that stored decision qualifies for auto-apply; every
// other semantic clone — fit below the bar, no fit, or auto-apply off or paused — loses an AI
// label. Representatives and signature clones are left alone (their label follows the decision,
// which Explain does not change). Human labels are never touched.
func (h *Handler) followSemanticCloneLabels(rep *models.RunResultAnalysis, changed []*models.RunResultAnalysis,
	deps failureanalysis.JobDeps, runID string) {
	value, ok := deps.AutoApplyTargetRow(rep)
	var apply, reset []string
	for _, a := range changed {
		if a.DedupMethod != models.DedupMethodSemantic {
			continue
		}
		if ok && a.NarrativeFit != nil && *a.NarrativeFit >= failureanalysis.TransferFitMin {
			apply = append(apply, a.RunResultID)
		} else {
			reset = append(reset, a.RunResultID)
		}
	}
	var touched []string
	if len(apply) > 0 {
		if n, err := h.store.ApplyAutoDefectType(apply, value); err != nil {
			slog.Warn("failure-analysis: Explain could not label semantic clones", "analysis_id", rep.ID, "err", err)
		} else if n > 0 {
			touched = append(touched, apply...)
		}
	}
	if len(reset) > 0 {
		if n, err := h.store.ResetAutoDefectType(reset); err != nil {
			slog.Warn("failure-analysis: Explain could not reset semantic clone labels", "analysis_id", rep.ID, "err", err)
		} else if n > 0 {
			touched = append(touched, reset...)
		}
	}
	h.publishResultRows(runID, touched)
}

// publishResultRows republishes results as full rows so the run grid shows (or drops) the AI
// badge live. Best effort, like every broadcast.
func (h *Handler) publishResultRows(runID string, ids []string) {
	if h.broadcaster == nil || len(ids) == 0 {
		return
	}
	run, err := h.store.GetTestRunSummary(runID)
	if err != nil || run == nil {
		return
	}
	rows, err := h.store.GetRunResultsByIDs(runID, ids)
	if err != nil || len(rows) == 0 {
		return
	}
	h.broadcaster.BroadcastRunResultsUpdated(run, rows)
}
