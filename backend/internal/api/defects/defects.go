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
// @Param        status    query     string  false  "Filter by status: open | fixed | closed"
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
	if msg := ValidateCreate(h.store, req); msg != "" {
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
	// assignee_id is passed to the store as sent: "" clears, nil leaves it unchanged.
	if msg := ValidateAssignee(h.store, req.AssigneeID); msg != "" {
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

// BulkUpdate godoc
//
// @Summary      Bulk update defects
// @Description  Applies the same status, severity and/or assignee to many defects in one call. Omitted (null) fields are left unchanged; assignee_id "" clears the assignee across the whole selection. Unknown ids are tolerated — they simply match nothing — and the response carries the defects that exist, enriched with assignee_name and linked_test_count exactly like GET /defects. A request that sets no field at all is a read-back: nothing is written and updated_at is left alone, because that column is the staleness signal.
// @Tags         defects
// @Accept       json
// @Produce      json
// @Param        body  body      models.BulkUpdateDefectsRequest  true  "Defect IDs and the fields to apply"
// @Security     BearerAuth
// @Success      200  {array}   models.Defect
// @Failure      400  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /defects/bulk-update [post]
func (h *Handler) BulkUpdate(w http.ResponseWriter, r *http.Request) {
	var req models.BulkUpdateDefectsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if len(req.IDs) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "ids are required"})
		return
	}
	if len(req.IDs) > httpx.MaxBulkIDs {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many ids (max 500 per request)"})
		return
	}
	if msg := validateBulkUpdate(req); msg != "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	// same rule as Update: "" is a valid unassign, a non-empty id must name an active user.
	if msg := ValidateAssignee(h.store, req.AssigneeID); msg != "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	updated, err := h.store.BulkUpdateDefects(req.IDs, req)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if updated == nil {
		updated = []models.Defect{}
	}
	httpx.JSON(w, http.StatusOK, updated)
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
// The assignee check lives here rather than in the handler so CreateAndLinkResultDefect cannot
// bypass it.
func ValidateCreate(s *store.Store, req models.CreateDefectRequest) string {
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
	return ValidateAssignee(s, req.AssigneeID)
}

func DefectFromCreate(req models.CreateDefectRequest) *models.Defect {
	return &models.Defect{
		Title: strings.TrimSpace(req.Title), Description: req.Description, Severity: req.Severity, Status: req.Status,
		AssigneeID:       normalizeAssignee(req.AssigneeID),
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

// validateBulkUpdate checks the value fields of a bulk request against the same tables as the
// single-defect update, so a selection can never be moved to a status or severity that PATCH
// would refuse. The assignee is validated separately, against the store.
func validateBulkUpdate(req models.BulkUpdateDefectsRequest) string {
	if req.Severity != nil && !validSeverity[*req.Severity] {
		return "invalid severity"
	}
	if req.Status != nil && !validStatus[*req.Status] {
		return "invalid status"
	}
	return ""
}
