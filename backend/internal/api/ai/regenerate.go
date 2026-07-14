package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ttgo/internal/api/authctx"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/aigen"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"gorm.io/gorm"
)

const maxRegenInstructionLen = 2000

var validRegenActions = map[string]bool{
	"": true, "make_more_specific": true, "add_negative_case": true, "repair_findings": true,
}

var regenActionText = map[string]string{
	"make_more_specific": "Make this test case more specific: concrete test data, exact field names, precise observable outcomes.",
	"add_negative_case":  "Rewrite this as the NEGATIVE-path variant of the same scenario (invalid input, error handling, rejection).",
	"repair_findings":    "Fix ONLY the reported findings below. Keep everything that is not flagged as close to the original as possible.",
}

// buildRegenerationPrompt renders the focused single-draft revision prompt.
// Deterministic; no template involvement (the run's template shaped the
// original batch — regeneration is a surgical edit).
func buildRegenerationPrompt(req *models.Requirement, original models.DraftContent, instruction, action string, findings []aigen.Finding) string {
	var b strings.Builder
	b.WriteString("You are revising ONE existing software test case draft.\n\n## Requirement\n")
	fmt.Fprintf(&b, "%s: %s\n", req.Identifier, req.Title)
	desc := aigen.StripHTML(req.Description)
	if len(desc) > 4000 {
		desc = desc[:4000] + "\n…(truncated)"
	}
	b.WriteString(desc)

	b.WriteString("\n\n## Current draft (JSON)\n")
	origJSON, _ := json.MarshalIndent(original, "", "  ")
	b.Write(origJSON)

	b.WriteString("\n\n## Revision instructions\n")
	if txt := regenActionText[action]; txt != "" {
		b.WriteString(txt + "\n")
	}
	if instruction != "" {
		b.WriteString(instruction + "\n")
	}
	if len(findings) > 0 {
		b.WriteString("\nFindings to address:\n")
		for _, f := range findings {
			fmt.Fprintf(&b, "- %s: %s\n", f.Field, f.Message)
		}
	}

	b.WriteString("\n## Output Format\nReturn ONLY a valid JSON object of the form " +
		`{"test_cases": [ { "name", "category", "description", "source_refs", "steps": [{"action","expected_result"}] } ]}` +
		" containing exactly ONE revised test case. No markdown fences, no commentary.")
	return b.String()
}

// RegenerateDraft creates an alternative version of one pending draft.
//
// @Summary      Regenerate one AI draft
// @Description  Calls the provider with a focused revision prompt and persists the result as a NEW pending alternative (original untouched, versions retained). Choose between them via the choose endpoint.
// @Tags         ai-generations
// @Accept       json
// @Produce      json
// @Param        id        path  string  true  "Run ID"
// @Param        draft_id  path  string  true  "Draft ID"
// @Param        body      body  object  true  "instruction, action (make_more_specific|add_negative_case|repair_findings), finding_codes"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      422  {object}  map[string]string
// @Router       /ai-generations/{id}/drafts/{draft_id}/regenerate [post]
// @Security     BearerAuth
func (h *Handler) RegenerateDraft(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	draft, err := h.getRunDraft(runID, r.PathValue("draft_id"))
	if err != nil {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("draft not found"))
		return
	}
	if draft.Status != models.AIDraftStatusPending {
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": "only pending drafts can be regenerated"})
		return
	}

	var req struct {
		Instruction  string   `json:"instruction"`
		Action       string   `json:"action"`
		FindingCodes []string `json:"finding_codes"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxLifecycleBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if !validRegenActions[req.Action] {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "unknown action; use make_more_specific, add_negative_case, or repair_findings"})
		return
	}
	if len(req.Instruction) > maxRegenInstructionLen {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "instruction too long"})
		return
	}

	run, err := h.store.GetGenerationRun(runID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("run not found"))
		return
	}
	requirement, err := h.store.GetRequirement(run.RequirementID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("linked requirement not found"))
		return
	}
	providerID := ""
	if run.ProviderID != nil {
		providerID = *run.ProviderID
	}
	providerCfg, err := h.resolveProviderConfig(providerID)
	if err != nil || providerCfg == nil {
		// The run's original provider may have been deleted — fall back to default.
		providerCfg, err = h.resolveProviderConfig("")
		if err != nil || providerCfg == nil {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "no enabled LLM provider configured"})
			return
		}
	}

	original, err := draft.Content()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	findings := selectedFindings(draft, req.FindingCodes, req.Action)
	prompt := buildRegenerationPrompt(requirement, original, req.Instruction, req.Action, findings)

	provider, err := llm.NewProvider(providerCfg)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	timeout := time.Duration(providerCfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	chatReq := llm.ChatRequest{
		Model: providerCfg.ModelName,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: generationSystemMessage},
			{Role: "user", Content: prompt},
		},
		Temperature:    0.5,
		MaxTokens:      2048,
		ResponseFormat: resolveResponseFormat(providerCfg),
	}
	start := time.Now()
	chatResp, retries, callErr := llm.ChatWithRetry(ctx, provider, chatReq, llm.RetryOptions{})
	if callErr != nil && llm.Classify(callErr) == llm.ErrCatSchema && chatReq.ResponseFormat != nil && chatReq.ResponseFormat.Type == "json_schema" {
		chatReq.ResponseFormat = &llm.ResponseFormat{Type: "json_object"}
		chatResp, retries, callErr = llm.ChatWithRetry(ctx, provider, chatReq, llm.RetryOptions{})
	}
	if callErr != nil {
		category := llm.Classify(callErr)
		httpx.JSON(w, httpStatusForCategory(category), map[string]string{
			"error": fmt.Sprintf("regeneration failed: %v", callErr), "category": string(category),
		})
		return
	}
	_ = retries

	parsed, parseErr := parseLLMResponse(chatResp.Content)
	if parseErr != nil || len(parsed) == 0 {
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error": "the provider returned no parseable test case", "category": string(llm.ErrCatParse),
		})
		return
	}
	revised := h.sanitizeGeneratedTestCase(parsed[0])

	validationJSON, qualityJSON, duplicatesJSON, coverageJSON, err := h.analyzeRevision(run, draft, revised)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	var actorID *string
	if u := authctx.UserFromRequest(r); u != nil {
		actorID = &u.ID
	}
	meta := store.RegenMeta{Instruction: req.Instruction, Action: req.Action, DurationMs: time.Since(start).Milliseconds()}
	if chatResp.Usage != nil {
		meta.PromptTokens = chatResp.Usage.PromptTokens
		meta.CompletionTokens = chatResp.Usage.CompletionTokens
	}
	content := models.DraftContent{
		Name: revised.Name, Category: revised.Category, Description: revised.Description,
		SourceRefs: revised.SourceRefs, Steps: revised.Steps,
	}
	alt, err := h.store.CreateDraftAlternative(draft.ID, content, validationJSON, qualityJSON, duplicatesJSON, meta, actorID)
	if err != nil {
		if errors.Is(err, store.ErrDraftNotPending) {
			httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	// Fold the attempt's usage into the run totals (kept cumulative; Stage 6
	// adds per-attempt rows) and refresh the coverage snapshot.
	if chatResp.Usage != nil {
		run.PromptTokens += chatResp.Usage.PromptTokens
		run.CompletionTokens += chatResp.Usage.CompletionTokens
		run.TotalTokens += chatResp.Usage.TotalTokens
		if err := h.store.UpdateGenerationRun(run); err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
	}
	if coverageJSON != "" {
		if err := h.store.UpdateGenerationRunCoverage(run.ID, coverageJSON); err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
	}

	resp, err := alt.ToResponse()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"draft": resp, "original_id": draft.ID})
}

// selectedFindings resolves the findings the repair prompt should list:
// the stored structural + rubric findings, filtered to finding_codes when
// provided (repair_findings), else all of them.
func selectedFindings(draft *models.AIGeneratedDraft, codes []string, action string) []aigen.Finding {
	var all []aigen.Finding
	if draft.ValidationJSON != "" {
		var v []aigen.Finding
		if json.Unmarshal([]byte(draft.ValidationJSON), &v) == nil {
			all = append(all, v...)
		}
	}
	if draft.QualityJSON != "" {
		var dims []aigen.QualityDimension
		if json.Unmarshal([]byte(draft.QualityJSON), &dims) == nil {
			for _, d := range dims {
				all = append(all, d.Findings...)
			}
		}
	}
	if action != "repair_findings" || len(codes) == 0 {
		if action == "repair_findings" {
			return all
		}
		return nil
	}
	want := map[string]bool{}
	for _, c := range codes {
		want[c] = true
	}
	var out []aigen.Finding
	for _, f := range all {
		if want[f.Code] {
			out = append(out, f)
		}
	}
	return out
}

// analyzeRevision runs Stage 3 analysis for the revised content in its
// position family context and returns the four JSON payloads.
func (h *Handler) analyzeRevision(run *models.AIGenerationRun, original *models.AIGeneratedDraft, revised models.GeneratedTestCase) (validationJSON, qualityJSON, duplicatesJSON, coverageJSON string, err error) {
	findings := aigen.ValidateDraft(revised)
	vb, _ := json.Marshal(findings)

	runFull, err := h.store.GetGenerationRunWithDrafts(run.ID)
	if err != nil {
		return "", "", "", "", err
	}
	var targets []aigen.CoverageTarget
	if runFull.CoverageJSON != "" {
		var rep aigen.CoverageReport
		if json.Unmarshal([]byte(runFull.CoverageJSON), &rep) == nil {
			for _, tc := range rep.Targets {
				targets = append(targets, tc.CoverageTarget)
			}
		}
	}
	// Batch context: one draft per position (prefer pending/accepted over
	// superseded/rejected), with the revision standing in at the original's
	// position.
	best := map[int]models.GeneratedTestCase{}
	for _, d := range runFull.Drafts {
		if d.Status == models.AIDraftStatusSuperseded || d.Status == models.AIDraftStatusRejected {
			continue
		}
		c, cerr := d.Content()
		if cerr != nil {
			continue
		}
		best[d.Position] = models.GeneratedTestCase{Name: c.Name, Category: c.Category, Description: c.Description, SourceRefs: c.SourceRefs, Steps: c.Steps}
	}
	best[original.Position] = revised

	var batch []aigen.BatchDraft
	var flat []models.GeneratedTestCase
	nameCounts := map[string]int{}
	for pos, d := range best {
		batch = append(batch, aigen.BatchDraft{Position: pos, Draft: d})
		flat = append(flat, d)
		nameCounts[aigen.NormalizeTestText(d.Name)]++
	}

	dims := aigen.EvaluateDraftQuality(revised, nameCounts, targets)
	qb, _ := json.Marshal(dims)

	cands := aigen.FindBatchDuplicates(batch)[original.Position]
	if existing, derr := h.store.SearchDuplicateCandidates(revised.Name, run.RequirementID, aigen.MaxDuplicateCandidates); derr == nil {
		cands = append(cands, existing...)
	}
	if len(cands) > aigen.MaxDuplicateCandidates {
		cands = cands[:aigen.MaxDuplicateCandidates]
	}
	db, _ := json.Marshal(cands)

	report := aigen.BuildCoverageReport(targets, flat)
	report.BatchFindings = append(report.BatchFindings, aigen.EvaluateBatchQuality(flat)...)
	cb, _ := json.Marshal(report)

	return string(vb), string(qb), string(db), string(cb), nil
}
