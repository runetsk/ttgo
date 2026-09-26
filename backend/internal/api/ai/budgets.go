package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
)

// aiBudgetResponse is the budget settings plus the estimated spend since the start of the
// month, in total and split between test generation and failure analysis, so Settings → AI can
// show spend against the monthly budget.
type aiBudgetResponse struct {
	*models.AIBudgetSettings
	MonthSpentUSD           float64 `json:"month_spent_usd"`
	MonthSpentGenerationUSD float64 `json:"month_spent_generation_usd"`
	MonthSpentAnalysisUSD   float64 `json:"month_spent_analysis_usd"`
}

func (h *Handler) budgetResponse(cfg *models.AIBudgetSettings) (aiBudgetResponse, error) {
	since := store.MonthStartUTC(time.Now())
	gen, err := h.store.SumGenerationCostSince(since)
	if err != nil {
		return aiBudgetResponse{}, err
	}
	analysis, err := h.store.SumAnalysisCostSince(since)
	if err != nil {
		return aiBudgetResponse{}, err
	}
	return aiBudgetResponse{AIBudgetSettings: cfg, MonthSpentUSD: gen + analysis,
		MonthSpentGenerationUSD: gen, MonthSpentAnalysisUSD: analysis}, nil
}

// GetAIBudgetSettings returns the soft budget configuration.
//
// @Summary  Get AI budget settings
// @Tags     ai-settings
// @Produce  json
// @Success  200 {object} models.AIBudgetSettings "settings plus month_spent_usd (estimated spend since the 1st, UTC) split into month_spent_generation_usd and month_spent_analysis_usd"
// @Router   /settings/ai-budgets [get]
// @Security BearerAuth
func (h *Handler) GetAIBudgetSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetOrCreateAIBudgetSettings()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	resp, err := h.budgetResponse(cfg)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// UpdateAIBudgetSettings updates the soft budgets (admin).
//
// @Summary  Update AI budget settings
// @Tags     ai-settings
// @Accept   json
// @Produce  json
// @Param    body body object true "per_request_usd, monthly_usd (0 = off)"
// @Success  200 {object} models.AIBudgetSettings "settings plus month_spent_usd (estimated spend since the 1st, UTC) split into month_spent_generation_usd and month_spent_analysis_usd"
// @Router   /settings/ai-budgets [put]
// @Security BearerAuth
func (h *Handler) UpdateAIBudgetSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PerRequestUSD *float64 `json:"per_request_usd"`
		MonthlyUSD    *float64 `json:"monthly_usd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	updates := map[string]interface{}{}
	if req.PerRequestUSD != nil {
		if *req.PerRequestUSD < 0 {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "per_request_usd must be >= 0"})
			return
		}
		updates["per_request_usd"] = *req.PerRequestUSD
	}
	if req.MonthlyUSD != nil {
		if *req.MonthlyUSD < 0 {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "monthly_usd must be >= 0"})
			return
		}
		updates["monthly_usd"] = *req.MonthlyUSD
	}
	cfg, err := h.store.UpdateAIBudgetSettings(updates)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	resp, err := h.budgetResponse(cfg)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// checkBudget estimates the upcoming call's worst-case cost and returns a
// 409 payload when a soft budget would be exceeded and the caller has not
// acknowledged. nil = proceed. Unpriced providers skip the check (cost is
// unknowable); budgets never mutate the request (spec).
func (h *Handler) checkBudget(cfg *models.LLMProviderConfig, promptChars, maxCompletionTokens int, acknowledged bool) map[string]interface{} {
	if acknowledged || cfg == nil {
		return nil
	}
	estimate := llm.EstimateCostUSD(promptChars/4, maxCompletionTokens,
		cfg.PromptPricePerMTok, cfg.CompletionPricePerMTok)
	if estimate == nil {
		return nil
	}
	budgets, err := h.store.GetOrCreateAIBudgetSettings()
	if err != nil {
		return nil // fail open: budgets are soft
	}
	if budgets.PerRequestUSD > 0 && *estimate > budgets.PerRequestUSD {
		return map[string]interface{}{
			"error":    fmt.Sprintf("estimated cost $%.4f exceeds the per-request budget $%.2f; resend with acknowledge_budget=true to proceed", *estimate, budgets.PerRequestUSD),
			"category": "budget", "scope": "request",
			"estimated_cost_usd": *estimate, "budget_usd": budgets.PerRequestUSD,
		}
	}
	if budgets.MonthlyUSD > 0 {
		spent, err := h.store.SumEstimatedCostSince(store.MonthStartUTC(time.Now()))
		if err == nil && spent+*estimate > budgets.MonthlyUSD {
			return map[string]interface{}{
				"error":    fmt.Sprintf("this call (~$%.4f) would exceed the monthly budget $%.2f (spent $%.4f); resend with acknowledge_budget=true to proceed", *estimate, budgets.MonthlyUSD, spent),
				"category": "budget", "scope": "month",
				"estimated_cost_usd": *estimate, "budget_usd": budgets.MonthlyUSD, "month_spent_usd": spent,
			}
		}
	}
	return nil
}
