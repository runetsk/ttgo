package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
)

// GetAIBudgetSettings returns the soft budget configuration.
//
// @Summary  Get AI budget settings
// @Tags     ai-settings
// @Produce  json
// @Success  200 {object} models.AIBudgetSettings
// @Router   /settings/ai-budgets [get]
// @Security BearerAuth
func (h *Handler) GetAIBudgetSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetOrCreateAIBudgetSettings()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cfg)
}

// UpdateAIBudgetSettings updates the soft budgets (admin).
//
// @Summary  Update AI budget settings
// @Tags     ai-settings
// @Accept   json
// @Produce  json
// @Param    body body object true "per_request_usd, monthly_usd (0 = off)"
// @Success  200 {object} models.AIBudgetSettings
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
	httpx.JSON(w, http.StatusOK, cfg)
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
		now := time.Now().UTC()
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		spent, err := h.store.SumEstimatedCostSince(monthStart)
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
