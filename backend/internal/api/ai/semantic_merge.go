package ai

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"ttgo/internal/api/authctx"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
)

// semanticFailure is one side of an inspected semantic merge.
type semanticFailure struct {
	ResultID     string `json:"result_id"`
	TestName     string `json:"test_name"`
	FailureType  string `json:"failure_type"`
	ErrorMessage string `json:"error_message"`
}

// semanticPairView is the job's decision about the (representative, result) signature pair.
type semanticPairView struct {
	PSame          *float64   `json:"p_same"`
	Source         string     `json:"source"`
	Model          string     `json:"model"`
	AnsweredModel  string     `json:"answered_model"`
	PolicyVersion  string     `json:"policy_version"`
	CreatedAt      time.Time  `json:"created_at"`
	RememberedFrom *time.Time `json:"remembered_from"`
}

// SemanticMergeView is what the inspect endpoint returns (spec Wave 4 §3).
type SemanticMergeView struct {
	Representative     *semanticFailure  `json:"representative"`
	Result             semanticFailure   `json:"result"`
	Pair               *semanticPairView `json:"pair"`
	GroupSize          int               `json:"group_size"`
	SplitGroupSize     int               `json:"split_group_size"`
	OtherGroups        int               `json:"other_groups"`
	CanSplit           bool              `json:"can_split"`
	SplitBlockedReason string            `json:"split_blocked_reason"`
}

// semanticMerge is everything inspect and split derive about one semantic clone.
type semanticMerge struct {
	result      *models.RunResult
	analysis    *models.RunResultAnalysis
	rep         *models.RunResultAnalysis // nil when the representative is gone
	repResult   *models.RunResult
	repSig      string
	sig         string
	clusterSigs []string            // every signature of the merged group, repSig first
	splitOff    []*models.RunResult // latest failing results the split takes out
	groupSize   int                 // results that carry this group's decision (representative included)
	blocked     string              // why a split is not possible now, or ""
}

func failureView(r *models.RunResult) semanticFailure {
	return semanticFailure{ResultID: r.ID, TestName: r.TestNameSnapshot, FailureType: r.FailureType, ErrorMessage: r.ErrorMessage}
}

// loadSemanticMerge resolves the clicked analysis into its merge. It writes the HTTP error and
// returns nil for an unknown result/analysis (404) or a row that is not a semantic clone (409).
func (h *Handler) loadSemanticMerge(w http.ResponseWriter, r *http.Request) *semanticMerge {
	result, err := h.store.GetRunResultByID(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return nil
	}
	if result == nil {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("run result not found"))
		return nil
	}
	a, err := h.store.GetAnalysisByID(r.PathValue("analysisId"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return nil
	}
	if a == nil || a.RunResultID != result.ID {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("analysis not found"))
		return nil
	}
	if a.DedupMethod != models.DedupMethodSemantic || a.SourceAnalysisID == nil || a.JobID == nil || a.DedupGroupKey == nil {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "this analysis was not grouped semantically"})
		return nil
	}
	m := &semanticMerge{result: result, analysis: a, repSig: *a.DedupGroupKey,
		sig: failureanalysis.Signature(result.FailureType, result.ErrorMessage)}
	if m.rep, err = h.store.GetAnalysisByID(*a.SourceAnalysisID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return nil
	}
	if m.rep != nil {
		if m.repResult, err = h.store.GetRunResultByID(m.rep.RunResultID); err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return nil
		}
	}
	if err := h.fillSemanticMerge(m); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return nil
	}
	return m
}

// fillSemanticMerge derives the cluster, the split-off results, the group size and whether a split
// is possible now.
func (h *Handler) fillSemanticMerge(m *semanticMerge) error {
	jobID := *m.analysis.JobID
	// The merged group's signatures: complete linkage merged every cross pair, so the job's merged
	// rows with the representative's signature on one side name every other member.
	pairs, err := h.store.ListSemanticPairsForJob(jobID)
	if err != nil {
		return err
	}
	seen := map[string]bool{m.repSig: true}
	m.clusterSigs = []string{m.repSig}
	for _, p := range pairs {
		if !p.Merged || (p.SigA != m.repSig && p.SigB != m.repSig) {
			continue
		}
		other := p.SigA
		if other == m.repSig {
			other = p.SigB
		}
		if !seen[other] {
			seen[other] = true
			m.clusterSigs = append(m.clusterSigs, other)
		}
	}
	sort.Strings(m.clusterSigs[1:])

	// Results whose current analysis is this group's semantic clone and that share this result's
	// signature — and that are still the latest failing attempts, because only those are analyzed.
	current, err := h.store.GetCurrentAnalysesByRun(m.result.TestRunID)
	if err != nil {
		return err
	}
	var cloneIDs []string
	m.groupSize = 0
	for rid, a := range current {
		if m.rep != nil && (a.ID == m.rep.ID || (a.SourceAnalysisID != nil && *a.SourceAnalysisID == m.rep.ID)) {
			m.groupSize++
		}
		if a.SourceAnalysisID != nil && *a.SourceAnalysisID == *m.analysis.SourceAnalysisID &&
			a.DedupMethod == models.DedupMethodSemantic && a.JobID != nil && *a.JobID == jobID {
			cloneIDs = append(cloneIDs, rid)
		}
	}
	latest, err := h.store.ListLatestFailingResults(m.result.TestRunID)
	if err != nil {
		return err
	}
	isClone := make(map[string]bool, len(cloneIDs))
	for _, id := range cloneIDs {
		isClone[id] = true
	}
	for _, rr := range latest {
		if isClone[rr.ID] && failureanalysis.Signature(rr.FailureType, rr.ErrorMessage) == m.sig {
			m.splitOff = append(m.splitOff, rr)
		}
	}
	sort.Slice(m.splitOff, func(i, j int) bool { return m.splitOff[i].ID < m.splitOff[j].ID })

	cur, err := h.store.GetCurrentAnalysisForResult(m.result.ID)
	if err != nil {
		return err
	}
	switch {
	case m.rep == nil || m.repResult == nil:
		m.blocked = "the group's representative no longer exists; re-analyze instead"
	case cur == nil || cur.ID != m.analysis.ID:
		m.blocked = "a newer analysis of this result exists"
	case !seen[m.sig] || m.sig == m.repSig:
		m.blocked = "this result's failure no longer matches the merge; re-analyze instead"
	case len(m.splitOff) == 0:
		m.blocked = "these results are no longer the latest failing attempts"
	}
	if m.blocked == "" {
		on, err := h.aiEnabled()
		if err != nil {
			return err
		}
		if !on {
			m.blocked = aiOffMessage
		}
	}
	if m.blocked == "" {
		job, err := h.store.GetLatestAnalysisJobForRun(m.result.TestRunID)
		if err != nil {
			return err
		}
		if job != nil && (job.Status == models.RunAnalysisJobStatusQueued || job.Status == models.RunAnalysisJobStatusRunning) {
			m.blocked = "an analysis is running on this run"
		}
	}
	return nil
}

// GetSemanticMerge shows why a result was grouped semantically with its representative.
//
// @Summary      Inspect a semantic merge
// @Description  For a result grouped semantically (dedup_method semantic): the representative's and this result's failure, the job's decision about the pair (p_same, source typesafe = asked in that analysis or memory = reused from an earlier one, with remembered_from, model, policy_version), how many results carry the group's decision (group_size), how many a split would take out (split_group_size, the results sharing this result's error signature that are still the latest failing attempts), how many other error signatures the split keeps it apart from (other_groups), and whether a split is possible now (can_split, split_blocked_reason). 409 when the analysis was not grouped semantically.
// @Tags         ai-failure-analysis
// @Produce      json
// @Param        id          path      string  true  "Run result ID"
// @Param        analysisId  path      string  true  "Analysis ID"
// @Success      200  {object}  SemanticMergeView
// @Failure      404  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}
// @Router       /run-results/{id}/analyses/{analysisId}/semantic [get]
// @Security     BearerAuth
func (h *Handler) GetSemanticMerge(w http.ResponseWriter, r *http.Request) {
	m := h.loadSemanticMerge(w, r)
	if m == nil {
		return
	}
	view := SemanticMergeView{Result: failureView(m.result), GroupSize: m.groupSize, SplitGroupSize: len(m.splitOff),
		OtherGroups: len(m.clusterSigs) - 1, CanSplit: m.blocked == "", SplitBlockedReason: m.blocked}
	if m.repResult != nil {
		rv := failureView(m.repResult)
		view.Representative = &rv
	}
	pair, err := h.store.SemanticPairFor(*m.analysis.JobID, m.repSig, m.sig)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if pair != nil {
		pv := &semanticPairView{PSame: pair.PSame, Source: pair.Source, Model: pair.Model, AnsweredModel: pair.AnsweredModel,
			PolicyVersion: pair.PolicyVersion, CreatedAt: pair.CreatedAt}
		src, err := h.store.RememberedSource(pair)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		if src != nil {
			at := src.CreatedAt
			pv.RememberedFrom = &at
		}
		view.Pair = pv
	} else if m.analysis.DedupPSame != nil {
		// Merged before the pair record existed: the clone row still carries its probability.
		view.Pair = &semanticPairView{PSame: m.analysis.DedupPSame, Source: failureanalysis.SemanticSourceTypeSafe,
			Model: m.analysis.DedupModel, AnsweredModel: m.analysis.DedupModel, PolicyVersion: m.analysis.DedupPolicyVersion,
			CreatedAt: m.analysis.CreatedAt}
	}
	httpx.JSON(w, http.StatusOK, view)
}

// SplitSemanticMerge takes a wrongly merged failure out of its semantic group and analyzes it on
// its own (spec Wave 4 §4).
//
// @Summary      Split a result out of a semantic merge
// @Description  Takes every result that shares this result's error signature out of its semantic group (those that are still the latest failing attempts) and queues an analysis limited to them (the job's scope_result_ids, split_from_analysis_id). The person's judgement is recorded as a permanent "different" between this signature and every other signature of the merged group, so later analyses never merge them again. The rest of the group keeps its decision. 409 when the analysis was not grouped semantically, when a split is not possible now (split_blocked_reason of GET …/semantic: AI switched off, an analysis already running, a newer analysis of the result, the representative gone, nothing left to split), or when the estimated cost exceeds a soft AI budget and acknowledge_budget is not true.
// @Tags         ai-failure-analysis
// @Produce      json
// @Param        id                  path   string  true   "Run result ID"
// @Param        analysisId          path   string  true   "Analysis ID"
// @Param        acknowledge_budget  query  bool    false  "Proceed although a soft AI budget would be exceeded"
// @Success      201  {object}  models.RunAnalysisJob
// @Failure      404  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}
// @Router       /run-results/{id}/analyses/{analysisId}/split [post]
// @Security     BearerAuth
func (h *Handler) SplitSemanticMerge(w http.ResponseWriter, r *http.Request) {
	m := h.loadSemanticMerge(w, r)
	if m == nil {
		return
	}
	if m.blocked != "" {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": m.blocked})
		return
	}
	settings, err := h.store.GetFailureAnalysisSettings()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	groups := failureanalysis.PlannedGroups(m.splitOff, settings.DedupEnabled, settings.MaxAnalysesPerRun)
	if warn := h.jobBudgetWarning(r, groups); warn != nil {
		httpx.JSON(w, http.StatusConflict, warn)
		return
	}
	jobID := *m.analysis.JobID
	var pins []*models.SemanticPair
	for _, other := range m.clusterSigs {
		if other == m.sig {
			continue
		}
		pin := &models.SemanticPair{JobID: jobID, RunID: m.result.TestRunID, SigA: m.sig, SigB: other,
			ResultAID: m.result.ID, Source: failureanalysis.SemanticSourceHuman, PolicyVersion: failureanalysis.SemanticPolicyVersion}
		if other == m.repSig {
			pin.ResultBID = m.repResult.ID
		}
		pins = append(pins, pin)
	}
	ids := make([]string, len(m.splitOff))
	for i, rr := range m.splitOff {
		ids[i] = rr.ID
	}
	job, created, err := h.store.EnqueueScopedAnalysis(m.result.TestRunID, authctx.ActorID(r.Context()), ids, m.analysis.ID, pins)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !created {
		httpx.JSON(w, http.StatusConflict, map[string]interface{}{"error": "analysis already running", "job": job})
		return
	}
	httpx.JSON(w, http.StatusCreated, job)
}
