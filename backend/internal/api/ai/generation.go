package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
	"ttgo/internal/api/authctx"
	"ttgo/internal/api/httpx"
	"ttgo/internal/safehttp"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"

	"log/slog"

	"github.com/google/uuid"
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
		Label          string `json:"label"`
		ProviderType   string `json:"provider_type"`
		EndpointURL    string `json:"endpoint_url"`
		APIKey         string `json:"api_key"`
		ModelName      string `json:"model_name"`
		TimeoutSeconds int    `json:"timeout_seconds"`
		IsDefault      bool   `json:"is_default"`
		Enabled        *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := validateProviderInput(req.Label, req.ProviderType, req.ModelName, req.EndpointURL, req.APIKey); err != nil {
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
		Label:          req.Label,
		ProviderType:   req.ProviderType,
		EndpointURL:    req.EndpointURL,
		APIKey:         req.APIKey,
		ModelName:      req.ModelName,
		TimeoutSeconds: timeout,
		IsDefault:      req.IsDefault,
		Enabled:        enabled,
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
		Label          string `json:"label"`
		ProviderType   string `json:"provider_type"`
		EndpointURL    string `json:"endpoint_url"`
		APIKey         string `json:"api_key"`
		ModelName      string `json:"model_name"`
		TimeoutSeconds int    `json:"timeout_seconds"`
		IsDefault      bool   `json:"is_default"`
		Enabled        *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := validateProviderInput(req.Label, req.ProviderType, req.ModelName, req.EndpointURL, req.APIKey); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	updates := map[string]interface{}{
		"label":         req.Label,
		"provider_type": req.ProviderType,
		"endpoint_url":  req.EndpointURL,
		"model_name":    req.ModelName,
		"is_default":    req.IsDefault,
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

	// Fetch the requirement for context.
	requirement, err := h.store.GetRequirement(requirementID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}

	// Fetch children for context (if this is a parent requirement)
	children, _ := h.store.ListChildRequirements(requirementID)
	childrenContext := buildChildrenContext(children)

	// Resolve provider.
	var providerCfg *models.LLMProviderConfig
	if req.ProviderID != "" {
		providerCfg, err = h.store.GetProviderConfigByID(req.ProviderID)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, fmt.Errorf("provider not found: %s", req.ProviderID))
			return
		}
	} else {
		// Use default enabled provider.
		cfgs, err := h.store.GetAllProviderConfigs()
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		for _, c := range cfgs {
			if c.IsDefault && c.Enabled {
				providerCfg = c
				break
			}
		}
		if providerCfg == nil {
			for _, c := range cfgs {
				if c.Enabled {
					providerCfg = c
					break
				}
			}
		}
		if providerCfg == nil {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "no enabled LLM provider configured"})
			return
		}
	}

	// Default values.
	coverageLevel := req.CoverageLevel
	if coverageLevel == "" {
		coverageLevel = "thorough"
	}
	coverageGuidance := coverageLevelGuidance(coverageLevel)
	if coverageGuidance == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "coverage_level must be one of: essential, thorough, comprehensive"})
		return
	}

	// Scale coverage guidance when children are present but standard template is used
	// (parent template already has its own children-aware instructions)
	if len(children) > 0 {
		coverageGuidance += fmt.Sprintf(
			"\n\nIMPORTANT: This requirement has %d child issues/sub-tickets. "+
				"Generate at least one test case per child issue to ensure full coverage.",
			len(children))
	}

	detailLevel := req.DetailLevel
	if detailLevel == "" {
		detailLevel = "Standard"
	}

	// Load coverage config for max_tokens.
	coverageCfg, _ := h.store.GetOrCreateCoverageConfig()
	maxTokens := coverageMaxTokens(coverageLevel, coverageCfg)

	// Load template — use parent template when children are present.
	templateWarning := ""
	tmpl, err := h.store.GetOrCreateDefaultTemplate()
	var promptTemplate string
	templateType := "standard"
	if err != nil {
		templateWarning = "Using built-in default template (custom template unavailable)"
		if len(children) > 0 {
			promptTemplate = buildBuiltinParentTemplate()
			templateType = "parent"
		} else {
			promptTemplate = buildBuiltinTemplate()
		}
	} else if len(children) > 0 && strings.TrimSpace(tmpl.ParentContent) != "" {
		promptTemplate = tmpl.ParentContent
		templateType = "parent"
	} else if strings.TrimSpace(tmpl.Content) != "" {
		promptTemplate = tmpl.Content
	} else {
		templateWarning = "Using built-in default template (custom template unavailable)"
		promptTemplate = buildBuiltinTemplate()
	}

	// Assemble prompt.
	prompt := assemblePrompt(promptTemplate, requirement, childrenContext, coverageGuidance, detailLevel, req.AdditionalInstructions)

	// Call LLM.
	provider, err := llm.NewProvider(providerCfg)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to initialize provider: %w", err))
		return
	}

	timeout := time.Duration(providerCfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// ── Build the chat request ──
	// A system message instructs the model to skip reasoning / <think> blocks
	// and return JSON directly. This dramatically reduces wasted tokens for
	// chain-of-thought models (DeepSeek-R1, QwQ, etc.) that otherwise spend
	// their entire budget on reasoning and never emit the actual answer.
	systemMsg := "You are a JSON API that generates software test cases. " +
		"Respond with ONLY a valid JSON array. " +
		"Do NOT include <think> tags, markdown fences, or any text outside the JSON. " +
		"Start your response with the [ character."

	chatReq := llm.ChatRequest{
		Model: providerCfg.ModelName,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemMsg},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.7,
		MaxTokens:   maxTokens,
	}

	start := time.Now()
	chatResp, err := provider.Chat(ctx, chatReq)

	// Log audit event regardless of success/failure.
	user := authctx.UserFromRequest(r)
	userID := ""
	if user != nil {
		userID = user.ID
	}
	auditStatus := "success"
	if err != nil {
		auditStatus = "failure"
		if ctx.Err() == context.DeadlineExceeded {
			auditStatus = "timeout"
		}
	}
	_ = h.store.CreateAuditLog(&models.AuditLog{
		ID:         uuid.New().String(),
		TestCaseID: "",
		Action: fmt.Sprintf("ai_generation:requirement:%s:provider:%s:status:%s:coverage:%s:duration_ms:%d",
			requirementID, providerCfg.ID, auditStatus, coverageLevel, time.Since(start).Milliseconds()),
		UserID:    userID,
		Timestamp: time.Now(),
	})

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			httpx.JSON(w, http.StatusGatewayTimeout, map[string]string{
				"error": fmt.Sprintf("LLM request timed out after %d seconds", providerCfg.TimeoutSeconds),
			})
			return
		}
		slog.ErrorContext(r.Context(), "ai_generation: LLM call failed", "error", err)
		httpx.Error(w, http.StatusBadGateway, fmt.Errorf("LLM generation failed: %w", err))
		return
	}

	// Parse the JSON response from the LLM.
	drafts, parseErr := parseLLMResponse(chatResp.Content)
	retried := false

	// ── Automatic retry on parse failure ──
	// If the model returned unparseable output (e.g. all tokens spent on
	// <think> reasoning), retry once with a stronger "JSON-only" instruction
	// and lower temperature for more deterministic output.
	if parseErr != nil && ctx.Err() == nil {
		slog.WarnContext(r.Context(), "ai_generation: first attempt parse failed, retrying with JSON-only prompt", "error", parseErr)
		retried = true

		retryReq := llm.ChatRequest{
			Model: providerCfg.ModelName,
			Messages: []llm.ChatMessage{
				{Role: "system", Content: "CRITICAL: Output ONLY a raw JSON array. " +
					"No reasoning, no <think> tags, no markdown, no commentary. " +
					"Your entire response must start with [ and end with ]."},
				{Role: "user", Content: prompt},
			},
			Temperature: 0.3,
			MaxTokens:   maxTokens,
		}

		retryResp, retryErr := provider.Chat(ctx, retryReq)
		if retryErr == nil {
			retryDrafts, retryParseErr := parseLLMResponse(retryResp.Content)
			if retryParseErr == nil {
				slog.InfoContext(r.Context(), "ai_generation: retry succeeded", "drafts", len(retryDrafts))
				drafts = retryDrafts
				parseErr = nil
				// Accumulate token usage across both attempts.
				if chatResp.Usage != nil && retryResp.Usage != nil {
					chatResp.Usage.PromptTokens += retryResp.Usage.PromptTokens
					chatResp.Usage.CompletionTokens += retryResp.Usage.CompletionTokens
					chatResp.Usage.TotalTokens += retryResp.Usage.TotalTokens
				} else if retryResp.Usage != nil {
					chatResp.Usage = retryResp.Usage
				}
				if retryResp.FinishReason != "" {
					chatResp.FinishReason = retryResp.FinishReason
				}
			} else {
				slog.WarnContext(r.Context(), "ai_generation: retry also failed to parse", "error", retryParseErr)
			}
		} else {
			slog.WarnContext(r.Context(), "ai_generation: retry LLM call failed", "error", retryErr)
		}
	}

	if parseErr != nil {
		slog.WarnContext(r.Context(), "ai_generation: failed to parse LLM response", "error", parseErr, "raw", chatResp.Content)
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error": "LLM returned an unexpected response format after 2 attempts. Try using a lower coverage level, a different model, or increasing the provider timeout.",
		})
		return
	}

	debug := map[string]interface{}{
		"duration_ms":       time.Since(start).Milliseconds(),
		"model":             chatResp.Model,
		"finish_reason":     chatResp.FinishReason,
		"max_tokens_budget": maxTokens,
		"retried":           retried,
		"provider_label":    providerCfg.Label,
		"provider_type":     providerCfg.ProviderType,
		"request_context":   prompt,
		"template_type":     templateType,
	}
	if chatResp.Usage != nil {
		debug["usage"] = chatResp.Usage
	}

	out := map[string]interface{}{
		"drafts":   drafts,
		"provider": providerCfg.MaskedConfig(),
		"debug":    debug,
	}
	if templateWarning != "" {
		out["template_warning"] = templateWarning
	}
	httpx.JSON(w, http.StatusOK, out)
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

// assemblePrompt builds the final LLM prompt by substituting template variables.
func assemblePrompt(template string, req *models.Requirement, childrenContext, coverageGuidance, detailLevel, additionalInstructions string) string {
	additionalInstr := ""
	if strings.TrimSpace(additionalInstructions) != "" {
		additionalInstr = "Additional Instructions: " + additionalInstructions
	}
	prompt := template
	prompt = strings.ReplaceAll(prompt, "{{COVERAGE}}", coverageGuidance)
	prompt = strings.ReplaceAll(prompt, "{{TITLE}}", req.Title)
	// If the template contains {{CHILDREN}}, substitute it; otherwise append children after description
	if strings.Contains(prompt, "{{CHILDREN}}") {
		prompt = strings.ReplaceAll(prompt, "{{CHILDREN}}", childrenContext)
		prompt = strings.ReplaceAll(prompt, "{{DESCRIPTION}}", req.Description)
	} else {
		prompt = strings.ReplaceAll(prompt, "{{DESCRIPTION}}", req.Description+childrenContext)
	}
	prompt = strings.ReplaceAll(prompt, "{{DETAIL_LEVEL}}", detailLevel)
	prompt = strings.ReplaceAll(prompt, "{{ADDITIONAL_INSTRUCTIONS}}", additionalInstr)
	return prompt
}

// llmDraftShape is the JSON shape the LLM is expected to output per test case.
// Defined at package level so both parseLLMResponse and collectJSONObjects share it.
type llmDraftShape struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
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

	// Strategy A: bare JSON array.
	if strings.HasPrefix(raw, "[") {
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
Return ONLY a valid JSON array — no markdown, no explanation.
[{"name":"...","category":"Functional","description":"...","steps":[{"action":"...","expected_result":"..."}]}]
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
Return ONLY a valid JSON array — no markdown, no explanation.
[{"name":"...","category":"Functional","description":"...","steps":[{"action":"...","expected_result":"..."}]}]`
}

// ────────────────────────────────────────────────────────────────────────────
// US3: Accept generated test cases
// ────────────────────────────────────────────────────────────────────────────

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

	// When grouping by category, cache resolved subfolder IDs so we only
	// create each subfolder once per request.
	subfolderCache := map[string]string{} // category → folder ID

	createdIDs := make([]string, 0, len(req.Tests))
	for _, draft := range req.Tests {
		targetFolderID := req.FolderID

		// Resolve subfolder when grouping is enabled and the test has a category.
		if req.GroupByCategory {
			cat := strings.TrimSpace(draft.Category)
			if cat != "" {
				if cached, ok := subfolderCache[cat]; ok {
					targetFolderID = cached
				} else {
					sub, err := h.store.FindOrCreateSubfolder(req.FolderID, cat)
					if err != nil {
						httpx.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to create subfolder %q: %w", cat, err))
						return
					}
					subfolderCache[cat] = sub.ID
					targetFolderID = sub.ID
				}
			}
		}

		steps := make([]*models.TestStep, len(draft.Steps))
		for i, st := range draft.Steps {
			steps[i] = &models.TestStep{
				Action:         st.Action,
				ExpectedResult: st.ExpectedResult,
				OrderIndex:     i,
			}
		}
		tc := &models.TestCase{
			FolderID:    targetFolderID,
			Name:        draft.Name,
			Description: draft.Description,
			Steps:       steps,
		}
		if err := h.store.CreateTestCase(tc); err != nil {
			httpx.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to create test case %q: %w", draft.Name, err))
			return
		}
		// Link to requirement.
		if _, err := h.store.CreateLink(requirementID, tc.ID); err != nil {
			slog.WarnContext(r.Context(), "ai_generation: failed to link test case to requirement", "test_case_id", tc.ID, "requirement_id", requirementID, "error", err)
		}
		createdIDs = append(createdIDs, tc.ID)
	}

	httpx.JSON(w, http.StatusCreated, map[string]interface{}{
		"created_ids":        createdIDs,
		"count":              len(createdIDs),
		"subfolders_created": len(subfolderCache),
	})
}
