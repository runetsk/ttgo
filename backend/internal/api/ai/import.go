package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"
	"ttgo/internal/api/authctx"
	"ttgo/internal/api/httpx"
	"ttgo/internal/importparser"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"log/slog"

	"github.com/google/uuid"
)

// maxImportTestCases is the maximum number of test cases allowed per import session (FR-015).
const maxImportTestCases = 50

// ────────────────────────────────────────────────────────────────────────────
// 014-ai-test-import: Parse and accept imported AI-generated test cases
// ────────────────────────────────────────────────────────────────────────────

// handleParseImport parses raw AI-generated content into structured test cases.
//
// @Summary      Parse import content
// @Description  Parse raw AI-generated or pasted content into structured test cases. Supports JSON, markdown table, numbered list, CSV formats with LLM fallback.
// @Tags         ai-import
// @Accept       json
// @Produce      json
// @Param        body  body  models.ParseImportRequest  true  "Import content and optional hints"
// @Success      200  {object}  models.ParseImportResponse
// @Failure      400  {object}  map[string]string
// @Failure      422  {object}  map[string]string
// @Router       /import/parse [post]
// @Security     BearerAuth
func (h *Handler) ParseImport(w http.ResponseWriter, r *http.Request) {
	var req models.ParseImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "content is required"})
		return
	}
	// Bound the input before the regex-heavy deterministic parsers / LLM fallback (F-058).
	if len(req.Content) > 1<<20 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "content too large (max 1 MB)"})
		return
	}

	testCases, unparseable, detectedFormat, err := importparser.ParseImportContent(req.Content, req.FormatHint, parseLLMResponse)

	// If deterministic parsers failed, attempt LLM-powered fallback.
	var llmDebug map[string]interface{}
	if err != nil || len(testCases) == 0 {
		llmCases, debug, llmErr := h.llmParseImportFallback(r.Context(), req.Content)
		if llmErr != nil {
			slog.WarnContext(r.Context(), "ai_import: LLM fallback also failed", "error", llmErr)
			httpx.JSON(w, http.StatusUnprocessableEntity, map[string]string{
				"error": "Unable to parse any test cases from the provided content. Supported formats: JSON array, markdown table, numbered/bulleted list, CSV. AI-powered parsing also failed: " + llmErr.Error(),
			})
			return
		}
		if len(llmCases) == 0 {
			httpx.JSON(w, http.StatusUnprocessableEntity, map[string]string{
				"error": "Unable to parse any test cases from the provided content. Supported formats: JSON array, markdown table, numbered/bulleted list, CSV. AI-powered parsing returned no results.",
			})
			return
		}
		testCases = llmCases
		detectedFormat = "ai"
		unparseable = nil
		llmDebug = debug
	}

	// Sanitize all text fields with bluemonday (FR-011).
	// Unescape HTML entities afterward so the review panel displays clean plain text
	// (bluemonday encodes " → &#34; etc. even for plain-text input).
	p := h.sanitizer
	for i := range testCases {
		testCases[i].Name = html.UnescapeString(p.Sanitize(testCases[i].Name))
		testCases[i].Description = html.UnescapeString(p.Sanitize(testCases[i].Description))
		testCases[i].Category = html.UnescapeString(p.Sanitize(testCases[i].Category))
		for j := range testCases[i].Steps {
			testCases[i].Steps[j].Action = html.UnescapeString(p.Sanitize(testCases[i].Steps[j].Action))
			testCases[i].Steps[j].ExpectedResult = html.UnescapeString(p.Sanitize(testCases[i].Steps[j].ExpectedResult))
		}
	}

	// Assign temp_id to each parsed test case.
	for i := range testCases {
		testCases[i].TempID = uuid.New().String()
	}

	// Cap at 50 test cases (FR-015).
	totalFound := len(testCases)
	truncated := false
	if totalFound > maxImportTestCases {
		testCases = testCases[:maxImportTestCases]
		truncated = true
	}

	// Duplicate name detection (FR-016).
	var duplicateNames []string
	if req.FolderID != "" {
		duplicateNames = h.findDuplicateNames(testCases, req.FolderID)
	}

	resp := models.ParseImportResponse{
		DetectedFormat: detectedFormat,
		TestCases:      testCases,
		Unparseable:    unparseable,
		DuplicateNames: duplicateNames,
		TotalFound:     totalFound,
		Truncated:      truncated,
		Debug:          llmDebug,
	}
	if resp.Unparseable == nil {
		resp.Unparseable = []models.UnparseableItem{}
	}
	if resp.DuplicateNames == nil {
		resp.DuplicateNames = []string{}
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// findDuplicateNames queries existing test cases in the target folder and returns
// names from the parsed set that match (case-insensitive).
func (h *Handler) findDuplicateNames(parsed []models.GeneratedTestCase, folderID string) []string {
	existing, err := h.store.ListTestCases(store.TestCaseFilter{FolderIDs: []string{folderID}})
	if err != nil {
		slog.Warn("ai_import: failed to list test cases for duplicate check", "error", err)
		return nil
	}
	existingNames := make(map[string]bool, len(existing))
	for _, tc := range existing {
		existingNames[strings.ToLower(tc.Name)] = true
	}
	var dupes []string
	seen := make(map[string]bool)
	for _, tc := range parsed {
		lower := strings.ToLower(tc.Name)
		if existingNames[lower] && !seen[lower] {
			dupes = append(dupes, tc.Name)
			seen[lower] = true
		}
	}
	return dupes
}

// handleAcceptImport accepts parsed and reviewed test cases, creating them in the target folder.
//
// @Summary      Accept imported test cases
// @Description  Accept reviewed test cases from a parse session and persist them to the target folder. Optionally links them to a requirement.
// @Tags         ai-import
// @Accept       json
// @Produce      json
// @Param        body  body  models.AcceptImportRequest  true  "Tests to accept with target folder"
// @Success      201  {object}  models.AcceptImportResponse
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /import/accept [post]
// @Security     BearerAuth
func (h *Handler) AcceptImport(w http.ResponseWriter, r *http.Request) {
	var req models.AcceptImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.FolderID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "folder_id is required"})
		return
	}
	if len(req.Tests) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "no tests to accept"})
		return
	}

	// Verify folder exists.
	if _, err := h.store.GetFolder(req.FolderID); err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "folder not found"})
		return
	}

	// Verify requirement exists if provided.
	if req.RequirementID != "" {
		if _, err := h.store.GetRequirement(req.RequirementID); err != nil {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "requirement not found"})
			return
		}
	}

	// Sanitize all fields with bluemonday before persistence (defense-in-depth).
	pol := h.sanitizer

	createdIDs := make([]string, 0, len(req.Tests))
	for _, draft := range req.Tests {
		steps := make([]*models.TestStep, len(draft.Steps))
		for i, st := range draft.Steps {
			steps[i] = &models.TestStep{
				Action:         httpx.NormalizeEmptyHTML(pol, st.Action),
				ExpectedResult: httpx.NormalizeEmptyHTML(pol, st.ExpectedResult),
				OrderIndex:     i,
			}
		}
		tc := &models.TestCase{
			FolderID:    req.FolderID,
			Name:        html.UnescapeString(pol.Sanitize(draft.Name)),
			Description: httpx.NormalizeEmptyHTML(pol, draft.Description),
			Steps:       steps,
		}
		if err := h.store.CreateTestCase(tc); err != nil {
			httpx.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to create test case %q: %w", draft.Name, err))
			return
		}
		// Optional: link to requirement (US3 backend foundation).
		if req.RequirementID != "" {
			if _, err := h.store.CreateLink(req.RequirementID, tc.ID); err != nil {
				slog.WarnContext(r.Context(), "ai_import: failed to link test case to requirement", "test_case_id", tc.ID, "requirement_id", req.RequirementID, "error", err)
			}
		}
		createdIDs = append(createdIDs, tc.ID)
	}

	// Audit log entry (Constitution Principle V).
	user := authctx.UserFromRequest(r)
	userID := ""
	if user != nil {
		userID = user.ID
	}
	_ = h.store.CreateAuditLog(&models.AuditLog{
		ID:        uuid.New().String(),
		Action:    fmt.Sprintf("ai_import:accept:folder:%s:count:%d", req.FolderID, len(createdIDs)),
		UserID:    userID,
		Timestamp: time.Now(),
	})

	resp := models.AcceptImportResponse{
		CreatedIDs: createdIDs,
		Count:      len(createdIDs),
	}
	if req.RequirementID != "" {
		resp.LinkedTo = req.RequirementID
	}
	httpx.JSON(w, http.StatusCreated, resp)
}

// ────────────────────────────────────────────────────────────────────────────
// LLM-powered fallback parser
// ────────────────────────────────────────────────────────────────────────────

// llmParseImportFallback uses a configured LLM provider to extract structured
// test cases from unstructured content when deterministic parsers fail.
// Returns drafts and debug info (duration, model, usage, etc.).
func (h *Handler) llmParseImportFallback(ctx context.Context, rawContent string) ([]models.GeneratedTestCase, map[string]interface{}, error) {
	start := time.Now()

	// Find an enabled provider (prefer default).
	cfgs, err := h.store.GetAllProviderConfigs()
	if err != nil {
		return nil, nil, fmt.Errorf("no LLM providers available")
	}
	var providerCfg *models.LLMProviderConfig
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
		return nil, nil, fmt.Errorf("no enabled LLM provider configured")
	}

	provider, err := llm.NewProvider(providerCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize LLM provider: %w", err)
	}

	timeout := time.Duration(providerCfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	llmCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Truncate content to avoid blowing up context windows.
	content := rawContent
	if len(content) > 15000 {
		content = content[:15000] + "\n\n[... content truncated ...]"
	}

	systemPrompt := `You are a test case extraction engine. Your job is to extract software test cases from ANY format of text input.

Return ONLY a valid JSON array. No markdown fences, no explanation, no commentary.
Start your response with [ and end with ].

Each test case object must have:
- "name": string (short descriptive name)
- "description": string (brief description, can be empty)
- "category": string (e.g. "Functional", "Negative", "Boundary", "Edge Case", "Security", etc.)
- "steps": array of {"action": "...", "expected_result": "..."}

If the input contains test scenarios, user stories, acceptance criteria, requirements, or any testable content — extract test cases from it.
If you can identify specific steps, include them. If not, create reasonable test steps based on the content.
Always produce at least 1 test case if there is any testable content.`

	userPrompt := fmt.Sprintf("Extract test cases from the following content:\n\n%s", content)

	chatReq := llm.ChatRequest{
		Model: providerCfg.ModelName,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.3,
		MaxTokens:   4096,
	}

	chatResp, err := provider.Chat(llmCtx, chatReq)
	if err != nil {
		if llmCtx.Err() == context.DeadlineExceeded {
			return nil, nil, fmt.Errorf("LLM request timed out after %d seconds", providerCfg.TimeoutSeconds)
		}
		return nil, nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// Reuse the existing robust JSON parser.
	drafts, parseErr := parseLLMResponse(chatResp.Content)
	if parseErr != nil {
		return nil, nil, fmt.Errorf("LLM returned unparseable response: %w", parseErr)
	}

	// Build debug info (same structure as AI generation feedback).
	requestContext := fmt.Sprintf("[System]\n%s\n\n[User]\n%s", systemPrompt, userPrompt)
	debug := map[string]interface{}{
		"duration_ms":       time.Since(start).Milliseconds(),
		"model":             chatResp.Model,
		"finish_reason":     chatResp.FinishReason,
		"max_tokens_budget": 4096,
		"provider_label":    providerCfg.Label,
		"provider_type":     providerCfg.ProviderType,
		"request_context":   requestContext,
	}
	if chatResp.Usage != nil {
		debug["usage"] = chatResp.Usage
	}

	slog.Info("ai_import: LLM fallback extracted test cases", "count", len(drafts))
	return drafts, debug, nil
}
