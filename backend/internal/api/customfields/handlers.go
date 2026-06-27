package customfields

import (
	"encoding/json"
	"net/http"
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

// handleGetCustomFields godoc
// @Summary      List custom field definitions
// @Description  Returns all custom field definitions
// @Tags         custom-fields
// @Accept       json
// @Produce      json
// @Success      200  {array}   models.CustomFieldDefinition
// @Failure      500  {object}  map[string]string
// @Router       /custom-fields [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	defs, err := h.store.ListCustomFieldDefinitions()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, defs)
}

// handleCreateCustomField godoc
// @Summary      Create a custom field definition
// @Description  Creates a new custom field definition
// @Tags         custom-fields
// @Accept       json
// @Produce      json
// @Param        body  body      models.CreateCustomFieldRequest  true  "Custom field definition payload"
// @Success      201  {object}  models.CustomFieldDefinition
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /custom-fields [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateCustomFieldRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if req.Name == "" || req.Type == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Name and Type are required"})
		return
	}
	if len(req.Name) > 200 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Name must be at most 200 characters"})
		return
	}
	// Enforce the type enum and require a usable options array for SELECT (F-032).
	validTypes := map[models.CustomFieldType]bool{"TEXT": true, "SELECT": true, "NUMBER": true, "DATE": true, "CHECKBOX": true}
	if !validTypes[req.Type] {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "type must be one of TEXT, SELECT, NUMBER, DATE, CHECKBOX"})
		return
	}
	if req.Type == "SELECT" {
		var opts []string
		if len(req.Options) == 0 || json.Unmarshal(req.Options, &opts) != nil || len(opts) == 0 {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "SELECT fields require a non-empty options array of strings"})
			return
		}
	}

	def := &models.CustomFieldDefinition{
		Name:        req.Name,
		Type:        req.Type,
		IsMandatory: req.IsMandatory,
		Options:     req.Options,
	}
	if err := h.store.CreateCustomFieldDefinition(def); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, def)
}

// handleDeleteCustomField godoc
// @Summary      Delete a custom field definition
// @Description  Deletes a custom field definition by ID
// @Tags         custom-fields
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Custom field definition ID"
// @Success      204
// @Failure      500  {object}  map[string]string
// @Router       /custom-fields/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteCustomFieldDefinition(id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
