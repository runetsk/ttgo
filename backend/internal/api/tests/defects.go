package tests

import (
	"encoding/json"
	"errors"
	"net/http"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/models"
)

// DismissReverification godoc
//
// @Summary      Dismiss the reverification flag
// @Description  Clears the reverification_flagged state for the given test case.
// @Tags         tests
// @Param        id  path  string  true  "Test case ID"
// @Success      204
// @Failure      500  {object}  object{error=string}
// @Router       /tests/{id}/reverification-flag [delete]
func (h *Handler) DismissReverification(w http.ResponseWriter, r *http.Request) {
	testCaseID := r.PathValue("id")
	if err := h.store.DismissReverification(testCaseID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListTestDefects godoc
//
// @Summary      List defects for a test case
// @Description  Returns all defects linked to the specified test case.
// @Tags         tests
// @Produce      json
// @Param        id  path      string  true  "Test case ID"
// @Success      200  {array}   models.Defect
// @Failure      500  {object}  object{error=string}
// @Router       /tests/{id}/defect-links [get]
func (h *Handler) ListTestDefects(w http.ResponseWriter, r *http.Request) {
	defects, err := h.store.ListDefectsByTestCase(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if defects == nil {
		defects = []models.Defect{}
	}
	httpx.JSON(w, http.StatusOK, defects)
}

// LinkTestDefect godoc
//
// @Summary      Link a defect to a test case
// @Description  Links an existing defect to the specified test case (case-scoped link).
// @Tags         tests
// @Accept       json
// @Produce      json
// @Param        id    path      string                    true  "Test case ID"
// @Param        body  body      models.LinkDefectRequest  true  "Defect to link"
// @Success      201  {object}  models.DefectLink
// @Failure      400  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Failure      409  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /tests/{id}/defect-links [post]
func (h *Handler) LinkTestDefect(w http.ResponseWriter, r *http.Request) {
	testID := r.PathValue("id")
	var req models.LinkDefectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DefectID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "defect_id is required"})
		return
	}
	if _, err := h.store.GetTestCase(testID); err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "test case not found"})
		return
	}
	link, err := h.store.LinkDefectToTestCase(req.DefectID, testID)
	if err != nil {
		if errors.Is(err, models.ErrDuplicateDefectLink) {
			httpx.JSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, link)
}

// UnlinkTestDefect godoc
//
// @Summary      Unlink a defect from a test case
// @Description  Removes the case-scoped defect link from the specified test case.
// @Tags         tests
// @Param        id         path  string  true  "Test case ID"
// @Param        defect_id  path  string  true  "Defect ID"
// @Success      204
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /tests/{id}/defect-links/{defect_id} [delete]
func (h *Handler) UnlinkTestDefect(w http.ResponseWriter, r *http.Request) {
	if err := h.store.UnlinkDefectFromTestCase(r.PathValue("defect_id"), r.PathValue("id")); err != nil {
		if err.Error() == "defect link not found" {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
