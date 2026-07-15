package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"strings"
	"time"
	"ttgo/internal/api/authctx"
	"ttgo/internal/api/httpx"
	"ttgo/internal/safehttp"
	"ttgo/pkg/tracker/aigen"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ────────────────────────────────────────────────────────────────────────────
// US1: LLM Provider Config handlers
// ────────────────────────────────────────────────────────────────────────────

func (h *Handler) ListProviders(w http.ResponseWriter, r *http.Request) {
	cfgs, err := h.store.GetAllProviderConfigs()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]models.LLMProviderConfigResponse, len(cfgs))
	for i, c := range cfgs {
		out[i] = c.MaskedConfig()
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Label                  string   `json:"label"`
		ProviderType           string   `json:"provider_type"`
		EndpointURL            string   `json:"endpoint_url"`
		APIKey                 string   `json:"api_key"`
		ModelName              string   `json:"model_name"`
		TimeoutSeconds         int      `json:"timeout_seconds"`
		IsDefault              bool     `json:"is_default"`
		Enabled                *bool    `json:"enabled"`
		PromptPricePerMTok     *float64 `json:"prompt_price_per_mtok"`
		CompletionPricePerMTok *float64 `json:"completion_price_per_mtok"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := validateProviderInput(req.Label, req.ProviderType, req.ModelName, req.EndpointURL, req.APIKey); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := validateProviderPrices(req.PromptPricePerMTok, req.CompletionPricePerMTok); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	timeout := req.TimeoutSeconds
	if timeout == 0 {
		// Local/Ollama models (especially CoT reasoning models like DeepSeek-R1)
		// can take several minutes; use a generous default. Cloud providers are
		// much faster so 90 s is sufficient there.
		if req.ProviderType == "local" {
			timeout = 600
		} else {
			timeout = 90
		}
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	cfg := &models.LLMProviderConfig{
		Label:                  req.Label,
		ProviderType:           req.ProviderType,
		EndpointURL:            req.EndpointURL,
		APIKey:                 req.APIKey,
		ModelName:              req.ModelName,
		TimeoutSeconds:         timeout,
		IsDefault:              req.IsDefault,
		Enabled:                enabled,
		PromptPricePerMTok:     req.PromptPricePerMTok,
		CompletionPricePerMTok: req.CompletionPricePerMTok,
	}
	if err := h.store.CreateProviderConfig(cfg); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if req.IsDefault {
		if err := h.store.SetDefaultProviderConfig(cfg.ID); err != nil { // don't swallow (F-057)
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
	}
	httpx.JSON(w, http.StatusCreated, cfg.MaskedConfig())
}

func (h *Handler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Label                  string   `json:"label"`
		ProviderType           string   `json:"provider_type"`
		EndpointURL            string   `json:"endpoint_url"`
		APIKey                 string   `json:"api_key"`
		ModelName              string   `json:"model_name"`
		TimeoutSeconds         int      `json:"timeout_seconds"`
		IsDefault              bool     `json:"is_default"`
		Enabled                *bool    `json:"enabled"`
		PromptPricePerMTok     *float64 `json:"prompt_price_per_mtok"`
		CompletionPricePerMTok *float64 `json:"completion_price_per_mtok"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := validateProviderInput(req.Label, req.ProviderType, req.ModelName, req.EndpointURL, req.APIKey); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := validateProviderPrices(req.PromptPricePerMTok, req.CompletionPricePerMTok); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	updates := map[string]interface{}{
		"label":                     req.Label,
		"provider_type":             req.ProviderType,
		"endpoint_url":              req.EndpointURL,
		"model_name":                req.ModelName,
		"is_default":                req.IsDefault,
		"prompt_price_per_mtok":     req.PromptPricePerMTok,
		"completion_price_per_mtok": req.CompletionPricePerMTok,
	}
	if req.TimeoutSeconds > 0 {
		updates["timeout_seconds"] = req.TimeoutSeconds
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	cfg, err := h.store.UpdateProviderConfig(id, updates, req.APIKey)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		if strings.Contains(err.Error(), "already exists") {
			httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if req.IsDefault {
		if err := h.store.SetDefaultProviderConfig(id); err != nil { // don't swallow (F-057)
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		cfg.IsDefault = true
	}
	httpx.JSON(w, http.StatusOK, cfg.MaskedConfig())
}

func (h *Handler) DeleteProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteProviderConfig(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cfg, err := h.store.GetProviderConfigByID(id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}

	provider, err := llm.NewProvider(cfg)
	if err != nil {
		httpx.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	req := llm.ChatRequest{
		Model: cfg.ModelName,
		Messages: []llm.ChatMessage{
			{Role: "user", Content: "Respond with exactly: {\"ok\":true}"},
		},
		Temperature: 0,
		MaxTokens:   50,
	}
	resp, err := provider.Chat(ctx, req)
	if err != nil {
		httpx.JSON(w, http.StatusOK, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"success": true, "response": resp.Content})
}

func (h *Handler) SetDefaultProvider(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.SetDefaultProviderConfig(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// validateProviderInput checks required fields for provider config creation/update.
func validateProviderInput(label, providerType, modelName, endpointURL, apiKey string) error {
	if strings.TrimSpace(label) == "" {
		return fmt.Errorf("label is required")
	}
	allowed := map[string]bool{"local": true, "openai": true, "gemini": true, "anthropic": true}
	if !allowed[providerType] {
		return fmt.Errorf("provider_type must be one of: local, openai, gemini, anthropic")
	}
	if strings.TrimSpace(modelName) == "" {
		return fmt.Errorf("model_name is required")
	}
	if providerType == "local" && strings.TrimSpace(endpointURL) == "" {
		return fmt.Errorf("endpoint_url is required for local providers")
	}
	// SSRF guard: an attacker-set endpoint_url drives server-side outbound LLM
	// calls (and the response is surfaced back), so reject internal/metadata hosts.
	// "local" providers legitimately point at a LAN host, so they use the
	// integration check (allows private but still blocks cloud-metadata); cloud
	// providers use the strict check (F-002).
	if u := strings.TrimSpace(endpointURL); u != "" {
		var verr error
		if providerType == "local" {
			verr = safehttp.ValidateIntegrationURL(u)
		} else {
			verr = safehttp.ValidatePublicURL(u)
		}
		if verr != nil {
			return fmt.Errorf("endpoint_url rejected: %w", verr)
		}
	}
	return nil
}

// validateProviderPrices rejects negative, non-finite, or absurdly large
// per-Mtok prices. A negative price would make estimated cost negative,
// silently offsetting monthly budget spend and bypassing budgets; a huge
// value (e.g. 1e308) makes tokens*price overflow to +Inf, which the JSON
// encoder rejects — the response write then fails and the client gets an
// empty body.
func validateProviderPrices(prompt, completion *float64) error {
	check := func(field string, v *float64) error {
		if v == nil {
			return nil
		}
		if math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 || *v > 1_000_000 {
			return fmt.Errorf("%s must be between 0 and 1000000", field)
		}
		return nil
	}
	if err := check("prompt_price_per_mtok", prompt); err != nil {
		return err
	}
	return check("completion_price_per_mtok", completion)
}

// ────────────────────────────────────────────────────────────────────────────
// Coverage config handlers
// ────────────────────────────────────────────────────────────────────────────

func (h *Handler) GetCoverageConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetOrCreateCoverageConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cfg)
}

func (h *Handler) UpdateCoverageConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EssentialMaxTokens     *int `json:"essential_max_tokens"`
		ThoroughMaxTokens      *int `json:"thorough_max_tokens"`
		ComprehensiveMaxTokens *int `json:"comprehensive_max_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	updates := map[string]interface{}{}
	if req.EssentialMaxTokens != nil {
		if *req.EssentialMaxTokens < 1024 || *req.EssentialMaxTokens > 32768 {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "essential_max_tokens must be between 1024 and 32768"})
			return
		}
		updates["essential_max_tokens"] = *req.EssentialMaxTokens
	}
	if req.ThoroughMaxTokens != nil {
		if *req.ThoroughMaxTokens < 1024 || *req.ThoroughMaxTokens > 32768 {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "thorough_max_tokens must be between 1024 and 32768"})
			return
		}
		updates["thorough_max_tokens"] = *req.ThoroughMaxTokens
	}
	if req.ComprehensiveMaxTokens != nil {
		if *req.ComprehensiveMaxTokens < 1024 || *req.ComprehensiveMaxTokens > 32768 {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "comprehensive_max_tokens must be between 1024 and 32768"})
			return
		}
		updates["comprehensive_max_tokens"] = *req.ComprehensiveMaxTokens
	}
	if len(updates) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "no fields to update"})
		return
	}
	cfg, err := h.store.UpdateCoverageConfig(updates)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cfg)
}

// ────────────────────────────────────────────────────────────────────────────
// US4: Prompt template handlers
// ────────────────────────────────────────────────────────────────────────────

func (h *Handler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	tmpl, err := h.store.GetOrCreateDefaultTemplate()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, tmpl)
}

func (h *Handler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "content is required"})
		return
	}
	tmpl, err := h.store.UpdateTemplateContent(req.Content)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	// Warn when critical placeholders are absent — without them the requirement
	// details are never injected into the LLM prompt.
	required := []string{"{{TITLE}}", "{{DESCRIPTION}}", "{{COVERAGE}}"}
	var missing []string
	for _, ph := range required {
		if !strings.Contains(req.Content, ph) {
			missing = append(missing, ph)
		}
	}
	type updateTemplateResponse struct {
		*models.AIGenTemplate
		Warnings []string `json:"warnings,omitempty"`
	}
	out := updateTemplateResponse{AIGenTemplate: tmpl}
	if len(missing) > 0 {
		out.Warnings = []string{fmt.Sprintf(
			"Template is missing required placeholders: %s — requirement details will NOT be sent to the LLM.",
			strings.Join(missing, ", "),
		)}
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) ResetTemplate(w http.ResponseWriter, r *http.Request) {
	tmpl, err := h.store.ResetTemplateToDefault()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, tmpl)
}

func (h *Handler) UpdateParentTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "content is required"})
		return
	}
	tmpl, err := h.store.UpdateParentTemplateContent(req.Content)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	required := []string{"{{TITLE}}", "{{CHILDREN}}"}
	var missing []string
	for _, ph := range required {
		if !strings.Contains(req.Content, ph) {
			missing = append(missing, ph)
		}
	}
	type resp struct {
		*models.AIGenTemplate
		Warnings []string `json:"warnings,omitempty"`
	}
	out := resp{AIGenTemplate: tmpl}
	if len(missing) > 0 {
		out.Warnings = []string{fmt.Sprintf(
			"Parent template is missing recommended placeholders: %s",
			strings.Join(missing, ", "),
		)}
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) ResetParentTemplate(w http.ResponseWriter, r *http.Request) {
	tmpl, err := h.store.ResetParentTemplateToDefault()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, tmpl)
}

// ────────────────────────────────────────────────────────────────────────────
// US2: Generate test cases from a requirement
// ────────────────────────────────────────────────────────────────────────────

// GenerateTests generates draft test cases from a requirement via the
// configured LLM provider.
//
// Deprecated: use POST /ai-generations. This endpoint now delegates to the
// same durable, idempotent generation lifecycle — the run is persisted and
// each returned temp_id is the ID of a real, persisted draft — but its
// request/response shape stays byte-identical for existing clients (spec: the
// ttgo CLI, internal/cli/client/ai.go, depends on this contract).
//
// @Summary      Generate test cases from a requirement (deprecated)
// @Description  Deprecated: use POST /ai-generations. Delegates to the durable generation lifecycle so runs are persisted; the legacy request/response shape is unchanged.
// @Tags         ai-generations
// @Deprecated
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Requirement ID"
// @Param        body  body  object  true  "provider_id, coverage_level, detail_level, additional_instructions"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      422  {object}  map[string]string
// @Router       /requirements/{id}/generate-tests [post]
// @Security     BearerAuth
func (h *Handler) GenerateTests(w http.ResponseWriter, r *http.Request) {
	requirementID := r.PathValue("id")

	var req struct {
		ProviderID             string `json:"provider_id"`
		CoverageLevel          string `json:"coverage_level"`
		DetailLevel            string `json:"detail_level"`
		AdditionalInstructions string `json:"additional_instructions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	// Delegated to the durable lifecycle (ai-generation-improvements stage 6):
	// runs are persisted and idempotent; the legacy response shape is preserved.
	out := h.executeGeneration(r, createGenerationRequest{
		RequirementID:          requirementID,
		ProviderID:             req.ProviderID,
		CoverageLevel:          req.CoverageLevel,
		DetailLevel:            req.DetailLevel,
		AdditionalInstructions: req.AdditionalInstructions,
		IdempotencyKey:         uuid.New().String(), // legacy calls are never replays
		AcknowledgeBudget:      false,               // budget warnings surface as 409s
	})
	if out.status >= 400 {
		httpx.JSON(w, out.status, out.payload) // {error, category?, run_id?} — supersets the legacy {error}
		return
	}

	drafts := make([]models.GeneratedTestCase, len(out.drafts))
	for i, d := range out.drafts {
		drafts[i] = models.GeneratedTestCase{
			TempID: d.ID, Name: d.Name, Category: d.Category,
			Description: d.Description, SourceRefs: d.SourceRefs, Steps: d.Steps,
		}
	}
	run := out.run
	debug := map[string]interface{}{
		"duration_ms": run.DurationMs, "model": run.ModelName,
		"finish_reason": run.FinishReason, "max_tokens_budget": run.MaxTokens,
		"retried": run.RetryCount > 0, "provider_label": run.ProviderLabel,
		"provider_type": run.ProviderType, "request_context": run.RequestContext,
		"template_type": run.TemplateType,
	}
	if run.TotalTokens > 0 {
		debug["usage"] = map[string]int{
			"prompt_tokens": run.PromptTokens, "completion_tokens": run.CompletionTokens,
			"total_tokens": run.TotalTokens,
		}
	}
	// out.provider is nil only on an idempotency-replay outcome produced
	// before provider resolution (executeGeneration's fast path) — legacy
	// calls always mint a fresh uuid key above so that path is unreachable
	// here, but guard rather than assume: fall back to the run's own provider.
	var providerResp models.LLMProviderConfigResponse
	if out.provider != nil {
		providerResp = out.provider.MaskedConfig()
	} else if run.ProviderID != nil {
		if cfg, cerr := h.resolveProviderConfig(*run.ProviderID); cerr == nil && cfg != nil {
			providerResp = cfg.MaskedConfig()
		}
	}
	resp := map[string]interface{}{
		"drafts":   drafts,
		"provider": providerResp,
		"debug":    debug,
	}
	if tw, ok := out.payload["template_warning"]; ok {
		resp["template_warning"] = tw
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// resolveProviderConfig returns the requested provider, or the default enabled
// one, or the first enabled one. A nil return with nil error means none configured.
func (h *Handler) resolveProviderConfig(providerID string) (*models.LLMProviderConfig, error) {
	if providerID != "" {
		cfg, err := h.store.GetProviderConfigByID(providerID)
		if err != nil {
			return nil, fmt.Errorf("provider not found: %s", providerID)
		}
		return cfg, nil
	}
	cfgs, err := h.store.GetAllProviderConfigs()
	if err != nil {
		return nil, err
	}
	for _, c := range cfgs {
		if c.IsDefault && c.Enabled {
			return c, nil
		}
	}
	for _, c := range cfgs {
		if c.Enabled {
			return c, nil
		}
	}
	return nil, nil
}

// coverageLevelGuidance returns the prompt guidance for a given coverage level.
// Returns "" for unknown levels.
func coverageLevelGuidance(level string) string {
	switch level {
	case "essential":
		return "Only the primary happy path and most likely failure. No edge cases or boundary tests."
	case "thorough":
		return "Cover happy paths, key negatives, and boundary values. No redundant tests."
	case "comprehensive":
		return "Exhaust all categories: functional, negative, boundary, edge case, security, accessibility. Cover every distinct scenario."
	default:
		return ""
	}
}

// coverageMaxTokens returns the max_tokens budget for a given coverage level using the config.
func coverageMaxTokens(level string, cfg *models.AIGenCoverageConfig) int {
	if cfg == nil {
		switch level {
		case "essential":
			return 4096
		case "comprehensive":
			return 16384
		default:
			return 8192
		}
	}
	switch level {
	case "essential":
		return cfg.EssentialMaxTokens
	case "comprehensive":
		return cfg.ComprehensiveMaxTokens
	default:
		return cfg.ThoroughMaxTokens
	}
}

// buildChildrenContext formats child requirements into a text block for the LLM prompt.
func buildChildrenContext(children []*models.Requirement) string {
	if len(children) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\nChild Issues / Sub-tickets:\n")
	for i, child := range children {
		sb.WriteString(fmt.Sprintf("\n%d. [%s] %s", i+1, child.Identifier, child.Title))
		desc := strings.TrimSpace(child.Description)
		if desc != "" {
			plain := stripHTMLTags(desc)
			if len(plain) > 500 {
				plain = plain[:500] + "..."
			}
			sb.WriteString("\n   " + plain)
		}
	}
	return sb.String()
}

// stripHTMLTags removes HTML tags from a string for plain-text prompt context.
func stripHTMLTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// llmDraftShape is the JSON shape the LLM is expected to output per test case.
// Defined at package level so both parseLLMResponse and collectJSONObjects share it.
type llmDraftShape struct {
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	SourceRefs  []string `json:"source_refs"`
	Steps       []struct {
		Action         string `json:"action"`
		ExpectedResult string `json:"expected_result"`
	} `json:"steps"`
}

// parseLLMResponse extracts a []GeneratedTestCase from the raw LLM text.
// It is intentionally lenient: it handles <think> blocks (closed or truncated),
// markdown fences, object wrappers, and leading/trailing prose.
func parseLLMResponse(raw string) ([]models.GeneratedTestCase, error) {
	original := strings.TrimSpace(raw) // kept for fallback strategies
	raw = original

	// ── 1. Strip <think>…</think> reasoning blocks (DeepSeek-R1, QwQ, etc.) ──
	// Case A: properly closed block — try text AFTER </think> first.
	if closeIdx := strings.Index(raw, "</think>"); closeIdx != -1 {
		raw = strings.TrimSpace(raw[closeIdx+len("</think>"):])
	} else if openIdx := strings.Index(raw, "<think>"); openIdx != -1 {
		// Case B: block was opened but never closed (model spent all tokens
		// reasoning and never emitted the JSON outside the tag).
		// Strip the opening tag and keep everything that follows.
		raw = strings.TrimSpace(raw[openIdx+len("<think>"):])
	}

	// ── 2. Strip markdown code fences ──
	if strings.HasPrefix(raw, "```") {
		lines := strings.Split(raw, "\n")
		var inner []string
		for i, line := range lines {
			if i == 0 && strings.HasPrefix(line, "```") {
				continue
			}
			if line == "```" {
				continue
			}
			inner = append(inner, line)
		}
		raw = strings.TrimSpace(strings.Join(inner, "\n"))
	}

	var drafts []llmDraftShape

	// ── 3. Attempt structured parse strategies in order ──

	// Strategy 0: strict canonical envelope {"test_cases":[...]} — the shape
	// providers are asked (and schema-forced) to return. Tried first so the
	// canonical path never depends on the tolerant fallbacks below.
	if strings.HasPrefix(raw, "{") {
		var envelope struct {
			TestCases []llmDraftShape `json:"test_cases"`
		}
		if err := json.Unmarshal([]byte(raw), &envelope); err == nil && len(envelope.TestCases) > 0 {
			drafts = envelope.TestCases
		}
	}

	// Strategy A: bare JSON array.
	if len(drafts) == 0 && strings.HasPrefix(raw, "[") {
		_ = json.Unmarshal([]byte(raw), &drafts)
	}

	// Strategy B: object wrapper like {"test_cases": [...]} or {"items": [...]}.
	if len(drafts) == 0 && strings.HasPrefix(raw, "{") {
		var wrapper map[string]json.RawMessage
		if err := json.Unmarshal([]byte(raw), &wrapper); err == nil {
			for _, v := range wrapper {
				var candidate []llmDraftShape
				if json.Unmarshal(v, &candidate) == nil && len(candidate) > 0 {
					drafts = candidate
					break
				}
			}
		}
	}

	// Strategy C: scan the think-stripped text for the first complete JSON array.
	if len(drafts) == 0 {
		if extracted := extractFirstJSONArray(raw); extracted != "" {
			_ = json.Unmarshal([]byte(extracted), &drafts)
		}
	}

	// Strategy D: search the ORIGINAL full response (including inside <think>).
	// Handles the common case where a reasoning model embeds the JSON array
	// inside its <think> block and never outputs anything after </think>.
	if len(drafts) == 0 && original != raw {
		if extracted := extractFirstJSONArray(original); extracted != "" {
			_ = json.Unmarshal([]byte(extracted), &drafts)
		}
	}

	// Strategy E: collect standalone JSON objects scattered in the text and
	// wrap them in an array.  Handles models that output one object per line
	// instead of a proper array.
	if len(drafts) == 0 {
		if collected := collectJSONObjects(original); len(collected) > 0 {
			drafts = collected
		}
	}

	if len(drafts) == 0 {
		return nil, fmt.Errorf("failed to parse JSON array: no valid JSON array found in LLM response")
	}

	result := make([]models.GeneratedTestCase, len(drafts))
	for i, d := range drafts {
		steps := make([]models.GeneratedStep, len(d.Steps))
		for j, st := range d.Steps {
			steps[j] = models.GeneratedStep{
				Action:         st.Action,
				ExpectedResult: st.ExpectedResult,
			}
		}
		result[i] = models.GeneratedTestCase{
			TempID:      uuid.New().String(),
			Name:        d.Name,
			Category:    normalizeCategory(d.Category, d.Name),
			Description: d.Description,
			SourceRefs:  d.SourceRefs,
			Steps:       steps,
		}
	}
	return result, nil
}

// extractFirstJSONArray scans s for the first syntactically complete JSON array
// and returns it. Returns "" if no complete array is found.
// This allows us to recover JSON that is surrounded by prose, reasoning text, or
// incomplete XML-style tags emitted by chain-of-thought models.
func extractFirstJSONArray(s string) string {
	start := strings.Index(s, "[")
	if start == -1 {
		return ""
	}
	depth := 0
	inString := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if inString {
			if c == '\\' {
				escape = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return "" // array was opened but never closed
}

// collectJSONObjects scans s for all top-level JSON objects that look like
// test case drafts (have a "name" key) and returns them as a slice.
// This handles models that output one object per line instead of a proper array.
func collectJSONObjects(s string) []llmDraftShape {
	var results []llmDraftShape
	i := 0
	for i < len(s) {
		start := strings.Index(s[i:], "{")
		if start == -1 {
			break
		}
		start += i
		depth := 0
		inStr := false
		esc := false
		end := -1
		for j := start; j < len(s); j++ {
			c := s[j]
			if esc {
				esc = false
				continue
			}
			if inStr {
				if c == '\\' {
					esc = true
				} else if c == '"' {
					inStr = false
				}
				continue
			}
			switch c {
			case '"':
				inStr = true
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					end = j
				}
			}
			if end != -1 {
				break
			}
		}
		if end == -1 {
			break
		}
		var d llmDraftShape
		if json.Unmarshal([]byte(s[start:end+1]), &d) == nil && d.Name != "" {
			results = append(results, d)
		}
		i = end + 1
	}
	return results
}

// normalizeCategory returns a cleaned category string.
// If the explicit category is empty, it attempts to extract one from
// the "[Category]" prefix pattern in the test case name.
func normalizeCategory(explicit, name string) string {
	cat := strings.TrimSpace(explicit)
	if cat != "" {
		return cat
	}
	if strings.HasPrefix(name, "[") {
		if idx := strings.Index(name, "]"); idx > 1 {
			return strings.TrimSpace(name[1:idx])
		}
	}
	return ""
}

// buildBuiltinTemplate returns the hard-coded fallback template.
func buildBuiltinTemplate() string {
	return `You are a QA engineer. Generate test cases for:
Title: {{TITLE}}
Description: {{DESCRIPTION}}
{{CHILDREN}}
Detail Level: {{DETAIL_LEVEL}}
Coverage: {{COVERAGE}}
{{ADDITIONAL_INSTRUCTIONS}}

Name tests as "[Category] Verb + Object + Condition".
Actions must name exact UI elements/fields and include concrete test data (emails, dates, boundary values).
Expected results must state the exact observable outcome — never "works correctly" or "error is shown".
For negative tests, state both what should happen (error message) and what should NOT happen (data not saved).
Each test must be self-contained with its own setup steps.
Return ONLY a valid JSON object — no markdown, no explanation.
{"test_cases":[{"name":"...","category":"Functional","description":"...","source_refs":[],"steps":[{"action":"...","expected_result":"..."}]}]}
The "category" field must be one of: Functional, Negative, Boundary, Edge Case, Security, Performance, API, Mobile/Responsive, Accessibility — or a brief custom category if none fit.`
}

func buildBuiltinParentTemplate() string {
	return `You are a QA engineer. Generate test cases for a requirement with child issues:
Title: {{TITLE}}
Description: {{DESCRIPTION}}

Child Issues:
{{CHILDREN}}

Detail Level: {{DETAIL_LEVEL}}
Coverage: {{COVERAGE}}
{{ADDITIONAL_INSTRUCTIONS}}

Generate at least one test per child issue. Name tests so it is clear which child issue they cover.
Return ONLY a valid JSON object — no markdown, no explanation.
{"test_cases":[{"name":"...","category":"Functional","description":"...","source_refs":[],"steps":[{"action":"...","expected_result":"..."}]}]}`
}

// ────────────────────────────────────────────────────────────────────────────
// US3: Accept generated test cases
// ────────────────────────────────────────────────────────────────────────────

// AcceptGeneratedTests materializes selected generated test-case drafts into
// real test cases linked to the requirement.
//
// Deprecated: use POST /ai-generations/{id}/accept, which operates on durable
// draft records with idempotent replay. When every submitted temp_id is a
// persisted draft of one durable run linked to this requirement (i.e. the
// client round-tripped a response from the delegated GenerateTests), this
// endpoint completes that lifecycle — saving any client-side edits, then
// accepting atomically — instead of materializing orphaned test cases with no
// draft trail. Otherwise it falls back to transient materialization, which is
// now atomic (spec: fixes the historic bug where one bad test case in a batch
// left the earlier ones already created).
//
// @Summary      Accept generated test cases (deprecated)
// @Description  Deprecated: use POST /ai-generations/{id}/accept. Completes the durable lifecycle when the temp_ids round-trip a delegated run, else atomically materializes transient drafts.
// @Tags         ai-generations
// @Deprecated
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Requirement ID"
// @Param        body  body  object  true  "folder_id, tests, group_by_category"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      422  {object}  map[string]string
// @Router       /requirements/{id}/accept-generated-tests [post]
// @Security     BearerAuth
func (h *Handler) AcceptGeneratedTests(w http.ResponseWriter, r *http.Request) {
	requirementID := r.PathValue("id")

	// Verify requirement exists.
	if _, err := h.store.GetRequirement(requirementID); err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}

	var req struct {
		FolderID        string                     `json:"folder_id"`
		Tests           []models.GeneratedTestCase `json:"tests"`
		GroupByCategory bool                       `json:"group_by_category"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if len(req.Tests) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "no tests to accept"})
		return
	}
	if req.FolderID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "folder_id is required"})
		return
	}

	// Stage 6: when temp_ids identify persisted drafts of ONE run linked to
	// this requirement (i.e. the client round-tripped a delegated generate
	// response), complete the lifecycle: save edits, then accept atomically.
	// Once ANY temp_id resolves to a persisted draft, the request IS a
	// lifecycle accept and must be well-formed or rejected outright — it must
	// never be silently downgraded to transient legacy creation, which would
	// double-create test cases while leaving the matched draft(s) pending
	// forever.
	runID, draftByID, err := h.matchLifecycleDrafts(requirementID, req.Tests)
	if err != nil {
		if errors.Is(err, errMalformedLifecycleAccept) {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		} else {
			httpx.Error(w, http.StatusInternalServerError, err)
		}
		return
	}
	if runID != "" {
		var actorID *string
		if u := authctx.UserFromRequest(r); u != nil {
			actorID = &u.ID
		}
		ids := make([]string, 0, len(req.Tests))
		for _, tcase := range req.Tests {
			d := draftByID[tcase.TempID]
			ids = append(ids, d.ID)
			content := models.DraftContent{
				Name: tcase.Name, Category: tcase.Category, Description: tcase.Description,
				SourceRefs: tcase.SourceRefs, Steps: tcase.Steps,
			}
			if stored, err := d.Content(); err == nil && !reflect.DeepEqual(stored, content) {
				findings := aigen.ValidateDraft(tcase)
				vb, _ := json.Marshal(findings)
				if _, err := h.store.SaveDraftEdit(d.ID, content, string(vb), "", "", actorID); err != nil {
					httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
					return
				}
			}
		}
		result, err := h.store.AcceptGenerationDrafts(runID, ids, req.FolderID, req.GroupByCategory, actorID)
		if err != nil {
			h.writeLegacyAcceptError(w, err)
			return
		}
		httpx.JSON(w, http.StatusCreated, map[string]interface{}{
			"created_ids": result.CreatedTestCaseIDs, "count": len(result.CreatedTestCaseIDs),
			"subfolders_created": result.SubfoldersCreated,
		})
		return
	}

	// Fallback: transient drafts from pre-delegation clients — now atomic.
	result, err := h.store.AcceptLegacyGeneratedTests(requirementID, req.FolderID, req.Tests, req.GroupByCategory)
	if err != nil {
		h.writeLegacyAcceptError(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{
		"created_ids": result.CreatedTestCaseIDs, "count": len(result.CreatedTestCaseIDs),
		"subfolders_created": result.SubfoldersCreated,
	})
}

// errMalformedLifecycleAccept indicates the request's test temp_ids reference
// one or more persisted generation drafts but do not form a single complete,
// unique, single-run lifecycle accept (duplicate temp_ids, a partial/typo'd
// id mixed in with real ones, drafts spanning multiple runs, or a run linked
// to a different requirement). Once any temp_id resolves to a persisted
// draft, the request must be treated as a lifecycle-accept attempt and
// rejected when malformed — never silently downgraded to transient legacy
// materialization, which would double-create test cases and orphan the
// matched draft(s) in pending status.
var errMalformedLifecycleAccept = errors.New("test temp_ids reference generation drafts but do not form a single valid lifecycle accept")

// matchLifecycleDrafts classifies an accept request into one of three
// outcomes:
//   - runID != "", err == nil: every test's temp_id is a persisted draft of
//     one single run whose requirement matches — a clean lifecycle accept.
//   - runID == "", err == nil: no temp_id matches any persisted draft — a
//     genuine transient/legacy accept; the caller may fall back safely.
//   - err != nil wrapping errMalformedLifecycleAccept: some (but not a clean
//     complete set of) temp_ids matched persisted drafts — the caller must
//     reject the request (400), not fall back to transient creation. Any
//     other non-nil error is a store/DB failure (caller should return 500).
func (h *Handler) matchLifecycleDrafts(requirementID string, tests []models.GeneratedTestCase) (string, map[string]*models.AIGeneratedDraft, error) {
	n := len(tests)
	if n == 0 {
		return "", nil, nil
	}
	ids := make([]string, 0, n)
	for _, t := range tests {
		if t.TempID == "" {
			// A missing temp_id can't be a round-tripped delegated response —
			// treat the whole request as transient.
			return "", nil, nil
		}
		ids = append(ids, t.TempID)
	}
	uniqueSet := make(map[string]struct{}, n)
	unique := make([]string, 0, n)
	for _, id := range ids {
		if _, ok := uniqueSet[id]; !ok {
			uniqueSet[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	drafts, err := h.store.GetDraftsByIDs(unique)
	if err != nil {
		return "", nil, err
	}
	matched := len(drafts)
	if matched == 0 {
		return "", nil, nil // no lifecycle drafts involved — transient fallback OK
	}
	// At least one real draft is referenced: this must be a complete, unique,
	// single-run set or it's rejected. matched <= len(unique) <= n always
	// (GetDraftsByIDs returns at most one row per requested id), so
	// matched == n forces len(unique) == n (no duplicates) AND every id
	// resolved to a persisted draft — catching both duplicates and
	// partial/typo'd batches in one comparison.
	if matched != n {
		return "", nil, fmt.Errorf("incomplete or duplicate draft temp_ids: %w", errMalformedLifecycleAccept)
	}
	byID := make(map[string]*models.AIGeneratedDraft, matched)
	runID := drafts[0].RunID
	for _, d := range drafts {
		if d.RunID != runID {
			return "", nil, fmt.Errorf("temp_ids span multiple runs: %w", errMalformedLifecycleAccept)
		}
		byID[d.ID] = d
	}
	run, err := h.store.GetGenerationRun(runID)
	if err != nil {
		return "", nil, err
	}
	if run.RequirementID != requirementID {
		return "", nil, fmt.Errorf("drafts belong to a different requirement: %w", errMalformedLifecycleAccept)
	}
	return runID, byID, nil
}

// writeLegacyAcceptError maps store acceptance errors onto legacy-friendly
// statuses (mirrors AcceptGeneration's mapping).
func (h *Handler) writeLegacyAcceptError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrDraftNotPending):
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, store.ErrAmbiguousDraftVersion):
		httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, store.ErrDraftInvalid):
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	case errors.Is(err, store.ErrUnknownDrafts), errors.Is(err, gorm.ErrRecordNotFound):
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		httpx.Error(w, http.StatusInternalServerError, err)
	}
}
