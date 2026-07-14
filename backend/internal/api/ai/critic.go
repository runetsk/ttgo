package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"ttgo/pkg/tracker/aigen"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
)

const criticSystemMessage = "You are a strict test-case reviewer. You receive a requirement and a JSON array " +
	"of generated test-case drafts. Report SEMANTIC weaknesses only (irrelevance to the requirement, hidden " +
	"duplication, untestable outcomes, missing negative angles). Respond with ONLY a JSON object " +
	`{"findings":[{"draft_index":0,"dimension":"relevance","message":"..."}]}. An empty findings array is a valid answer.`

// criticResponseFormat mirrors resolveResponseFormat for the critic schema.
func criticResponseFormat(cfg *models.LLMProviderConfig) *llm.ResponseFormat {
	switch cfg.ProviderType {
	case "openai", "gemini":
		return &llm.ResponseFormat{Type: "json_schema", SchemaName: aigen.CriticSchemaName, Schema: aigen.CriticSchemaJSON}
	default:
		return &llm.ResponseFormat{Type: "json_object"}
	}
}

// runCriticPass reviews the freshly persisted drafts with one extra LLM call
// and appends a "critic" quality dimension per flagged draft. Failures are
// soft: they return a warning string and never fail the run. Usage is folded
// into chatUsage-style totals by the caller via the returned response.
func (h *Handler) runCriticPass(ctx context.Context, provider llm.Provider, cfg *models.LLMProviderConfig, requirementTitle string, drafts []models.GeneratedTestCase, rows []*models.AIGeneratedDraft) (*llm.ChatResponse, string) {
	payload, err := json.Marshal(drafts)
	if err != nil {
		return nil, "critic skipped: could not serialize drafts"
	}
	req := llm.ChatRequest{
		Model: cfg.ModelName,
		Messages: []llm.ChatMessage{
			{Role: "system", Content: criticSystemMessage},
			{Role: "user", Content: fmt.Sprintf("Requirement: %s\n\nDrafts JSON:\n%s", requirementTitle, payload)},
		},
		Temperature:    0.2,
		MaxTokens:      2048,
		ResponseFormat: criticResponseFormat(cfg),
	}
	resp, _, callErr := llm.ChatWithRetry(ctx, provider, req, llm.RetryOptions{})
	if callErr != nil {
		return nil, fmt.Sprintf("critic pass failed: %s", llm.Classify(callErr))
	}
	findings, parseErr := aigen.ParseCriticResponse(resp.Content)
	if parseErr != nil {
		return resp, "critic pass returned unparseable output"
	}

	perDraft := map[int][]aigen.Finding{}
	for _, f := range findings {
		if f.DraftIndex < 0 || f.DraftIndex >= len(rows) || f.Message == "" {
			continue
		}
		perDraft[f.DraftIndex] = append(perDraft[f.DraftIndex], aigen.Finding{
			Field: "draft", Code: "critic_" + f.Dimension, Message: f.Message, Severity: aigen.SeverityWarning,
		})
	}
	for idx, fs := range perDraft {
		var dims []aigen.QualityDimension
		if rows[idx].QualityJSON != "" {
			_ = json.Unmarshal([]byte(rows[idx].QualityJSON), &dims)
		}
		dims = append(dims, aigen.QualityDimension{Key: "critic", Label: aigen.QualityDimensionLabels["critic"], Findings: fs})
		qb, _ := json.Marshal(dims)
		rows[idx].QualityJSON = string(qb)
		if err := h.store.UpdateDraftQuality(rows[idx].ID, string(qb)); err != nil {
			slog.Warn("ai_generation: critic quality persist failed", "draft_id", rows[idx].ID, "error", err)
		}
	}
	if err := h.store.AppendGenerationEvent(&models.AIGenerationEvent{
		RunID: rows[0].RunID, EventType: models.AIGenEventValidated,
		MetadataJSON: fmt.Sprintf(`{"critic":true,"critic_findings":%d}`, len(findings)),
	}); err != nil {
		slog.Warn("ai_generation: critic event failed", "error", err)
	}
	return resp, ""
}
