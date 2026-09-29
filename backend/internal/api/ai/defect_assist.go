package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/aigen"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"
)

// DefectAssistRequest is a defect being written: its title and description, and what it was found
// from (optional).
type DefectAssistRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	TestCaseID   string `json:"test_case_id"`
	RunResultID  string `json:"run_result_id"`
	ErrorMessage string `json:"error_message"`
}

// DefectSeveritySuggestion is TypeSafe's severity answer.
type DefectSeveritySuggestion struct {
	Value         string             `json:"value"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}

// DefectDuplicate is an open defect that may describe the same problem.
type DefectDuplicate struct {
	DefectID    string  `json:"defect_id"`
	Title       string  `json:"title"`
	Status      string  `json:"status"`
	Severity    string  `json:"severity"`
	ExternalKey string  `json:"external_key"`
	PSame       float64 `json:"p_same"`
}

// DefectAssistResponse is what defect assist returns.
type DefectAssistResponse struct {
	Severity   *DefectSeveritySuggestion `json:"severity"`
	Duplicates []DefectDuplicate         `json:"duplicates"`
}

// selectDefectCandidates picks the open defects a new one is compared with (spec Wave 5 §3): the
// test case's own open defects first, then open defects whose titles share words with the new title
// (token Jaccard >= failureanalysis.DefectDupTitleFloor, most similar first), each once, at most
// failureanalysis.DefectDupCandidates.
func selectDefectCandidates(title string, sameTest, all []models.Defect) []models.Defect {
	seen := map[string]bool{}
	var out []models.Defect
	add := func(d models.Defect) {
		if d.Status == "closed" || seen[d.ID] || len(out) >= failureanalysis.DefectDupCandidates {
			return
		}
		seen[d.ID] = true
		out = append(out, d)
	}
	for _, d := range sameTest {
		add(d)
	}
	type scored struct {
		d   models.Defect
		sim float64
	}
	var sims []scored
	for _, d := range all {
		if s := aigen.TokenSimilarity(title, d.Title); s >= failureanalysis.DefectDupTitleFloor {
			sims = append(sims, scored{d, s})
		}
	}
	sort.SliceStable(sims, func(i, j int) bool { return sims[i].sim > sims[j].sim })
	for _, s := range sims {
		add(s.d)
	}
	return out
}

// AssistDefect suggests a new defect's severity and finds open defects it may duplicate.
//
// @Summary      Defect assist (TypeSafe.ai)
// @Description  For a defect being written: TypeSafe.ai suggests its severity (critical, major, minor, trivial, with probabilities) and compares it with up to 10 open defects — the test case's own, then those whose titles share words — returning the ones at least 70% likely to describe the same problem, most likely first. Nothing is saved. 409 when defect assist is unavailable (AI or TypeSafe.ai off, no key, or its switch off), 400 without a title.
// @Tags         defects
// @Accept       json
// @Produce      json
// @Param        body  body  DefectAssistRequest  true  "The defect being written"
// @Success      200  {object}  DefectAssistResponse
// @Failure      400  {object}  map[string]interface{}
// @Failure      409  {object}  map[string]interface{}
// @Failure      502  {object}  map[string]interface{}
// @Router       /defects/assist [post]
// @Security     BearerAuth
func (h *Handler) AssistDefect(w http.ResponseWriter, r *http.Request) {
	var req DefectAssistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}
	use, why := h.TypeSafeFor(TypeSafeUseDefectAssist)
	if use == nil {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": why})
		return
	}
	testName, errMsg := "", req.ErrorMessage
	if req.RunResultID != "" {
		if rr, err := h.store.GetRunResultByID(req.RunResultID); err == nil && rr != nil {
			testName = rr.TestNameSnapshot
			if errMsg == "" {
				errMsg = rr.ErrorMessage
			}
			if req.TestCaseID == "" && rr.TestCaseID != nil {
				req.TestCaseID = *rr.TestCaseID
			}
		}
	}
	var sameTest []models.Defect
	if req.TestCaseID != "" {
		sameTest, _ = h.store.ListDefectsByTestCase(req.TestCaseID)
	}
	all, err := h.store.ListDefects("", "", "")
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	cands := selectDefectCandidates(req.Title, sameTest, all)

	red := func(s string) string {
		if use.Redact {
			return failureanalysis.Redact(s)
		}
		return s
	}
	state := map[string]any{"defect": map[string]any{"title": red(req.Title), "description": red(req.Description),
		"found_by_test": red(testName), "error_message": red(errMsg)}}
	qs := map[string]typesafe.Question{"severity": failureanalysis.DefectSeverityQuestion()}
	if len(cands) > 0 {
		cs := make([]map[string]any, len(cands))
		for k, d := range cands {
			cs[k] = map[string]any{"title": red(d.Title), "description": red(d.Description), "status": d.Status}
			qs[fmt.Sprintf("dup_%d", k)] = failureanalysis.DefectDuplicateQuestion(k)
		}
		state["candidates"] = cs
	}
	resp, err := use.Client.Evaluate(r.Context(), typesafe.Request{State: state, Model: use.Model, Questions: qs})
	if err != nil {
		httpx.JSON(w, http.StatusBadGateway, map[string]string{"error": "TypeSafe.ai could not answer: " + err.Error()})
		return
	}
	h.recordTypeSafeUse(models.AnalysisCostKindDefectAssist, use, resp)
	out := DefectAssistResponse{Duplicates: []DefectDuplicate{}}
	if a, ok := resp.Answers["severity"]; ok && a.Choice != "" {
		out.Severity = &DefectSeveritySuggestion{Value: a.Choice, Confidence: a.Confidence, Probabilities: a.Probabilities}
	}
	for k, d := range cands {
		if a, ok := resp.Answers[fmt.Sprintf("dup_%d", k)]; ok && a.Noul >= failureanalysis.DefectDupReportMin {
			out.Duplicates = append(out.Duplicates, DefectDuplicate{DefectID: d.ID, Title: d.Title, Status: d.Status,
				Severity: d.Severity, ExternalKey: d.ExternalKey, PSame: a.Noul})
		}
	}
	sort.SliceStable(out.Duplicates, func(i, j int) bool { return out.Duplicates[i].PSame > out.Duplicates[j].PSame })
	httpx.JSON(w, http.StatusOK, out)
}
