package tests

import (
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"strings"
	"time"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/microcosm-cc/bluemonday"
	"gorm.io/gorm"
)

// handleGetTests godoc
// @Summary      List test cases
// @Description  Returns a list of test cases, optionally filtered by folder IDs or category ID
// @Tags         tests
// @Accept       json
// @Produce      json
// @Param        folder_ids  query     string  false  "Comma-separated list of folder IDs"
// @Param        folder_id   query     string  false  "Single folder ID (can be repeated)"
// @Param        category_id query     string  false  "Category ID to filter by"
// @Param        view        query     string  false  "Set to 'list' to omit full Steps/CustomValues and return steps_count instead (smaller payload)"
// @Success      200  {array}   models.TestCase
// @Failure      500  {object}  map[string]string
// @Router       /tests [get]
func (h *Handler) GetTests(w http.ResponseWriter, r *http.Request) {
	rawIDs := r.URL.Query().Get("folder_ids")
	var folderIDs []string
	if rawIDs != "" {
		folderIDs = strings.Split(rawIDs, ",")
	} else {
		folderIDs = r.URL.Query()["folder_id"]
	}
	categoryID := r.URL.Query().Get("category_id")
	view := r.URL.Query().Get("view")

	filter := store.TestCaseFilter{
		ListView: view == "list",
	}
	if len(folderIDs) > 0 {
		filter.FolderIDs = folderIDs
	}
	if categoryID != "" {
		filter.CategoryID = &categoryID
	}

	tests, err := h.store.ListTestCases(filter)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, tests)
}

// handleGetTestByCustomField godoc
// @Summary      Find a test case by custom field value
// @Description  Returns the test case whose custom field matches the given name and value
// @Tags         tests
// @Produce      json
// @Param        field  query     string  true  "Custom field name (e.g. QTestId)"
// @Param        value  query     string  true  "Custom field value to match"
// @Success      200  {object}  models.TestCase
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /tests/by-custom-field [get]
func (h *Handler) GetTestByCustomField(w http.ResponseWriter, r *http.Request) {
	fieldName := r.URL.Query().Get("field")
	fieldValue := r.URL.Query().Get("value")

	if fieldName == "" || fieldValue == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "field and value query parameters are required"})
		return
	}

	test, err := h.store.GetTestCaseByCustomField(fieldName, fieldValue)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}
	httpx.JSON(w, http.StatusOK, test)
}

// handleGetTest godoc
// @Summary      Get a test case
// @Description  Returns a single test case by ID
// @Tags         tests
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "Test case ID"
// @Success      200  {object}  models.TestCase
// @Failure      404  {object}  map[string]string
// @Router       /tests/{id} [get]
func (h *Handler) GetTest(w http.ResponseWriter, r *http.Request) {
	testID := r.PathValue("id")
	test, err := h.store.GetTestCase(testID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}
	httpx.JSON(w, http.StatusOK, test)
}

// handleCreateTest godoc
// @Summary      Create a test case
// @Description  Creates a new test case
// @Tags         tests
// @Accept       json
// @Produce      json
// @Param        body  body      object{name=string,folder_id=string,description=string,steps=[]models.TestStep,custom_values=[]models.CustomFieldValue}  true  "Test case payload"
// @Success      201  {object}  models.TestCase
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /tests [post]
func (h *Handler) CreateTest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string                    `json:"name"`
		FolderID     string                    `json:"folder_id"`
		Description  string                    `json:"description"`
		Steps        []models.TestStep         `json:"steps"`
		CustomValues []models.CustomFieldValue `json:"custom_values"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if req.Name == "" || req.FolderID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Name and FolderID are required"})
		return
	}

	// Validate folder exists
	if _, err := h.store.GetFolder(req.FolderID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "folder_id references a non-existent folder"})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	// Sanitize HTML-bearing fields before persisting (FR-002, FR-007).
	p := h.sanitizer
	req.Description = httpx.NormalizeEmptyHTML(p, req.Description)
	for i := range req.Steps {
		req.Steps[i].Action = httpx.NormalizeEmptyHTML(p, req.Steps[i].Action)
		req.Steps[i].ExpectedResult = httpx.NormalizeEmptyHTML(p, req.Steps[i].ExpectedResult)
	}

	test := &models.TestCase{
		Name:        req.Name,
		FolderID:    req.FolderID,
		Description: req.Description,
	}

	for _, step := range req.Steps {
		s := step
		test.Steps = append(test.Steps, &s)
	}

	for _, cv := range req.CustomValues {
		v := cv
		test.CustomValues = append(test.CustomValues, &v)
	}

	if err := h.store.CreateTestCase(test); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, test)
}

// handleDeleteTest godoc
// @Summary      Delete a test case
// @Description  Deletes the test case with the given ID. Run results are preserved with a NULL test_case_id.
// @Tags         tests
// @Param        id  path  string  true  "Test case ID"
// @Success      204
// @Failure      500  {object}  map[string]string
// @Router       /tests/{id} [delete]
func (h *Handler) DeleteTest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteTestCase(id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleBulkDeleteTests godoc
// @Summary      Bulk delete test cases
// @Description  Deletes multiple test cases by their IDs. Run results are preserved with a NULL test_case_id.
// @Tags         tests
// @Accept       json
// @Param        body  body  object{ids=[]string}  true  "List of test case IDs to delete"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /tests/bulk-delete [post]
func (h *Handler) BulkDeleteTests(w http.ResponseWriter, r *http.Request) {
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
	if err := h.store.DeleteTestCases(req.IDs); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleAssignCategory godoc
// @Summary      Assign a category to a test case
// @Description  Associates a category with the specified test case
// @Tags         tests
// @Accept       json
// @Produce      json
// @Param        id    path      string                  true  "Test case ID"
// @Param        body  body      object{category_id=string}  true  "Category assignment payload"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /tests/{id}/categories [post]
func (h *Handler) AssignCategory(w http.ResponseWriter, r *http.Request) {
	testID := r.PathValue("id")
	var req struct {
		CategoryID string `json:"category_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	if req.CategoryID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "CategoryID is required"})
		return
	}

	if err := h.store.AssignCategoryToTest(req.CategoryID, testID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleUpdateTest godoc
// @Summary      Update a test case
// @Description  Updates an existing test case by ID
// @Tags         tests
// @Accept       json
// @Produce      json
// @Param        id    path      string                                                                                                                                          true  "Test case ID"
// @Param        body  body      object{name=string,folder_id=string,description=string,steps=[]models.TestStep,custom_values=[]models.CustomFieldValue,categories=[]models.Category}  true  "Test case update payload"
// @Success      200  {object}  models.TestCase
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /tests/{id} [put]
func (h *Handler) UpdateTest(w http.ResponseWriter, r *http.Request) {
	testID := r.PathValue("id")
	var req struct {
		Name         string                    `json:"name"`
		FolderID     string                    `json:"folder_id"`
		Description  string                    `json:"description"`
		Steps        []models.TestStep         `json:"steps"`
		CustomValues []models.CustomFieldValue `json:"custom_values"`
		Categories   []models.Category         `json:"categories"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	// Validate the target folder exists, matching CreateTest (F-052).
	if req.FolderID != "" {
		if _, err := h.store.GetFolder(req.FolderID); err != nil {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "folder not found"})
			return
		}
	}

	// Sanitize HTML-bearing fields before persisting (FR-002, FR-007).
	p := h.sanitizer
	req.Description = httpx.NormalizeEmptyHTML(p, req.Description)
	for i := range req.Steps {
		req.Steps[i].Action = httpx.NormalizeEmptyHTML(p, req.Steps[i].Action)
		req.Steps[i].ExpectedResult = httpx.NormalizeEmptyHTML(p, req.Steps[i].ExpectedResult)
	}

	test := &models.TestCase{
		ID:          testID,
		Name:        req.Name,
		FolderID:    req.FolderID,
		Description: req.Description,
	}

	if req.Steps != nil {
		test.Steps = []*models.TestStep{}
		for _, step := range req.Steps {
			s := step
			test.Steps = append(test.Steps, &s)
		}
	}

	if req.CustomValues != nil {
		test.CustomValues = []*models.CustomFieldValue{}
		for _, cv := range req.CustomValues {
			v := cv
			test.CustomValues = append(test.CustomValues, &v)
		}
	}

	if req.Categories != nil {
		test.Categories = []*models.Category{}
		for _, category := range req.Categories {
			s := category
			test.Categories = append(test.Categories, &s)
		}
	}

	if err := h.store.UpdateTestCase(test); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, test)
}

// handleExportTests godoc
// @Summary      Export test cases as JSON
// @Description  Exports selected test cases as a downloadable JSON file with configurable fields
// @Tags         tests
// @Accept       json
// @Produce      json
// @Param        body  body  object{ids=[]string,fields=[]string}  true  "Test case IDs and fields to include"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /tests/export [post]
func (h *Handler) ExportTests(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs    []string `json:"ids"`
		Fields []string `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if len(req.IDs) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "ids must not be empty"})
		return
	}
	if len(req.IDs) > httpx.MaxBulkIDs {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many ids (max 500 per request)"})
		return
	}

	// Build a set of requested fields for fast lookup.
	validFields := map[string]bool{
		"name": true, "description": true, "steps": true,
		"categories": true, "custom_values": true, "linked_requirements": true,
	}
	fieldSet := make(map[string]bool, len(req.Fields))
	var cleanFields []string
	for _, f := range req.Fields {
		if validFields[f] {
			fieldSet[f] = true
			cleanFields = append(cleanFields, f)
		}
	}

	tests, err := h.store.GetTestCasesByIDs(req.IDs)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	// Build export objects with only the requested fields.
	// Strip HTML tags for plain-text export.
	strip := bluemonday.StrictPolicy()
	plainText := func(s string) string {
		return strings.TrimSpace(html.UnescapeString(strip.Sanitize(s)))
	}

	type exportStep struct {
		Order          int    `json:"order"`
		Action         string `json:"action"`
		ExpectedResult string `json:"expected_result"`
	}
	type exportRequirement struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}

	exported := make([]map[string]interface{}, 0, len(tests))
	for _, tc := range tests {
		obj := map[string]interface{}{"id": tc.ID}

		if fieldSet["name"] {
			obj["name"] = tc.Name
		}
		if fieldSet["description"] {
			obj["description"] = plainText(tc.Description)
		}
		if fieldSet["steps"] {
			steps := make([]exportStep, 0, len(tc.Steps))
			for _, s := range tc.Steps {
				steps = append(steps, exportStep{
					Order:          s.OrderIndex + 1,
					Action:         plainText(s.Action),
					ExpectedResult: plainText(s.ExpectedResult),
				})
			}
			obj["steps"] = steps
		}
		if fieldSet["categories"] {
			names := make([]string, 0, len(tc.Categories))
			for _, c := range tc.Categories {
				names = append(names, c.Name)
			}
			obj["categories"] = names
		}
		if fieldSet["custom_values"] {
			cv := make(map[string]interface{})
			for _, v := range tc.CustomValues {
				fieldName := v.CustomFieldID
				if v.CustomFieldDef != nil {
					fieldName = v.CustomFieldDef.Name
				}
				var val interface{}
				_ = json.Unmarshal(v.Value, &val)
				cv[fieldName] = val
			}
			obj["custom_values"] = cv
		}
		if fieldSet["linked_requirements"] {
			reqs := make([]exportRequirement, 0, len(tc.LinkedRequirements))
			for _, lr := range tc.LinkedRequirements {
				reqs = append(reqs, exportRequirement{ID: lr.ID, Title: lr.Title})
			}
			obj["linked_requirements"] = reqs
		}

		exported = append(exported, obj)
	}

	now := time.Now().UTC()
	result := map[string]interface{}{
		"exported_at": now.Format(time.RFC3339),
		"count":       len(exported),
		"fields":      cleanFields,
		"test_cases":  exported,
	}

	timestamp := now.Format("2006-01-02T150405")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="ttgo-export-`+timestamp+`.json"`)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}
