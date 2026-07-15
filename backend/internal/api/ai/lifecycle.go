package ai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"ttgo/internal/api/authctx"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/aigen"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const maxLifecycleBodyBytes = 1 << 20 // bound request payloads (spec: Privacy and Security)
const maxAdditionalInstructionsLen = 4000

type createGenerationRequest struct {
	RequirementID          string `json:"requirement_id"`
	ProviderID             string `json:"provider_id"`
	CoverageLevel          string `json:"coverage_level"`
	DetailLevel            string `json:"detail_level"`
	AdditionalInstructions string `json:"additional_instructions"`
	IdempotencyKey         string `json:"idempotency_key"`
	ParentRunID            string `json:"parent_run_id"`
	RunCritic              bool   `json:"run_critic"`
	AcknowledgeBudget      bool   `json:"acknowledge_budget"`
}

// runResponse builds the canonical {run, drafts} payload for a run.
func (h *Handler) runResponse(run *models.AIGenerationRun) (map[string]interface{}, error) {
	if run.Drafts == nil {
		full, err := h.store.GetGenerationRunWithDrafts(run.ID)
		if err != nil {
			return nil, err
		}
		run = full
	}
	draftResponses := make([]*models.AIGeneratedDraftResponse, 0, len(run.Drafts))
	for _, d := range run.Drafts {
		dr, err := d.ToResponse()
		if err != nil {
			return nil, err
		}
		draftResponses = append(draftResponses, dr)
	}
	out := map[string]interface{}{"run": run, "drafts": draftResponses}
	if run.CoverageJSON != "" {
		out["coverage"] = json.RawMessage(run.CoverageJSON)
	}
	if attempts, err := h.store.ListGenerationAttempts(run.ID); err == nil && len(attempts) > 0 {
		out["attempts"] = attempts
	}
	return out, nil
}

// analyzeDraftQuality computes rubric dimensions and duplicate candidates for
// every draft. Index i in the returned slices corresponds to drafts[i]
// (create-time position). FTS lookup failures degrade to batch-only
// candidates — quality analysis must never fail a run.
func (h *Handler) analyzeDraftQuality(drafts []models.GeneratedTestCase, targets []aigen.CoverageTarget, requirementID string) (qualityJSONs, duplicatesJSONs []string) {
	nameCounts := map[string]int{}
	batch := make([]aigen.BatchDraft, len(drafts))
	for i, d := range drafts {
		nameCounts[aigen.NormalizeTestText(d.Name)]++
		batch[i] = aigen.BatchDraft{Position: i, Draft: d}
	}
	batchDupes := aigen.FindBatchDuplicates(batch)

	qualityJSONs = make([]string, len(drafts))
	duplicatesJSONs = make([]string, len(drafts))
	for i, d := range drafts {
		dims := aigen.EvaluateDraftQuality(d, nameCounts, targets)
		qb, _ := json.Marshal(dims)
		qualityJSONs[i] = string(qb)

		cands := batchDupes[i]
		existing, err := h.store.SearchDuplicateCandidates(d.Name, requirementID, aigen.MaxDuplicateCandidates)
		if err != nil {
			slog.Warn("ai_generation: duplicate candidate search failed", "error", err)
		} else {
			cands = append(cands, existing...)
		}
		if len(cands) > aigen.MaxDuplicateCandidates {
			cands = cands[:aigen.MaxDuplicateCandidates]
		}
		db, _ := json.Marshal(cands)
		duplicatesJSONs[i] = string(db)
	}
	return qualityJSONs, duplicatesJSONs
}

// recomputeEditedDraftQuality rebuilds runID's draft batch with `content`
// applied at draftID's position (mirroring create-time behavior: accepted/
// rejected drafts still occupy positions and count toward name-uniqueness/
// duplicate checks) and returns that draft's fresh quality and duplicate
// JSON, plus the coverage targets and rebuilt batch — callers that also
// refresh run-level coverage (e.g. the PATCH endpoint) need those two to
// build the coverage report without re-fetching. Shared by the lifecycle
// PATCH (UpdateGenerationDraft) and the deprecated legacy accept adapter
// (AcceptGeneratedTests) so an edit never blanks Stage-3 analysis the way a
// bare "" used to for legacy accept.
func (h *Handler) recomputeEditedDraftQuality(runID, draftID string, content models.DraftContent) (qualityJSON, duplicatesJSON string, targets []aigen.CoverageTarget, all []models.GeneratedTestCase, err error) {
	runWithDrafts, err := h.store.GetGenerationRunWithDrafts(runID)
	if err != nil {
		return "", "", nil, nil, err
	}
	if runWithDrafts.CoverageJSON != "" {
		var stored aigen.CoverageReport
		if json.Unmarshal([]byte(runWithDrafts.CoverageJSON), &stored) == nil {
			for _, tc := range stored.Targets {
				targets = append(targets, tc.CoverageTarget)
			}
		}
	}
	// Rebuild the batch with the edit applied at this draft's position.
	all = make([]models.GeneratedTestCase, 0, len(runWithDrafts.Drafts))
	editedPos := 0
	for _, d := range runWithDrafts.Drafts {
		c, cerr := d.Content()
		if cerr != nil {
			return "", "", nil, nil, cerr
		}
		gc := models.GeneratedTestCase{Name: c.Name, Category: c.Category, Description: c.Description, SourceRefs: c.SourceRefs, Steps: c.Steps}
		if d.ID == draftID {
			editedPos = len(all)
			gc = models.GeneratedTestCase{Name: content.Name, Category: content.Category, Description: content.Description, SourceRefs: content.SourceRefs, Steps: content.Steps}
		}
		all = append(all, gc)
	}
	qualityJSONs, duplicatesJSONs := h.analyzeDraftQuality(all, targets, runWithDrafts.RequirementID)
	return qualityJSONs[editedPos], duplicatesJSONs[editedPos], targets, all, nil
}

// httpStatusForCategory maps a normalized LLM error category to the HTTP status
// the original request returned, so an idempotency-key replay of a failed run
// reproduces that status instead of a misleading 200.
func httpStatusForCategory(category llm.ErrorCategory) int {
	switch category {
	case llm.ErrCatTimeout:
		return http.StatusGatewayTimeout
	case llm.ErrCatRateLimit:
		return http.StatusTooManyRequests
	case llm.ErrCatParse, llm.ErrCatValidation:
		return http.StatusUnprocessableEntity
	case llm.ErrCatInternal:
		return http.StatusInternalServerError
	default: // provider, authentication, authorization, schema (provider rejected
		// our structured-output request — a gateway/capability fault, not client input)
		return http.StatusBadGateway
	}
}

// generationOutcome is executeGeneration's result: an HTTP status, the exact
// payload CreateGeneration writes today, plus typed accessors the legacy
// GenerateTests adapter needs (Task 7: ai-generation-improvements stage 6).
type generationOutcome struct {
	status   int
	payload  map[string]interface{}
	run      *models.AIGenerationRun
	drafts   []*models.AIGeneratedDraftResponse
	provider *models.LLMProviderConfig
}

// errorPayload mirrors httpx.Error's status→payload mapping (httpx/response.go:20)
// for flows that return a response instead of writing one: ≥500 is logged and
// genericized for the client; anything else echoes the error text.
func errorPayload(status int, err error) map[string]interface{} {
	if status >= 500 {
		slog.Error("server error", "status", status, "error", err)
		return map[string]interface{}{"error": "internal server error"}
	}
	return map[string]interface{}{"error": err.Error()}
}

// existingRunOutcome answers an idempotency-key replay. The key is scoped to
// the requesting requirement: reusing it for a different requirement is a
// conflict, not a silent replay of the unrelated run. Only a completed run
// replays its stored result (200); a failed run replays its original failure
// status; a cancelled or still-in-flight run conflicts (409). Payload-returning
// counterpart of the former writeExistingRun, shared by CreateGeneration (via
// executeGeneration) and the legacy GenerateTests adapter.
func (h *Handler) existingRunOutcome(run *models.AIGenerationRun, expectedRequirementID string) *generationOutcome {
	if run.RequirementID != expectedRequirementID {
		return &generationOutcome{status: http.StatusConflict, run: run, payload: map[string]interface{}{
			"error":  "this idempotency key was already used for a different requirement",
			"run_id": run.ID,
		}}
	}
	switch run.Status {
	case models.AIGenerationRunStatusCompleted:
		out, err := h.runResponse(run)
		if err != nil {
			return &generationOutcome{status: http.StatusInternalServerError, run: run, payload: errorPayload(http.StatusInternalServerError, err)}
		}
		outcome := &generationOutcome{status: http.StatusOK, run: run, payload: out}
		if dr, ok := out["drafts"].([]*models.AIGeneratedDraftResponse); ok {
			outcome.drafts = dr
		}
		return outcome
	case models.AIGenerationRunStatusFailed:
		return &generationOutcome{
			status: httpStatusForCategory(llm.ErrorCategory(run.ErrorCategory)),
			run:    run,
			payload: map[string]interface{}{
				"error":    run.ErrorMessage,
				"category": run.ErrorCategory,
				"run_id":   run.ID,
			},
		}
	case models.AIGenerationRunStatusCancelled:
		return &generationOutcome{status: http.StatusConflict, run: run, payload: map[string]interface{}{
			"error":  "this generation was cancelled; start a new one",
			"run_id": run.ID,
		}}
	default: // pending / running
		return &generationOutcome{status: http.StatusConflict, run: run, payload: map[string]interface{}{
			"error":  "a generation with this idempotency key is already in progress",
			"run_id": run.ID,
		}}
	}
}

// resolveResponseFormat picks the structured-output mode for a provider:
// cloud OpenAI-compatible endpoints get the strict envelope schema; Anthropic
// and local providers are prompt-only (the envelope system message still applies).
func resolveResponseFormat(cfg *models.LLMProviderConfig) *llm.ResponseFormat {
	switch cfg.ProviderType {
	case "openai", "gemini":
		return &llm.ResponseFormat{
			Type:       "json_schema",
			SchemaName: aigen.EnvelopeSchemaName,
			Schema:     aigen.EnvelopeSchemaJSON,
		}
	default:
		return nil
	}
}

// sanitizeGeneratedTestCase applies the draft sanitization rules shared by
// generation and regeneration (XSS-hardening: bc984a5, 5b26981, 068a0de).
// Category and step text are attacker-influenceable too (a requirement
// description can prompt-inject the LLM into echoing HTML/JS). Sanitize them
// in place — mirroring the /import/accept path — BEFORE validation, so a
// payload that sanitizes to "" (e.g. a pure-<script> action) is seen as empty
// by ValidateDraft and rejected rather than persisted.
func (h *Handler) sanitizeGeneratedTestCase(d models.GeneratedTestCase) models.GeneratedTestCase {
	d.Name = html.UnescapeString(h.sanitizer.Sanitize(d.Name))
	d.Description = httpx.NormalizeEmptyHTML(h.sanitizer, d.Description)
	d.Category = html.UnescapeString(h.sanitizer.Sanitize(d.Category))
	for i := range d.Steps {
		// Steps are plain-text prose (rendered as plain text in the UI, like
		// Name/Category), so unescape the sanitizer's entity encoding
		// (' -> &#39;, & -> &amp;) back to text. NormalizeEmptyHTML still runs
		// first, so a visually-empty/pure-markup step normalizes to "" and is
		// rejected by ValidateDraft.
		d.Steps[i].Action = html.UnescapeString(httpx.NormalizeEmptyHTML(h.sanitizer, d.Steps[i].Action))
		d.Steps[i].ExpectedResult = html.UnescapeString(httpx.NormalizeEmptyHTML(h.sanitizer, d.Steps[i].ExpectedResult))
	}
	for i := range d.SourceRefs {
		d.SourceRefs[i] = httpx.NormalizeEmptyHTML(h.sanitizer, d.SourceRefs[i])
	}
	return d
}

// failRun stamps a failed/cancelled terminal state onto the run.
func (h *Handler) failRun(run *models.AIGenerationRun, status string, category llm.ErrorCategory, msg string, start time.Time, retries int) {
	now := time.Now()
	run.Status = status
	run.CompletedAt = &now
	run.DurationMs = time.Since(start).Milliseconds()
	run.ErrorCategory = string(category)
	run.ErrorMessage = msg
	run.RetryCount = retries
	if err := h.store.UpdateGenerationRun(run); err != nil {
		slog.Error("ai_generation: failed to persist run failure", "run_id", run.ID, "error", err)
	}
}

// CreateGeneration creates (or idempotently returns) a generation run and
// executes it synchronously.
//
// @Summary      Create an AI generation run
// @Description  Creates an idempotent generation run for a requirement, calls the configured LLM provider with structured output, validates and persists the drafts, and returns the completed run. Replaying a finished idempotency key returns the stored result without calling the provider.
// @Tags         ai-generations
// @Accept       json
// @Produce      json
// @Param        body  body  object  true  "requirement_id (required), provider_id, coverage_level, detail_level, additional_instructions, idempotency_key, parent_run_id, run_critic"
// @Success      201  {object}  map[string]interface{}
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      422  {object}  map[string]string
// @Router       /ai-generations [post]
// @Security     BearerAuth
func (h *Handler) CreateGeneration(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLifecycleBodyBytes)
	var req createGenerationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	out := h.executeGeneration(r, req)
	httpx.JSON(w, out.status, out.payload)
}

// executeGeneration runs the entire idempotent generation flow — idempotency
// replay, prompt-plan + provider resolution, the LLM call(s) with retry/repair,
// sanitization/validation/persistence of drafts, and the critic pass — and
// returns the response instead of writing it, so CreateGeneration and the
// legacy GenerateTests adapter (Task 7: ai-generation-improvements stage 6)
// can share one implementation.
func (h *Handler) executeGeneration(r *http.Request, req createGenerationRequest) *generationOutcome {
	if req.RequirementID == "" {
		return &generationOutcome{status: http.StatusBadRequest, payload: map[string]interface{}{"error": "requirement_id is required"}}
	}
	if len(req.AdditionalInstructions) > maxAdditionalInstructionsLen {
		return &generationOutcome{status: http.StatusBadRequest, payload: map[string]interface{}{"error": "additional_instructions is too long"}}
	}

	// Idempotent replay fast-path: a key that already finished returns its
	// stored result without resolving providers or calling the LLM; a key
	// still in flight conflicts. CreateGenerationRun re-checks under the
	// unique index, so a concurrent duplicate landing between here and the
	// insert is still caught below.
	if req.IdempotencyKey != "" {
		existing, err := h.store.GetGenerationRunByKey(req.IdempotencyKey)
		if err != nil {
			return &generationOutcome{status: http.StatusInternalServerError, payload: errorPayload(http.StatusInternalServerError, err)}
		}
		if existing != nil {
			return h.existingRunOutcome(existing, req.RequirementID)
		}
	}

	// Resolve prompt plan (validates requirement + coverage level too).
	plan, err := h.buildPromptPlan(req.RequirementID, req.CoverageLevel, req.DetailLevel, req.AdditionalInstructions)
	if err != nil {
		return planErrorOutcome(err)
	}
	providerCfg, err := h.resolveProviderConfig(req.ProviderID)
	if err != nil {
		// Mirror GenerateTests: an explicit-but-missing provider_id is a client
		// error (400); a store-list failure while resolving the default is a
		// server fault (500). resolveProviderConfig conflates both, so branch
		// on whether the caller named a provider.
		if req.ProviderID != "" {
			return &generationOutcome{status: http.StatusBadRequest, payload: errorPayload(http.StatusBadRequest, err)}
		}
		return &generationOutcome{status: http.StatusInternalServerError, payload: errorPayload(http.StatusInternalServerError, err)}
	}
	if providerCfg == nil {
		return &generationOutcome{status: http.StatusBadRequest, payload: map[string]interface{}{"error": "no enabled LLM provider configured"}}
	}

	user := authctx.UserFromRequest(r)
	var userID *string
	if user != nil {
		userID = &user.ID
	}
	templateHash := ""
	if tmpl, terr := h.store.GetOrCreateDefaultTemplate(); terr == nil {
		content := tmpl.Content
		if plan.TemplateType == "parent" {
			content = tmpl.ParentContent
		}
		sum := sha256.Sum256([]byte(content))
		templateHash = hex.EncodeToString(sum[:])
	}
	run := &models.AIGenerationRun{
		IdempotencyKey:         req.IdempotencyKey,
		RequirementID:          req.RequirementID,
		UserID:                 userID,
		ProviderID:             &providerCfg.ID,
		ProviderLabel:          providerCfg.Label,
		ProviderType:           providerCfg.ProviderType,
		ModelName:              providerCfg.ModelName,
		TemplateType:           plan.TemplateType,
		TemplateHash:           templateHash,
		CoverageLevel:          plan.CoverageLevel,
		DetailLevel:            req.DetailLevel,
		AdditionalInstructions: req.AdditionalInstructions,
		MaxTokens:              plan.MaxTokens,
		RequestContext:         plan.Prompt,
	}
	if req.ParentRunID != "" {
		run.ParentRunID = &req.ParentRunID
	}
	if warn := h.checkBudget(providerCfg, len(plan.Prompt), plan.MaxTokens, req.AcknowledgeBudget); warn != nil {
		// TOCTOU guard: a concurrent same-key request may have created (and been
		// billed for) the run between our fast-path lookup and this gate. That would
		// push us over budget here, but this request is really an idempotent retry —
		// replay the existing run rather than 409ing it.
		if req.IdempotencyKey != "" {
			if existing, lookErr := h.store.GetGenerationRunByKey(req.IdempotencyKey); lookErr == nil && existing != nil {
				return h.existingRunOutcome(existing, req.RequirementID)
			}
		}
		return &generationOutcome{status: http.StatusConflict, payload: warn}
	}
	run, created, err := h.store.CreateGenerationRun(run)
	if err != nil {
		return &generationOutcome{status: http.StatusInternalServerError, payload: errorPayload(http.StatusInternalServerError, err)}
	}
	if !created {
		// Lost a same-key race between the fast-path check and the insert.
		return h.existingRunOutcome(run, req.RequirementID)
	}
	if err := h.store.MarkGenerationRunRunning(run.ID); err != nil {
		return &generationOutcome{status: http.StatusInternalServerError, payload: errorPayload(http.StatusInternalServerError, err)}
	}
	// MarkGenerationRunRunning only touches the DB row (targeted column
	// update); UpdateGenerationRun below does a full-row Save, so the
	// in-memory run must mirror both fields it changed or a later Save
	// would silently null started_at back out.
	startedAt := time.Now()
	run.Status = models.AIGenerationRunStatusRunning
	run.StartedAt = &startedAt

	// ── Execute the run synchronously ──
	provider, err := llm.NewProvider(providerCfg)
	if err != nil {
		h.failRun(run, models.AIGenerationRunStatusFailed, llm.ErrCatInternal, err.Error(), time.Now(), 0)
		return &generationOutcome{status: http.StatusInternalServerError, payload: errorPayload(http.StatusInternalServerError, fmt.Errorf("failed to initialize provider: %w", err))}
	}
	timeout := time.Duration(providerCfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	h.inflight.register(run.ID, cancel)
	defer h.inflight.unregister(run.ID)

	start := time.Now()
	responseFormat := resolveResponseFormat(providerCfg)
	chatReq := llm.ChatRequest{
		Model: providerCfg.ModelName,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: generationSystemMessage},
			{Role: "user", Content: plan.Prompt},
		},
		Temperature:    0.7,
		MaxTokens:      plan.MaxTokens,
		ResponseFormat: responseFormat,
	}
	totalRetries := 0
	attemptStart := time.Now()
	chatResp, retries, err := llm.ChatWithRetry(ctx, provider, chatReq, llm.RetryOptions{})
	totalRetries += retries
	h.recordAttempt(run.ID, nil, models.AIGenAttemptGeneration, providerCfg, chatResp, err, attemptStart, retries)
	if err != nil && llm.Classify(err) == llm.ErrCatSchema && responseFormat != nil && responseFormat.Type == "json_schema" {
		// The endpoint rejected json_schema — downgrade once to json_object.
		slog.WarnContext(r.Context(), "ai_generation: provider rejected json_schema, downgrading to json_object", "error", err)
		chatReq.ResponseFormat = &llm.ResponseFormat{Type: "json_object"}
		totalRetries++
		downgradeStart := time.Now()
		var r2 int
		chatResp, r2, err = llm.ChatWithRetry(ctx, provider, chatReq, llm.RetryOptions{})
		totalRetries += r2
		h.recordAttempt(run.ID, nil, models.AIGenAttemptGeneration, providerCfg, chatResp, err, downgradeStart, r2)
	}
	h.auditGeneration(r, run, providerCfg, plan.CoverageLevel, start, err, ctx)
	if err != nil {
		return h.generationFailureOutcome(r, run, err, start, totalRetries, providerCfg)
	}

	drafts, parseErr := parseLLMResponse(chatResp.Content)
	if parseErr != nil && ctx.Err() == nil {
		// One targeted repair attempt for malformed structured output.
		slog.WarnContext(r.Context(), "ai_generation: parse failed, one repair attempt", "error", parseErr)
		totalRetries++
		repairReq := chatReq
		repairReq.Temperature = 0.3
		repairReq.Messages = []llm.ChatMessage{
			{Role: "system", Content: "CRITICAL: Output ONLY a raw JSON object of the form " +
				"{\"test_cases\": [...]}. No reasoning, no <think> tags, no markdown, no commentary. " +
				"Your entire response must start with { and end with }."},
			{Role: "user", Content: plan.Prompt},
		}
		repairStart := time.Now()
		repairResp, repairRetries, repairErr := llm.ChatWithRetry(ctx, provider, repairReq, llm.RetryOptions{})
		h.recordAttempt(run.ID, nil, models.AIGenAttemptParseRepair, providerCfg, repairResp, repairErr, repairStart, repairRetries)
		if repairErr == nil {
			if repairDrafts, e2 := parseLLMResponse(repairResp.Content); e2 == nil {
				drafts, parseErr = repairDrafts, nil
				accumulateUsage(chatResp, repairResp)
			}
		}
	}
	if parseErr != nil {
		msg := "LLM returned an unexpected response format after retrying. Try a lower coverage level, a different model, or a longer provider timeout."
		h.failRun(run, models.AIGenerationRunStatusFailed, llm.ErrCatParse, parseErr.Error(), start, totalRetries)
		return &generationOutcome{status: http.StatusUnprocessableEntity, run: run, payload: map[string]interface{}{
			"error": msg, "category": string(llm.ErrCatParse), "run_id": run.ID,
		}}
	}

	// Sanitize + validate + persist drafts.
	invalid := 0
	rows := make([]*models.AIGeneratedDraft, len(drafts))
	// parsed retains the fully sanitized test cases (the per-iteration `d`
	// below is a value copy — its Name/Category/Description edits do not
	// write back into `drafts[i]`) for the quality/coverage analysis below.
	parsed := make([]models.GeneratedTestCase, len(drafts))
	for i, d := range drafts {
		d = h.sanitizeGeneratedTestCase(d)
		parsed[i] = d
		findings := aigen.ValidateDraft(d)
		if aigen.HasErrors(findings) {
			invalid++
		}
		findingsJSON, _ := json.Marshal(findings)
		content := models.DraftContent{
			Name: d.Name, Category: d.Category, Description: d.Description,
			SourceRefs: d.SourceRefs, Steps: d.Steps,
		}
		originalJSON, _ := json.Marshal(content)
		row := &models.AIGeneratedDraft{ID: uuid.New().String(), OriginalJSON: string(originalJSON), ValidationJSON: string(findingsJSON)}
		if err := row.ApplyContent(content); err != nil {
			return &generationOutcome{status: http.StatusInternalServerError, run: run, payload: errorPayload(http.StatusInternalServerError, err)}
		}
		rows[i] = row
	}

	// Coverage targets are derived from the requirement description + child
	// requirements (plan.Requirement is always non-nil on this path — unknown
	// requirements 404 in buildPromptPlan — but the guard keeps an
	// empty/blank requirement_id from panicking if validation ever changes).
	var targets []aigen.CoverageTarget
	if plan.Requirement != nil {
		targets = aigen.ExtractCoverageTargets(plan.Requirement.Description, plan.Children)
	}
	qualityJSONs, duplicatesJSONs := h.analyzeDraftQuality(parsed, targets, run.RequirementID)
	for i := range rows {
		rows[i].QualityJSON = qualityJSONs[i]
		rows[i].DuplicatesJSON = duplicatesJSONs[i]
	}

	actor := userID
	if err := h.store.CreateGenerationDrafts(run.ID, actor, rows, invalid); err != nil {
		h.failRun(run, models.AIGenerationRunStatusFailed, llm.ErrCatInternal, err.Error(), start, totalRetries)
		return &generationOutcome{status: http.StatusInternalServerError, run: run, payload: errorPayload(http.StatusInternalServerError, err)}
	}

	criticWarning := ""
	if req.RunCritic && (plan.CoverageLevel == "thorough" || plan.CoverageLevel == "comprehensive") && len(rows) > 0 {
		criticStart := time.Now()
		criticResp, warn := h.runCriticPass(ctx, provider, providerCfg, plan.Requirement.Title, parsed, rows)
		criticWarning = warn
		if criticResp != nil {
			h.recordAttempt(run.ID, nil, models.AIGenAttemptCritic, providerCfg, criticResp, nil, criticStart, 0)
			accumulateUsage(chatResp, criticResp)
		}
	}

	now := time.Now()
	run.Status = models.AIGenerationRunStatusCompleted
	run.CompletedAt = &now
	run.DurationMs = time.Since(start).Milliseconds()
	run.RetryCount = totalRetries
	run.FinishReason = chatResp.FinishReason
	if chatResp.Model != "" {
		run.ModelName = chatResp.Model
	}
	if chatResp.Usage != nil {
		run.PromptTokens = chatResp.Usage.PromptTokens
		run.CompletionTokens = chatResp.Usage.CompletionTokens
		run.TotalTokens = chatResp.Usage.TotalTokens
		run.EstimatedCost = llm.EstimateCostUSD(run.PromptTokens, run.CompletionTokens,
			providerCfg.PromptPricePerMTok, providerCfg.CompletionPricePerMTok)
	}
	report := aigen.BuildCoverageReport(targets, parsed)
	report.BatchFindings = append(report.BatchFindings, aigen.EvaluateBatchQuality(parsed)...)
	if covJSON, err := json.Marshal(report); err == nil {
		run.CoverageJSON = string(covJSON)
	}
	if err := h.store.UpdateGenerationRun(run); err != nil {
		return &generationOutcome{status: http.StatusInternalServerError, run: run, payload: errorPayload(http.StatusInternalServerError, err)}
	}
	run.Drafts = rows

	out, err := h.runResponse(run)
	if err != nil {
		return &generationOutcome{status: http.StatusInternalServerError, run: run, payload: errorPayload(http.StatusInternalServerError, err)}
	}
	if plan.TemplateWarning != "" {
		out["template_warning"] = plan.TemplateWarning
	}
	if criticWarning != "" {
		out["critic_warning"] = criticWarning
	}
	outcome := &generationOutcome{status: http.StatusCreated, payload: out, run: run, provider: providerCfg}
	if dr, ok := out["drafts"].([]*models.AIGeneratedDraftResponse); ok {
		outcome.drafts = dr
	}
	return outcome
}

// recordAttempt persists one provider round-trip, best-effort. It must never
// fail a run — store errors are logged and swallowed (spec: "Persist token
// usage and latency per run and attempt" is additive telemetry, not a
// correctness dependency for the generation/regeneration flow it observes).
func (h *Handler) recordAttempt(runID string, draftID *string, kind string, cfg *models.LLMProviderConfig, resp *llm.ChatResponse, callErr error, start time.Time, retries int) {
	a := &models.AIGenerationAttempt{
		RunID: runID, DraftID: draftID, Kind: kind,
		ModelName: cfg.ModelName, DurationMs: time.Since(start).Milliseconds(), Retries: retries,
	}
	if resp != nil {
		if resp.Model != "" {
			a.ModelName = resp.Model
		}
		if resp.Usage != nil {
			a.PromptTokens = resp.Usage.PromptTokens
			a.CompletionTokens = resp.Usage.CompletionTokens
			a.TotalTokens = resp.Usage.TotalTokens
			a.EstimatedCost = llm.EstimateCostUSD(a.PromptTokens, a.CompletionTokens,
				cfg.PromptPricePerMTok, cfg.CompletionPricePerMTok)
		}
	}
	if callErr != nil {
		a.ErrorCategory = string(llm.Classify(callErr))
	}
	if err := h.store.CreateGenerationAttempt(a); err != nil {
		slog.Warn("ai_generation: attempt persist failed", "run_id", runID, "error", err)
	}
}

// accumulateUsage folds a repair attempt's usage/finish reason into the first response.
func accumulateUsage(first, second *llm.ChatResponse) {
	if second == nil {
		return
	}
	if first.Usage != nil && second.Usage != nil {
		first.Usage.PromptTokens += second.Usage.PromptTokens
		first.Usage.CompletionTokens += second.Usage.CompletionTokens
		first.Usage.TotalTokens += second.Usage.TotalTokens
	} else if second.Usage != nil {
		first.Usage = second.Usage
	}
	if second.FinishReason != "" {
		first.FinishReason = second.FinishReason
	}
}

// auditGeneration mirrors the legacy endpoint's audit-log entry (no prompt text).
func (h *Handler) auditGeneration(r *http.Request, run *models.AIGenerationRun, cfg *models.LLMProviderConfig, coverage string, start time.Time, callErr error, ctx context.Context) {
	status := "success"
	if callErr != nil {
		status = "failure"
		if ctx.Err() == context.DeadlineExceeded {
			status = "timeout"
		}
	}
	userID := ""
	if u := authctx.UserFromRequest(r); u != nil {
		userID = u.ID
	}
	_ = h.store.CreateAuditLog(&models.AuditLog{
		ID: uuid.New().String(),
		Action: fmt.Sprintf("ai_generation_run:%s:requirement:%s:provider:%s:status:%s:coverage:%s:duration_ms:%d",
			run.ID, run.RequirementID, cfg.ID, status, coverage, time.Since(start).Milliseconds()),
		UserID:    userID,
		Timestamp: time.Now(),
	})
}

// generationFailureOutcome finishes the run in a terminal state and maps the
// error category to an HTTP status with an actionable message. Payload-
// returning counterpart of the former writeGenerationFailure — still performs
// every side effect (failRun + logging) exactly as before, just returns the
// response instead of writing it.
func (h *Handler) generationFailureOutcome(r *http.Request, run *models.AIGenerationRun, err error, start time.Time, retries int, cfg *models.LLMProviderConfig) *generationOutcome {
	category := llm.Classify(err)
	status := models.AIGenerationRunStatusFailed
	if category == llm.ErrCatCancelled {
		status = models.AIGenerationRunStatusCancelled
	}
	h.failRun(run, status, category, err.Error(), start, retries)
	slog.ErrorContext(r.Context(), "ai_generation: run failed", "run_id", run.ID, "category", category, "error", err)

	if category == llm.ErrCatCancelled {
		// The canceller may be another session (cancel endpoint) — the
		// original requester might still be connected. Writing to a gone
		// client is a harmless no-op.
		return &generationOutcome{status: http.StatusConflict, run: run, payload: map[string]interface{}{
			"error": "generation cancelled", "category": string(llm.ErrCatCancelled), "run_id": run.ID,
		}}
	}
	// Same category→status mapping a later idempotency replay uses, so the
	// first-time failure and its replay always agree.
	httpStatus := httpStatusForCategory(category)
	msg := fmt.Sprintf("LLM generation failed: %v", err)
	switch category {
	case llm.ErrCatTimeout:
		msg = fmt.Sprintf("LLM request timed out after %d seconds", cfg.TimeoutSeconds)
	case llm.ErrCatRateLimit:
		msg = "the provider rate-limited this request after retries; wait a moment and try again"
	case llm.ErrCatAuthentication, llm.ErrCatAuthorization:
		msg = "the provider rejected the configured API key; check the provider settings"
	}
	return &generationOutcome{status: httpStatus, run: run, payload: map[string]interface{}{
		"error": msg, "category": string(category), "run_id": run.ID,
	}}
}

// GetGeneration returns one run with its drafts and findings.
//
// @Summary      Get an AI generation run
// @Tags         ai-generations
// @Produce      json
// @Param        id  path  string  true  "Run ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]string
// @Router       /ai-generations/{id} [get]
// @Security     BearerAuth
func (h *Handler) GetGeneration(w http.ResponseWriter, r *http.Request) {
	run, err := h.store.GetGenerationRunWithDrafts(r.PathValue("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "generation run not found"})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	out, err := h.runResponse(run)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// ListGenerations returns a requirement's generation history, newest first.
//
// @Summary      List AI generation runs for a requirement
// @Tags         ai-generations
// @Produce      json
// @Param        requirement_id  query  string  true   "Requirement ID"
// @Param        limit           query  int     false  "Max runs (default 50)"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Router       /ai-generations [get]
// @Security     BearerAuth
func (h *Handler) ListGenerations(w http.ResponseWriter, r *http.Request) {
	requirementID := r.URL.Query().Get("requirement_id")
	if requirementID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "requirement_id query parameter is required"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	runs, err := h.store.ListGenerationRuns(requirementID, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"runs": runs})
}

// getRunDraft loads a draft and verifies it belongs to the run in the path.
func (h *Handler) getRunDraft(runID, draftID string) (*models.AIGeneratedDraft, error) {
	var d models.AIGeneratedDraft
	if err := h.store.DB().First(&d, "id = ? AND run_id = ?", draftID, runID).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

type updateDraftRequest struct {
	Name        *string                 `json:"name"`
	Category    *string                 `json:"category"`
	Description *string                 `json:"description"`
	SourceRefs  *[]string               `json:"source_refs"`
	Steps       *[]models.GeneratedStep `json:"steps"`
}

// UpdateGenerationDraft saves a partial edit to a pending draft, re-validates,
// and returns the updated draft with fresh findings.
//
// @Summary      Edit a generated draft
// @Tags         ai-generations
// @Accept       json
// @Produce      json
// @Param        id        path  string  true  "Run ID"
// @Param        draft_id  path  string  true  "Draft ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Router       /ai-generations/{id}/drafts/{draft_id} [patch]
// @Security     BearerAuth
func (h *Handler) UpdateGenerationDraft(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLifecycleBodyBytes)
	draft, err := h.getRunDraft(r.PathValue("id"), r.PathValue("draft_id"))
	if err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "draft not found"})
		return
	}
	var req updateDraftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	content, err := draft.Content()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if req.Name != nil {
		content.Name = html.UnescapeString(h.sanitizer.Sanitize(*req.Name))
	}
	if req.Category != nil {
		content.Category = html.UnescapeString(h.sanitizer.Sanitize(*req.Category))
	}
	if req.Description != nil {
		content.Description = httpx.NormalizeEmptyHTML(h.sanitizer, *req.Description)
	}
	if req.SourceRefs != nil {
		refs := *req.SourceRefs
		for k := range refs {
			refs[k] = httpx.NormalizeEmptyHTML(h.sanitizer, refs[k])
		}
		content.SourceRefs = refs
	}
	if req.Steps != nil {
		// Step text is attacker-influenceable (edits can come from a client
		// echoing LLM output) — sanitize Action/ExpectedResult in place before
		// validation, mirroring CreateGeneration's draft-generation path.
		steps := *req.Steps
		for j := range steps {
			// Plain-text prose fields — unescape entities back to text, matching
			// sanitizeGeneratedTestCase (see the note there).
			steps[j].Action = html.UnescapeString(httpx.NormalizeEmptyHTML(h.sanitizer, steps[j].Action))
			steps[j].ExpectedResult = html.UnescapeString(httpx.NormalizeEmptyHTML(h.sanitizer, steps[j].ExpectedResult))
		}
		content.Steps = steps
	}

	findings := aigen.ValidateDraft(models.GeneratedTestCase{
		Name: content.Name, Category: content.Category, Description: content.Description,
		SourceRefs: content.SourceRefs, Steps: content.Steps,
	})
	findingsJSON, _ := json.Marshal(findings)

	var actorID *string
	if u := authctx.UserFromRequest(r); u != nil {
		actorID = &u.ID
	}

	// Recompute quality, duplicates, and run coverage against the edited
	// content. Only pending drafts can be edited, but accepted/rejected
	// drafts still participate in the batch (they occupy positions and their
	// names count for uniqueness/duplicates) — matches create-time behavior.
	runID := r.PathValue("id")
	qualityJSON, duplicatesJSON, targets, all, err := h.recomputeEditedDraftQuality(runID, draft.ID, content)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	updated, err := h.store.SaveDraftEdit(draft.ID, content, string(findingsJSON), qualityJSON, duplicatesJSON, actorID)
	if err != nil {
		if errors.Is(err, store.ErrDraftNotPending) {
			httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	report := aigen.BuildCoverageReport(targets, all)
	report.BatchFindings = append(report.BatchFindings, aigen.EvaluateBatchQuality(all)...)
	covJSON, _ := json.Marshal(report)
	if err := h.store.UpdateGenerationRunCoverage(runID, string(covJSON)); err != nil {
		slog.Warn("ai_generation: coverage refresh failed", "run_id", runID, "error", err)
	}

	resp, err := updated.ToResponse()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"draft": resp, "coverage": json.RawMessage(covJSON)})
}

// RejectGenerationDraftEndpoint records a structured rejection reason.
//
// @Summary      Reject a generated draft
// @Tags         ai-generations
// @Accept       json
// @Produce      json
// @Param        id        path  string  true  "Run ID"
// @Param        draft_id  path  string  true  "Draft ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Router       /ai-generations/{id}/drafts/{draft_id}/reject [post]
// @Security     BearerAuth
func (h *Handler) RejectGenerationDraftEndpoint(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLifecycleBodyBytes)
	draft, err := h.getRunDraft(r.PathValue("id"), r.PathValue("draft_id"))
	if err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "draft not found"})
		return
	}
	var req struct {
		Reason string `json:"reason"`
		Note   string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if !models.AIRejectionReasons[req.Reason] {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{
			"error": "reason must be one of: duplicate, irrelevant, incorrect, too_vague, incomplete_coverage, poor_steps, other",
		})
		return
	}
	if len(req.Note) > 2000 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "note is too long"})
		return
	}
	// Sanitize the free-text note before persisting it on the event trail, so a
	// future events-feed endpoint can never surface stored HTML/JS.
	note := httpx.NormalizeEmptyHTML(h.sanitizer, req.Note)
	var actorID *string
	if u := authctx.UserFromRequest(r); u != nil {
		actorID = &u.ID
	}
	rejected, err := h.store.RejectGenerationDraft(draft.ID, req.Reason, note, actorID)
	if err != nil {
		if errors.Is(err, store.ErrDraftNotPending) {
			httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	resp, err := rejected.ToResponse()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"draft": resp})
}

// RestoreGenerationDraftEndpoint returns a rejected draft to pending.
//
// @Summary      Restore a rejected AI draft
// @Description  Moves a rejected draft back to pending and records a `restored` lifecycle event.
// @Tags         ai-generations
// @Produce      json
// @Param        id        path  string  true  "Run ID"
// @Param        draft_id  path  string  true  "Draft ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Router       /ai-generations/{id}/drafts/{draft_id}/restore [post]
// @Security     BearerAuth
func (h *Handler) RestoreGenerationDraftEndpoint(w http.ResponseWriter, r *http.Request) {
	draft, err := h.getRunDraft(r.PathValue("id"), r.PathValue("draft_id"))
	if err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "draft not found"})
		return
	}
	var actorID *string
	if u := authctx.UserFromRequest(r); u != nil {
		actorID = &u.ID
	}
	restored, err := h.store.RestoreGenerationDraft(draft.ID, actorID)
	if err != nil {
		if errors.Is(err, store.ErrDraftNotRejected) {
			httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	resp, err := restored.ToResponse()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"draft": resp})
}

// AcceptGeneration atomically materializes selected drafts into test cases.
//
// @Summary      Accept generated drafts
// @Description  Creates category subfolders, test cases with steps, and requirement links for the selected drafts in ONE transaction; any failure rolls back the whole batch. Replaying a fully-accepted selection returns the stored test-case IDs.
// @Tags         ai-generations
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "Run ID"
// @Param        body  body  object  true  "folder_id, draft_ids, group_by_category, idempotency_key"
// @Success      201  {object}  map[string]interface{}
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      422  {object}  map[string]string
// @Router       /ai-generations/{id}/accept [post]
// @Security     BearerAuth
func (h *Handler) AcceptGeneration(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxLifecycleBodyBytes)
	runID := r.PathValue("id")
	var req struct {
		FolderID        string   `json:"folder_id"`
		DraftIDs        []string `json:"draft_ids"`
		GroupByCategory bool     `json:"group_by_category"`
		IdempotencyKey  string   `json:"idempotency_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.FolderID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "folder_id is required"})
		return
	}
	if len(req.DraftIDs) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "draft_ids must not be empty"})
		return
	}
	if _, err := h.store.GetGenerationRun(runID); err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "generation run not found"})
		return
	}
	var actorID *string
	if u := authctx.UserFromRequest(r); u != nil {
		actorID = &u.ID
	}
	res, err := h.store.AcceptGenerationDrafts(runID, req.DraftIDs, req.FolderID, req.GroupByCategory, actorID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrDraftNotPending):
			httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		case errors.Is(err, store.ErrAmbiguousDraftVersion):
			httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		case errors.Is(err, store.ErrDraftInvalid):
			httpx.JSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		case errors.Is(err, store.ErrUnknownDrafts):
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, gorm.ErrRecordNotFound):
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			httpx.Error(w, http.StatusInternalServerError, err)
		}
		return
	}
	status := http.StatusCreated
	if res.AlreadyAccepted {
		status = http.StatusOK
	}
	httpx.JSON(w, status, map[string]interface{}{
		"created_ids":        res.CreatedTestCaseIDs,
		"count":              len(res.CreatedTestCaseIDs),
		"subfolders_created": res.SubfoldersCreated,
		"already_accepted":   res.AlreadyAccepted,
	})
}

// CancelGeneration cancels a running generation when possible.
//
// @Summary      Cancel an AI generation run
// @Description  Fires the in-flight run's cancellation (202). A stale `running` run (e.g. after a server restart) is stamped cancelled (200). Terminal runs conflict (409).
// @Tags         ai-generations
// @Produce      json
// @Param        id  path  string  true  "Run ID"
// @Success      200  {object}  map[string]interface{}
// @Success      202  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Router       /ai-generations/{id}/cancel [post]
// @Security     BearerAuth
func (h *Handler) CancelGeneration(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	run, err := h.store.GetGenerationRun(runID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, fmt.Errorf("run not found"))
		return
	}
	if h.inflight.cancel(runID) {
		httpx.JSON(w, http.StatusAccepted, map[string]string{"status": "cancelling", "run_id": runID})
		return
	}
	switch run.Status {
	case models.AIGenerationRunStatusPending, models.AIGenerationRunStatusRunning:
		// Not in flight in THIS process — stale after a crash/restart. Stamp it.
		now := time.Now()
		run.Status = models.AIGenerationRunStatusCancelled
		run.ErrorCategory = string(llm.ErrCatCancelled)
		run.ErrorMessage = "cancelled while stale (no in-flight request)"
		run.CompletedAt = &now
		if err := h.store.UpdateGenerationRun(run); err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		if err := h.store.AppendGenerationEvent(&models.AIGenerationEvent{
			RunID: runID, EventType: models.AIGenEventValidated,
			MetadataJSON: `{"cancelled_stale":true}`,
		}); err != nil {
			slog.Warn("ai_generation: cancel event failed", "error", err)
		}
		httpx.JSON(w, http.StatusOK, map[string]interface{}{"run": run})
	default:
		httpx.JSON(w, http.StatusConflict, map[string]string{
			"error": fmt.Sprintf("run is already %s", run.Status),
		})
	}
}
