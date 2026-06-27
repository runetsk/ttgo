package categories

import (
	"encoding/json"
	"fmt"
	"net/http"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/store"
)

type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{store: s}
}

// handleGetCategories handles GET /api/categories
//
// @Summary      List test categories
// @Description  Returns a paginated list of test categories
// @Tags         categories
// @Produce      json
// @Param        limit   query     int  false  "Maximum number of categories to return"  default(10)
// @Param        offset  query     int  false  "Number of categories to skip"            default(0)
// @Success      200     {object}  map[string]interface{}  "categories array and total count"
// @Failure      500     {object}  map[string]interface{}
// @Router       /categories [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit := 10
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}
	search := r.URL.Query().Get("q")

	categories, total, err := h.store.ListCategories(limit, offset, search)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"categories": categories,
		"total":      total,
	})
}

// handleCreateCategory handles POST /api/categories
//
// @Summary      Create a test category
// @Description  Creates a new test category with the given name and description
// @Tags         categories
// @Accept       json
// @Produce      json
// @Param        body  body      object{name=string,description=string}  true  "Category creation payload"
// @Success      201   {object}  map[string]interface{}  "Created category object"
// @Failure      400   {object}  map[string]interface{}
// @Failure      500   {object}  map[string]interface{}
// @Router       /categories [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if req.Name == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Name is required"})
		return
	}
	if len(req.Name) > 255 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Name must be at most 255 characters"})
		return
	}

	category, err := h.store.CreateCategory(req.Name, req.Description)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, category)
}

// handleDeleteCategory handles DELETE /api/categories/{id}
//
// @Summary      Delete a test category
// @Description  Deletes the test category with the given ID
// @Tags         categories
// @Param        id  path  string  true  "Category ID"
// @Success      204
// @Failure      500  {object}  map[string]interface{}
// @Router       /categories/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteCategories([]string{id}); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleBulkDeleteCategories handles POST /api/categories/bulk-delete
//
// @Summary      Bulk delete test categories
// @Description  Deletes multiple test categories by their IDs
// @Tags         categories
// @Accept       json
// @Param        body  body  object{ids=[]string}  true  "List of category IDs to delete"
// @Success      204
// @Failure      400  {object}  map[string]interface{}
// @Failure      500  {object}  map[string]interface{}
// @Router       /categories/bulk-delete [post]
func (h *Handler) BulkDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if len(req.IDs) > httpx.MaxBulkIDs {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many ids (max 500 per request)"})
		return
	}
	if err := h.store.DeleteCategories(req.IDs); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
