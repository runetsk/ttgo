package webhooks

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
)

type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{store: s}
}

// handleCreateWebhook handles POST /api/webhooks
//
//	@Summary		Create a webhook
//	@Description	Create a new webhook configuration with the given URL, description, and event type
//	@Tags			webhooks
//	@Accept			json
//	@Produce		json
//	@Param			body	body		object{url=string,description=string,event_type=string}	true	"Webhook URL (required), description, and event_type"
//	@Success		201		{object}	interface{}
//	@Failure		400		{object}	map[string]string
//	@Router			/webhooks [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL         string `json:"url"`
		Description string `json:"description"`
		EventType   string `json:"event_type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.URL == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}
	if req.EventType == "" {
		req.EventType = "run.completed"
	}

	wh, err := h.store.CreateWebhookConfig(req.URL, req.Description, req.EventType)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create webhook", "error", err)
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	// Return the signing secret ONCE so the receiver can verify X-TTGO-Signature
	// (it is json:"-" everywhere else and never shown again) (F-066).
	httpx.JSON(w, http.StatusCreated, struct {
		*models.WebhookConfig
		Secret string `json:"secret"`
	}{wh, wh.Secret})
}

// handleListWebhooks handles GET /api/webhooks
//
//	@Summary		List webhooks
//	@Description	Return all webhook configurations
//	@Tags			webhooks
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"webhooks array and total count"
//	@Failure		500	{object}	map[string]string
//	@Router			/webhooks [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	webhooks, err := h.store.ListWebhookConfigs()
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list webhooks", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"webhooks": webhooks,
		"total":    len(webhooks),
	})
}

// handleDeleteWebhook handles DELETE /api/webhooks/{id}
//
//	@Summary		Delete a webhook
//	@Description	Delete an existing webhook configuration by ID
//	@Tags			webhooks
//	@Param			id	path	string	true	"Webhook ID"
//	@Success		204
//	@Failure		400	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Router			/webhooks/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	if err := h.store.DeleteWebhookConfig(id); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete webhook", "id", id, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleGetWebhookLogs handles GET /api/webhooks/{id}/logs
//
//	@Summary		Get webhook dispatch logs
//	@Description	Return paginated dispatch logs for a webhook
//	@Tags			webhooks
//	@Produce		json
//	@Param			id		path		string	true	"Webhook ID"
//	@Param			limit	query		int		false	"Maximum number of logs to return"	default(50)
//	@Param			offset	query		int		false	"Number of logs to skip"			default(0)
//	@Success		200		{object}	map[string]interface{}	"logs array and total count"
//	@Failure		500		{object}	map[string]string
//	@Router			/webhooks/{id}/logs [get]
func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = httpx.ClampLimit(v, 50, 200) // bound page size (F-043)
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = httpx.ClampOffset(v)
		}
	}

	logs, total, err := h.store.GetDispatchLogs(id, limit, offset)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to get webhook logs", "id", id, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"total": total,
	})
}
