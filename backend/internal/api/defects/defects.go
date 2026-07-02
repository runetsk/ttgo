package defects

import (
	"encoding/json"
	"net/http"
	"strings"

	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
)

// List godoc
//
// @Summary      List defects
// @Description  Returns native defects with optional filtering by status, severity, or search query.
// @Tags         defects
// @Produce      json
// @Param        status    query     string  false  "Filter by status: open | closed"
// @Param        severity  query     string  false  "Filter by severity: critical | major | minor | trivial"
// @Param        q         query     string  false  "Full-text search query"
// @Success      200  {array}   models.Defect
// @Failure      500  {object}  object{error=string}
// @Router       /defects [get]
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	defects, err := h.store.ListDefects(q.Get("status"), q.Get("severity"), q.Get("q"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if defects == nil {
		defects = []models.Defect{}
	}
	httpx.JSON(w, http.StatusOK, defects)
}

// Create godoc
//
// @Summary      Create a defect
// @Description  Creates a new native defect.
// @Tags         defects
// @Accept       json
// @Produce      json
// @Param        body  body      models.CreateDefectRequest  true  "Defect to create"
// @Success      201  {object}  models.Defect
// @Failure      400  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /defects [post]
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateDefectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if msg := ValidateCreate(req); msg != "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	d := DefectFromCreate(req)
	if err := h.store.CreateDefect(d); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, d)
}

// Update godoc
//
// @Summary      Update a defect
// @Description  Partially updates a defect (nil fields are left unchanged).
// @Tags         defects
// @Accept       json
// @Produce      json
// @Param        id    path      string                      true  "Defect ID"
// @Param        body  body      models.UpdateDefectRequest  true  "Fields to update"
// @Success      200  {object}  models.Defect
// @Failure      400  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /defects/{id} [patch]
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var req models.UpdateDefectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if msg := validateUpdate(req); msg != "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	d, err := h.store.UpdateDefect(r.PathValue("id"), req)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if d == nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "defect not found"})
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

// Delete godoc
//
// @Summary      Delete a defect
// @Description  Permanently deletes a defect and all its links.
// @Tags         defects
// @Param        id  path  string  true  "Defect ID"
// @Success      204
// @Failure      500  {object}  object{error=string}
// @Router       /defects/{id} [delete]
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.store.DeleteDefect(r.PathValue("id")); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ValidateCreate / DefectFromCreate are exported so the runs package (create-and-link) reuses them.
func ValidateCreate(req models.CreateDefectRequest) string {
	if strings.TrimSpace(req.Title) == "" {
		return "title is required"
	}
	if len(req.Title) > 500 {
		return "title too long"
	}
	if req.Severity != "" && !validSeverity[req.Severity] {
		return "invalid severity"
	}
	if req.Status != "" && !validStatus[req.Status] {
		return "invalid status"
	}
	if err := ValidExternalURL(req.ExternalURL); err != nil {
		return err.Error()
	}
	return ""
}

func DefectFromCreate(req models.CreateDefectRequest) *models.Defect {
	return &models.Defect{
		Title: strings.TrimSpace(req.Title), Description: req.Description, Severity: req.Severity, Status: req.Status,
		ExternalProvider: req.ExternalProvider, ExternalKey: req.ExternalKey, ExternalURL: strings.TrimSpace(req.ExternalURL),
	}
}

// AffectedTests godoc
//
// @Summary      List a defect's affected test cases
// @Description  Returns the distinct test cases linked to the defect (directly or via a run result), ordered by name.
// @Tags         defects
// @Produce      json
// @Param        id  path  string  true  "Defect ID"
// @Success      200  {array}   store.AffectedTestCase
// @Failure      500  {object}  object{error=string}
// @Router       /defects/{id}/tests [get]
func (h *Handler) AffectedTests(w http.ResponseWriter, r *http.Request) {
	tests, err := h.store.ListAffectedTestCases(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if tests == nil {
		tests = []store.AffectedTestCase{}
	}
	httpx.JSON(w, http.StatusOK, tests)
}

func validateUpdate(req models.UpdateDefectRequest) string {
	if req.Severity != nil && !validSeverity[*req.Severity] {
		return "invalid severity"
	}
	if req.Status != nil && !validStatus[*req.Status] {
		return "invalid status"
	}
	if req.Title != nil && strings.TrimSpace(*req.Title) == "" {
		return "title cannot be empty"
	}
	if req.ExternalURL != nil {
		if err := ValidExternalURL(*req.ExternalURL); err != nil {
			return err.Error()
		}
	}
	return ""
}
