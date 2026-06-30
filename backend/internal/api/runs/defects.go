package runs

import (
	"encoding/json"
	"errors"
	"net/http"

	apidefects "ttgo/internal/api/defects"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/models"
)

func (h *Handler) resultBelongsToRun(runID, resultID string) (testCaseID string, ok bool) {
	var rr models.RunResult
	if err := h.store.DB().Select("test_case_id, test_run_id").Where("id = ?", resultID).First(&rr).Error; err != nil {
		return "", false
	}
	if rr.TestRunID != runID {
		return "", false
	}
	if rr.TestCaseID != nil {
		testCaseID = *rr.TestCaseID
	}
	return testCaseID, true
}

// ListRunDefects godoc
//
// @Summary      List defects for a run
// @Description  Returns all defects linked to any result within the given test run.
// @Tags         runs
// @Produce      json
// @Param        id  path      string  true  "Test run ID"
// @Success      200  {array}   models.RunDefectRow
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/defect-links [get]
func (h *Handler) ListRunDefects(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListDefectsByRun(r.PathValue("id"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if rows == nil {
		rows = []models.RunDefectRow{}
	}
	httpx.JSON(w, http.StatusOK, rows)
}

// ListResultDefects godoc
//
// @Summary      List defects for a run result
// @Description  Returns all defects linked to the specified run result.
// @Tags         runs
// @Produce      json
// @Param        id         path      string  true  "Test run ID"
// @Param        result_id  path      string  true  "Run result ID"
// @Success      200  {array}   models.Defect
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/results/{result_id}/defect-links [get]
func (h *Handler) ListResultDefects(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.resultBelongsToRun(r.PathValue("id"), r.PathValue("result_id")); !ok {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "result not found in run"})
		return
	}
	defects, err := h.store.ListDefectsByResult(r.PathValue("result_id"))
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if defects == nil {
		defects = []models.Defect{}
	}
	httpx.JSON(w, http.StatusOK, defects)
}

// LinkResultDefect godoc
//
// @Summary      Link an existing defect to a run result
// @Description  Links an existing defect to the specified run result (and its test case).
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        id         path      string                    true  "Test run ID"
// @Param        result_id  path      string                    true  "Run result ID"
// @Param        body       body      models.LinkDefectRequest  true  "Defect to link"
// @Success      201  {object}  models.DefectLink
// @Failure      400  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Failure      409  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/results/{result_id}/defect-links [post]
func (h *Handler) LinkResultDefect(w http.ResponseWriter, r *http.Request) {
	testCaseID, ok := h.resultBelongsToRun(r.PathValue("id"), r.PathValue("result_id"))
	if !ok {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "result not found in run"})
		return
	}
	var req models.LinkDefectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.DefectID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "defect_id is required"})
		return
	}
	d, err := h.store.GetDefect(req.DefectID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if d == nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "defect not found"})
		return
	}
	link, err := h.store.LinkDefectToResult(req.DefectID, r.PathValue("result_id"), testCaseID)
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

// CreateAndLinkResultDefect godoc
//
// @Summary      Create and link a defect to a run result
// @Description  Creates a new defect and immediately links it to the specified run result.
// @Tags         runs
// @Accept       json
// @Produce      json
// @Param        id         path      string                        true  "Test run ID"
// @Param        result_id  path      string                        true  "Run result ID"
// @Param        body       body      models.CreateDefectRequest    true  "Defect to create"
// @Success      201  {object}  object{defect=models.Defect,link=models.DefectLink}
// @Failure      400  {object}  object{error=string}
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/results/{result_id}/defects [post]
func (h *Handler) CreateAndLinkResultDefect(w http.ResponseWriter, r *http.Request) {
	testCaseID, ok := h.resultBelongsToRun(r.PathValue("id"), r.PathValue("result_id"))
	if !ok {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "result not found in run"})
		return
	}
	var req models.CreateDefectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if msg := apidefects.ValidateCreate(req); msg != "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	d := apidefects.DefectFromCreate(req)
	if err := h.store.CreateDefect(d); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	link, err := h.store.LinkDefectToResult(d.ID, r.PathValue("result_id"), testCaseID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]interface{}{"defect": d, "link": link})
}

// UnlinkResultDefect godoc
//
// @Summary      Unlink a defect from a run result
// @Description  Removes the defect link from the specified run result.
// @Tags         runs
// @Param        id         path  string  true  "Test run ID"
// @Param        result_id  path  string  true  "Run result ID"
// @Param        defect_id  path  string  true  "Defect ID"
// @Success      204
// @Failure      404  {object}  object{error=string}
// @Failure      500  {object}  object{error=string}
// @Router       /runs/{id}/results/{result_id}/defect-links/{defect_id} [delete]
func (h *Handler) UnlinkResultDefect(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.resultBelongsToRun(r.PathValue("id"), r.PathValue("result_id")); !ok {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "result not found in run"})
		return
	}
	if err := h.store.UnlinkDefectFromResult(r.PathValue("defect_id"), r.PathValue("result_id")); err != nil {
		if err.Error() == "defect link not found" {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
