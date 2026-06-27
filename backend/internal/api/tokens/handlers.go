package tokens

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/store"
)

// Handler serves token endpoints.
type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{store: s}
}

// handleCreateToken handles POST /api/tokens
//
//	@Summary		Create an API token
//	@Description	Create a new API token with the given description and scope
//	@Tags			tokens
//	@Accept			json
//	@Produce		json
//	@Param			body	body		object{description=string,scope=string}	true	"Token description (required) and scope (read|write)"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Router			/tokens [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Description string `json:"description"`
		Scope       string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.Scope != "read" && req.Scope != "write" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "scope must be 'read' or 'write'"})
		return
	}
	if req.Description == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "description is required"})
		return
	}
	if len(req.Description) > 200 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "description must be at most 200 characters"})
		return
	}

	token, rawToken, err := h.store.CreateToken(req.Description, req.Scope, nil)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to create token", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	httpx.JSON(w, http.StatusCreated, map[string]interface{}{
		"id":          token.ID,
		"token":       rawToken, // shown once only
		"description": token.Description,
		"scope":       token.Scope,
		"created_at":  token.CreatedAt,
	})
}

// handleListTokens handles GET /api/tokens
//
//	@Summary		List API tokens
//	@Description	Return all API tokens
//	@Tags			tokens
//	@Produce		json
//	@Success		200	{object}	map[string]interface{}	"tokens array and total count"
//	@Failure		500	{object}	map[string]string
//	@Router			/tokens [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	tokens, err := h.store.ListTokens()
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list tokens", "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"tokens": tokens,
		"total":  len(tokens),
	})
}

// handleDeleteToken handles DELETE /api/tokens/{id}
//
//	@Summary		Delete an API token
//	@Description	Delete an existing API token by ID
//	@Tags			tokens
//	@Param			id	path	string	true	"Token ID"
//	@Success		204
//	@Failure		400	{object}	map[string]string
//	@Failure		500	{object}	map[string]string
//	@Router			/tokens/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	if err := h.store.DeleteToken(id); err != nil {
		slog.ErrorContext(r.Context(), "failed to delete token", "id", id, "error", err)
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
