package ai

import (
	"encoding/json"
	"net/http"
	"ttgo/internal/api/httpx"
)

// GetAIFeatureSettings returns the global AI master switch. Auth: read.
func (h *Handler) GetAIFeatureSettings(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetOrCreateAIFeatureSettings()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cfg)
}

// UpdateAIFeatureSettings flips the global AI master switch. Auth: admin.
func (h *Handler) UpdateAIFeatureSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.Enabled == nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "enabled is required"})
		return
	}
	cfg, err := h.store.UpdateAIFeatureSettings(*req.Enabled)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, cfg)
}
