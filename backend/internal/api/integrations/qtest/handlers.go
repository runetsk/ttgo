package qtest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"ttgo/internal/api/httpx"
	apiws "ttgo/internal/api/websocket"
	"ttgo/internal/safehttp"
	"ttgo/pkg/tracker/models"
)

// maxQTestBatch bounds per-request array sizes on qTest bulk endpoints, which
// drive external API calls and local writes (F-029).
const maxQTestBatch = 2000

// ────────────────────────────────────────────────────────────────────────────
// T009: QTest config handlers (admin-only)
// ────────────────────────────────────────────────────────────────────────────

// handleGetQTestConfig returns the current qTest integration configuration.
//
// @Summary      Get qTest config
// @Description  Return the current qTest integration configuration with masked API token.
// @Tags         qtest
// @Produce      json
// @Success      200  {object}  object
// @Failure      500  {object}  map[string]string
// @Router       /settings/qtest [get]
// @Security     BearerAuth
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetQTestConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil {
		httpx.JSON(w, http.StatusOK, map[string]interface{}{"enabled": false})
		return
	}
	resp := cfg.MaskedConfig()
	// Attach enabled projects
	projects, _ := h.store.ListEnabledQTestProjects()
	resp.Projects = projects
	httpx.JSON(w, http.StatusOK, resp)
}

// handleUpsertQTestConfig creates or updates the qTest integration configuration.
//
// @Summary      Upsert qTest config
// @Description  Create or update the qTest integration configuration (base URL, email, API token, project).
// @Tags         qtest
// @Accept       json
// @Produce      json
// @Param        body  body  object{base_url=string,email=string,api_token=string,project_id=int,project_name=string,enabled=bool}  true  "qTest config"
// @Success      200  {object}  object
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /settings/qtest [put]
// @Security     BearerAuth
func (h *Handler) UpsertConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BaseURL     string `json:"base_url"`
		Email       string `json:"email"`
		APIToken    string `json:"api_token"`
		ProjectID   int64  `json:"project_id,string"`
		ProjectName string `json:"project_name"`
		Enabled     bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.BaseURL) == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "base_url is required"})
		return
	}
	if strings.TrimSpace(req.Email) == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}
	if err := safehttp.ValidateIntegrationURL(strings.TrimSpace(req.BaseURL)); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "base_url rejected: " + err.Error()})
		return
	}

	cfg, err := h.store.UpsertQTestConfig(req.BaseURL, req.Email, req.APIToken, req.ProjectID, req.ProjectName, req.Enabled)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	resp := cfg.MaskedConfig()
	projects, _ := h.store.ListEnabledQTestProjects()
	resp.Projects = projects

	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventSettingsChanged, "settings:*", map[string]string{"integration": "qtest"}))
	}

	httpx.JSON(w, http.StatusOK, resp)
}

// ────────────────────────────────────────────────────────────────────────────
// T011: Test QTest connection
// ────────────────────────────────────────────────────────────────────────────

// handleTestQTestConnection tests the qTest API connection using stored credentials.
//
// @Summary      Test qTest connection
// @Description  Verify the qTest API connection using the stored configuration.
// @Tags         qtest
// @Produce      json
// @Success      200  {object}  object{connected=bool,message=string}
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /settings/qtest/test-connection [post]
// @Security     BearerAuth
func (h *Handler) TestConnection(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetQTestConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "QTest is not configured"})
		return
	}

	ok, message, err := h.store.TestQTestConnection(cfg)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"connected": ok,
		"message":   message,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// T013: List QTest projects (from API)
// ────────────────────────────────────────────────────────────────────────────

// handleListQTestProjects lists available qTest projects from the API.
//
// @Summary      List qTest projects
// @Description  Fetch all available projects from the qTest API.
// @Tags         qtest
// @Produce      json
// @Success      200  {array}  object
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /qtest/projects [get]
// @Security     BearerAuth
func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetQTestConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "QTest is not configured"})
		return
	}

	projects, err := h.store.FetchQTestProjectsFromAPI(cfg)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, projects)
}

// ────────────────────────────────────────────────────────────────────────────
// Enabled projects CRUD (multi-project support)
// ────────────────────────────────────────────────────────────────────────────

// handleListEnabledQTestProjects lists locally enabled qTest projects.
//
// @Summary      List enabled qTest projects
// @Description  Return locally enabled qTest projects.
// @Tags         qtest
// @Produce      json
// @Success      200  {array}  object
// @Failure      500  {object}  map[string]string
// @Router       /qtest/enabled-projects [get]
// @Security     BearerAuth
func (h *Handler) ListEnabledProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.store.ListEnabledQTestProjects()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, projects)
}

// handleAddEnabledQTestProject adds a project to the locally enabled list.
//
// @Summary      Add enabled qTest project
// @Description  Add a qTest project to the locally enabled list for upload/sync.
// @Tags         qtest
// @Accept       json
// @Produce      json
// @Param        body  body  object{project_id=int,project_name=string}  true  "Project to enable"
// @Success      200  {object}  object
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /qtest/enabled-projects [post]
// @Security     BearerAuth
func (h *Handler) AddEnabledProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID   int64  `json:"project_id"`
		ProjectName string `json:"project_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.ProjectID == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "project_id is required"})
		return
	}
	if req.ProjectName == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "project_name is required"})
		return
	}

	p, err := h.store.AddEnabledQTestProject(req.ProjectID, req.ProjectName)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

// handleRemoveEnabledQTestProject removes a project from the locally enabled list.
//
// @Summary      Remove enabled qTest project
// @Description  Remove a qTest project from the locally enabled list.
// @Tags         qtest
// @Accept       json
// @Produce      json
// @Param        body  body  object{project_id=int}  true  "Project to remove"
// @Success      200  {object}  map[string]string
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /qtest/enabled-projects/remove [post]
// @Security     BearerAuth
func (h *Handler) RemoveEnabledProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID int64 `json:"project_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.ProjectID == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "project_id is required"})
		return
	}
	if err := h.store.RemoveEnabledQTestProject(req.ProjectID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// handleSetDefaultQTestProject sets a project as the default qTest project.
//
// @Summary      Set default qTest project
// @Description  Set an enabled qTest project as the default for uploads.
// @Tags         qtest
// @Accept       json
// @Produce      json
// @Param        body  body  object{project_id=int}  true  "Project to set as default"
// @Success      200  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /qtest/enabled-projects/set-default [post]
// @Security     BearerAuth
func (h *Handler) SetDefaultProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID int64 `json:"project_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if err := h.store.SetDefaultQTestProject(req.ProjectID); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "default_set"})
}

// ────────────────────────────────────────────────────────────────────────────
// T019: Upload test cases to QTest
// ────────────────────────────────────────────────────────────────────────────

// handleUploadToQTest uploads selected test cases to a qTest module.
//
// @Summary      Upload test cases to qTest
// @Description  Upload one or more test cases to a specified qTest module. Supports skip/update conflict resolution.
// @Tags         qtest
// @Accept       json
// @Produce      json
// @Param        body  body  object{test_case_ids=[]string,module_id=int,module_path=string,on_conflict=string,project_id=int}  true  "Upload parameters"
// @Success      200  {object}  object
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /qtest/upload [post]
// @Security     BearerAuth
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TestCaseIDs []string `json:"test_case_ids"`
		ModuleID    int64    `json:"module_id"`
		ModulePath  string   `json:"module_path"`
		OnConflict  string   `json:"on_conflict"` // "skip" or "update"
		ProjectID   int64    `json:"project_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if len(req.TestCaseIDs) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "test_case_ids is required"})
		return
	}
	if len(req.TestCaseIDs) > maxQTestBatch {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many test_case_ids (max 2000 per request)"})
		return
	}
	if req.ModuleID == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "module_id is required"})
		return
	}
	if req.ProjectID == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "project_id is required"})
		return
	}
	if req.OnConflict == "" {
		req.OnConflict = "skip"
	}

	result, err := h.store.UploadTestCasesToQTest(req.TestCaseIDs, req.ModuleID, req.ModulePath, req.OnConflict, req.ProjectID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// ────────────────────────────────────────────────────────────────────────────
// T020: Get QTest mapping for a test case
// ────────────────────────────────────────────────────────────────────────────

// ────────────────────────────────────────────────────────────────────────────
// Batch get QTest mappings for multiple test cases
// ────────────────────────────────────────────────────────────────────────────

// handleBatchGetQTestMappings returns qTest mappings for multiple test cases.
//
// @Summary      Batch get qTest mappings
// @Description  Return qTest mappings for multiple test cases in a single request.
// @Tags         qtest
// @Accept       json
// @Produce      json
// @Param        body  body  object{test_case_ids=[]string}  true  "Test case IDs"
// @Success      200  {object}  object{mappings=object}
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /qtest/batch-mappings [post]
// @Security     BearerAuth
func (h *Handler) BatchGetMappings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TestCaseIDs []string `json:"test_case_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if len(body.TestCaseIDs) == 0 {
		httpx.JSON(w, http.StatusOK, map[string]interface{}{"mappings": map[string]interface{}{}})
		return
	}
	if len(body.TestCaseIDs) > maxQTestBatch {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many test_case_ids (max 2000 per request)"})
		return
	}

	mappings, err := h.store.GetQTestMappingsByTestCases(body.TestCaseIDs)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"mappings": mappings})
}

// ────────────────────────────────────────────────────────────────────────────
// T021: Unlink QTest mapping
// ────────────────────────────────────────────────────────────────────────────

// ────────────────────────────────────────────────────────────────────────────
// T028: Sync test cases to QTest
// ────────────────────────────────────────────────────────────────────────────

// handleSyncToQTest syncs local test case changes to qTest for already-mapped cases.
//
// @Summary      Sync test cases to qTest
// @Description  Push local changes for already-mapped test cases to qTest.
// @Tags         qtest
// @Accept       json
// @Produce      json
// @Param        body  body  object{test_case_ids=[]string}  true  "Test case IDs to sync"
// @Success      200  {object}  object
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /qtest/sync [post]
// @Security     BearerAuth
func (h *Handler) Sync(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TestCaseIDs []string `json:"test_case_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if len(req.TestCaseIDs) > maxQTestBatch {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many test_case_ids (max 2000 per request)"})
		return
	}

	result, err := h.store.SyncTestCasesToQTest(req.TestCaseIDs)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}

// ────────────────────────────────────────────────────────────────────────────
// Upload entire folder to QTest (create module + upload test cases)
// ────────────────────────────────────────────────────────────────────────────

// handleUploadFolderToQTest uploads an entire folder's test cases to qTest.
//
// @Summary      Upload folder to qTest
// @Description  Create a qTest module from a folder and upload all its test cases. Supports skip/update conflict resolution.
// @Tags         qtest
// @Accept       json
// @Produce      json
// @Param        body  body  object{folder_id=string,project_id=int,parent_module_id=int,on_conflict=string}  true  "Upload parameters"
// @Success      200  {object}  object{result=object,module_id=int}
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /qtest/upload-folder [post]
// @Security     BearerAuth
func (h *Handler) UploadFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FolderID       string `json:"folder_id"`
		ProjectID      int64  `json:"project_id"`
		ParentModuleID int64  `json:"parent_module_id"` // 0 = root
		OnConflict     string `json:"on_conflict"`      // "skip" or "update"
		Recursive      bool   `json:"recursive"`        // mirror the full subtree
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.FolderID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "folder_id is required"})
		return
	}
	if req.ProjectID == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "project_id is required"})
		return
	}
	if req.OnConflict == "" {
		req.OnConflict = "skip"
	}

	result, moduleID, err := h.store.UploadFolderToQTest(req.FolderID, req.ProjectID, req.ParentModuleID, req.OnConflict, req.Recursive)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"result":    result,
		"module_id": moduleID,
	})
}

// UnlinkFolder removes qTest mappings for every test case under a folder.
//
// @Summary      Unlink folder from qTest
// @Description  Remove qTest test-case links for a folder, optionally recursing into subfolders.
// @Tags         qtest
// @Accept       json
// @Produce      json
// @Param        body  body  object{folder_id=string,recursive=bool}  true  "Unlink parameters"
// @Success      200  {object}  object{deleted=int}
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /qtest/unlink-folder [post]
// @Security     BearerAuth
func (h *Handler) UnlinkFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FolderID  string `json:"folder_id"`
		Recursive bool   `json:"recursive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.FolderID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "folder_id is required"})
		return
	}
	deleted, err := h.store.UnlinkQTestMappingsByFolder(req.FolderID, req.Recursive)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"deleted": deleted})
}

// BulkUnlink removes qTest mappings for the given test case IDs.
//
// @Summary      Bulk unlink qTest mappings
// @Description  Remove qTest test-case links for every supplied test case ID.
// @Tags         qtest
// @Accept       json
// @Produce      json
// @Param        body  body  object{test_case_ids=[]string}  true  "Bulk unlink parameters"
// @Success      200  {object}  object{deleted=int}
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /qtest/bulk-unlink [post]
// @Security     BearerAuth
func (h *Handler) BulkUnlink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TestCaseIDs []string `json:"test_case_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if len(req.TestCaseIDs) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "test_case_ids is required"})
		return
	}
	if len(req.TestCaseIDs) > maxQTestBatch {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many test_case_ids (max 2000 per request)"})
		return
	}
	deleted, err := h.store.BulkDeleteQTestMappings(req.TestCaseIDs)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"deleted": deleted})
}

// ────────────────────────────────────────────────────────────────────────────
// T037: List QTest modules
// ────────────────────────────────────────────────────────────────────────────

// handleListQTestModules lists qTest modules for a project.
//
// @Summary      List qTest modules
// @Description  Fetch the module tree from qTest for a given project. Falls back to the configured project if project_id is not provided.
// @Tags         qtest
// @Produce      json
// @Param        project_id  query  int  false  "qTest project ID (defaults to configured project)"
// @Success      200  {array}  object
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /qtest/modules [get]
// @Security     BearerAuth
func (h *Handler) ListModules(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetQTestConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "QTest is not configured"})
		return
	}

	// Accept project_id from query string; fall back to legacy config
	var projectID int64
	if pidStr := r.URL.Query().Get("project_id"); pidStr != "" {
		projectID, _ = strconv.ParseInt(pidStr, 10, 64)
	}
	if projectID == 0 {
		projectID = cfg.ProjectID
	}
	if projectID == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "project_id is required — select a project or configure one in Settings"})
		return
	}

	modules, err := h.store.ListQTestModules(cfg, projectID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, modules)
}

// ListTestCases lists test cases from a QTest module.
//
// @Summary      List qTest test cases
// @Description  Fetch test cases from a qTest module, including steps.
// @Tags         qtest
// @Produce      json
// @Param        project_id  query  int  true  "QTest project ID"
// @Param        module_id   query  int  true   "QTest module ID"
// @Param        recursive   query  bool false  "Include descendant modules"
// @Success      200  {array}  models.QTestRemoteTestCase
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /qtest/test-cases [get]
// @Security     BearerAuth
func (h *Handler) ListTestCases(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetQTestConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil || !cfg.Enabled {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "QTest is not configured or disabled"})
		return
	}

	projectIDStr := r.URL.Query().Get("project_id")
	moduleIDStr := r.URL.Query().Get("module_id")
	if projectIDStr == "" || moduleIDStr == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "project_id and module_id are required"})
		return
	}

	projectID, err := strconv.ParseInt(projectIDStr, 10, 64)
	if err != nil || projectID == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project_id"})
		return
	}
	moduleID, err := strconv.ParseInt(moduleIDStr, 10, 64)
	if err != nil || moduleID == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "invalid module_id"})
		return
	}

	recursive := strings.EqualFold(r.URL.Query().Get("recursive"), "true") || r.URL.Query().Get("recursive") == "1"

	var testCases []models.QTestRemoteTestCase
	if recursive {
		testCases, err = h.store.FetchQTestTestCasesRecursive(cfg, projectID, moduleID)
	} else {
		testCases, err = h.store.FetchQTestTestCases(cfg, projectID, moduleID)
	}
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, testCases)
}

// Import imports selected QTest test cases into a TTGO folder.
//
// @Summary      Import qTest test cases
// @Description  Import selected test cases from qTest into a TTGO folder, setting QTestId and creating sync mappings.
// @Tags         qtest
// @Accept       json
// @Produce      json
// @Param        body  body  object{project_id=int,module_id=int,module_path=string,folder_id=string,test_case_ids=[]int,test_cases=[]models.QTestRemoteTestCase,on_conflict=string,preserve_hierarchy=bool,recursive=bool}  true  "Import request"
// @Success      200  {object}  models.QTestBulkResult
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /qtest/import [post]
// @Security     BearerAuth
func (h *Handler) Import(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetQTestConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil || !cfg.Enabled {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "QTest is not configured or disabled"})
		return
	}

	var req struct {
		ProjectID         int64                        `json:"project_id"`
		ModuleID          int64                        `json:"module_id"`
		ModulePath        string                       `json:"module_path"`
		FolderID          string                       `json:"folder_id"`
		TestCaseIDs       []int64                      `json:"test_case_ids"`
		TestCases         []models.QTestRemoteTestCase `json:"test_cases"`
		OnConflict        string                       `json:"on_conflict"`
		PreserveHierarchy bool                         `json:"preserve_hierarchy"`
		Recursive         bool                         `json:"recursive"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.ProjectID == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "project_id is required"})
		return
	}
	if req.ModuleID == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "module_id is required"})
		return
	}
	if strings.TrimSpace(req.FolderID) == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "folder_id is required"})
		return
	}
	if len(req.TestCaseIDs) == 0 && len(req.TestCases) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "either test_case_ids or test_cases must be provided"})
		return
	}
	if len(req.TestCaseIDs) > maxQTestBatch || len(req.TestCases) > maxQTestBatch {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many items (max 2000 per request)"})
		return
	}
	if req.OnConflict == "" {
		req.OnConflict = "skip"
	}
	if req.OnConflict != "skip" && req.OnConflict != "update" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "on_conflict must be 'skip' or 'update'"})
		return
	}

	testCases := req.TestCases
	if len(req.TestCaseIDs) > 0 {
		if req.Recursive {
			testCases, err = h.store.FetchQTestTestCasesRecursive(cfg, req.ProjectID, req.ModuleID)
		} else {
			testCases, err = h.store.FetchQTestTestCases(cfg, req.ProjectID, req.ModuleID)
		}
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}

		selectedIDs := make(map[int64]struct{}, len(req.TestCaseIDs))
		for _, id := range req.TestCaseIDs {
			selectedIDs[id] = struct{}{}
		}

		filtered := make([]models.QTestRemoteTestCase, 0, len(req.TestCaseIDs))
		for _, testCase := range testCases {
			if _, ok := selectedIDs[testCase.ID]; ok {
				filtered = append(filtered, testCase)
			}
		}
		testCases = filtered
	}

	if len(testCases) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "no matching qTest test cases found for import"})
		return
	}

	result, err := h.store.ImportQTestTestCases(cfg, req.ProjectID, req.ModuleID, req.ModulePath, req.FolderID, testCases, req.OnConflict, req.PreserveHierarchy)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, result)
}
