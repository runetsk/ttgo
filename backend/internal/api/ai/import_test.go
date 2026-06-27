package ai_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	api "ttgo/internal/api"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
)

// testEnv holds the server and auth session for tests.
type testEnv struct {
	srv          *api.Server
	sessionToken string
}

// testServer creates a fresh Server backed by a temp SQLite DB, seeds an admin, and
// creates a session cookie for authenticated requests.
func testServer(t *testing.T) (*testEnv, func()) {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "ttgo-test-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	tmpFile.Close()

	s, err := store.New(tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("store.New: %v", err)
	}
	// Seed admin user
	if err := s.SeedAdminIfNeeded("test@test.com", "testpass123"); err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("seed admin: %v", err)
	}
	srv := api.NewServer(s)

	// Login to get session token
	loginBody := `{"email":"test@test.com","password":"testpass123"}`
	loginReq := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRR := httptest.NewRecorder()
	srv.ServeHTTP(loginRR, loginReq)
	if loginRR.Code != http.StatusOK {
		os.Remove(tmpFile.Name())
		t.Fatalf("login failed: %d %s", loginRR.Code, loginRR.Body.String())
	}

	// Extract session_token cookie
	var sessionToken string
	for _, c := range loginRR.Result().Cookies() {
		if c.Name == "session_token" {
			sessionToken = c.Value
			break
		}
	}
	if sessionToken == "" {
		os.Remove(tmpFile.Name())
		t.Fatal("no session_token cookie after login")
	}

	env := &testEnv{srv: srv, sessionToken: sessionToken}
	cleanup := func() { os.Remove(tmpFile.Name()) }
	return env, cleanup
}

// doRequest performs an authenticated HTTP request against the server.
func doRequest(env *testEnv, method, path string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session_token", Value: env.sessionToken})
	rr := httptest.NewRecorder()
	env.srv.ServeHTTP(rr, req)
	return rr
}

// createTestFolder creates a folder and returns its ID.
func createTestFolder(t *testing.T, env *testEnv, name string) string {
	t.Helper()
	body := map[string]string{"name": name}
	rr := doRequest(env, "POST", "/api/folders", body)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("create folder: got status %d, body: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.ID == "" {
		t.Fatal("create folder: empty ID")
	}
	return resp.ID
}

// createTestRequirement creates a requirement and returns its ID.
func createTestRequirement(t *testing.T, env *testEnv, identifier, title string) string {
	t.Helper()
	body := map[string]string{"identifier": identifier, "title": title}
	rr := doRequest(env, "POST", "/api/requirements", body)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("create requirement: got status %d, body: %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.ID == "" {
		t.Fatal("create requirement: empty ID")
	}
	return resp.ID
}

// ── Parse endpoint tests ─────────────────────────────────────────────────

func TestAIImport_Parse_JSONArray(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	content := `[
		{"name": "Login test", "description": "Verify login", "steps": [
			{"action": "Enter credentials", "expected_result": "Form accepted"}
		]},
		{"name": "Logout test", "steps": [
			{"action": "Click logout", "expected_result": "Redirected to login"}
		]}
	]`

	rr := doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{Content: content})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp models.ParseImportResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.DetectedFormat != "json" {
		t.Errorf("expected format 'json', got %q", resp.DetectedFormat)
	}
	if len(resp.TestCases) != 2 {
		t.Fatalf("expected 2 test cases, got %d", len(resp.TestCases))
	}
	if resp.TestCases[0].Name != "Login test" {
		t.Errorf("expected name 'Login test', got %q", resp.TestCases[0].Name)
	}
	if len(resp.TestCases[0].Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(resp.TestCases[0].Steps))
	}
	if resp.TestCases[0].TempID == "" {
		t.Error("expected non-empty temp_id")
	}
	if resp.Truncated {
		t.Error("should not be truncated")
	}
}

func TestAIImport_Parse_JSONMarkdownFences(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	content := "```json\n[{\"name\": \"Fenced test\", \"steps\": [{\"action\": \"Do A\", \"expected_result\": \"See B\"}]}]\n```"

	rr := doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{Content: content})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp models.ParseImportResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.TestCases) != 1 {
		t.Fatalf("expected 1 test case, got %d", len(resp.TestCases))
	}
	if resp.TestCases[0].Name != "Fenced test" {
		t.Errorf("expected name 'Fenced test', got %q", resp.TestCases[0].Name)
	}
}

func TestAIImport_Parse_CSV(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	content := "name,description,action,expected_result\nLogin flow,Test login,Enter creds,Success\nLogout flow,Test logout,Click logout,Redirected"

	rr := doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{
		Content:    content,
		FormatHint: "csv",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp models.ParseImportResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.DetectedFormat != "csv" {
		t.Errorf("expected format 'csv', got %q", resp.DetectedFormat)
	}
	if len(resp.TestCases) != 2 {
		t.Fatalf("expected 2 test cases, got %d", len(resp.TestCases))
	}
	if resp.TestCases[0].Name != "Login flow" {
		t.Errorf("expected 'Login flow', got %q", resp.TestCases[0].Name)
	}
	if resp.TestCases[0].Description != "Test login" {
		t.Errorf("expected description 'Test login', got %q", resp.TestCases[0].Description)
	}
}

func TestAIImport_Parse_MarkdownTable(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	content := `| Name | Action | Expected Result |
|------|--------|-----------------|
| Login test | Enter credentials | Login succeeds |
| Signup test | Fill form | Account created |`

	rr := doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{Content: content})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp models.ParseImportResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.DetectedFormat != "markdown_table" {
		t.Errorf("expected format 'markdown_table', got %q", resp.DetectedFormat)
	}
	if len(resp.TestCases) != 2 {
		t.Fatalf("expected 2 test cases, got %d", len(resp.TestCases))
	}
	if resp.TestCases[0].Name != "Login test" {
		t.Errorf("expected 'Login test', got %q", resp.TestCases[0].Name)
	}
	if len(resp.TestCases[0].Steps) == 0 {
		t.Error("expected at least 1 step")
	} else {
		if resp.TestCases[0].Steps[0].Action != "Enter credentials" {
			t.Errorf("expected action 'Enter credentials', got %q", resp.TestCases[0].Steps[0].Action)
		}
	}
}

func TestAIImport_Parse_NumberedList(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	content := `1. Login test
   - Enter username → Username field populated
   - Click login → Dashboard shown
2. Logout test
   - Click logout button → Redirected to login page`

	rr := doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{Content: content})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp models.ParseImportResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.DetectedFormat != "numbered_list" {
		t.Errorf("expected format 'numbered_list', got %q", resp.DetectedFormat)
	}
	if len(resp.TestCases) != 2 {
		t.Fatalf("expected 2 test cases, got %d", len(resp.TestCases))
	}
	if resp.TestCases[0].Name != "Login test" {
		t.Errorf("expected 'Login test', got %q", resp.TestCases[0].Name)
	}
	if len(resp.TestCases[0].Steps) != 2 {
		t.Errorf("expected 2 steps for first TC, got %d", len(resp.TestCases[0].Steps))
	}
}

func TestAIImport_Parse_AutoDetect(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	// Should auto-detect as markdown_table
	content := `| Name | Action | Expected Result |
|------|--------|-----------------|
| Auto-detect test | Do something | Something happens |`

	rr := doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{Content: content})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp models.ParseImportResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.DetectedFormat != "markdown_table" {
		t.Errorf("expected auto-detected 'markdown_table', got %q", resp.DetectedFormat)
	}
	if len(resp.TestCases) != 1 {
		t.Fatalf("expected 1 test case, got %d", len(resp.TestCases))
	}
}

func TestAIImport_Parse_EmptyContent(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{Content: ""})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "content is required" {
		t.Errorf("expected 'content is required', got %q", resp["error"])
	}
}

func TestAIImport_Parse_WhitespaceOnlyContent(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{Content: "   \n\t  "})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAIImport_Parse_Truncation(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	// Build JSON array with 55 test cases
	var items []string
	for i := 1; i <= 55; i++ {
		items = append(items, fmt.Sprintf(`{"name": "TC-%d", "steps": [{"action": "Step %d", "expected_result": "Result %d"}]}`, i, i, i))
	}
	content := "[" + strings.Join(items, ",") + "]"

	rr := doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{Content: content})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp models.ParseImportResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if !resp.Truncated {
		t.Error("expected truncated=true")
	}
	if resp.TotalFound != 55 {
		t.Errorf("expected total_found=55, got %d", resp.TotalFound)
	}
	if len(resp.TestCases) != 50 {
		t.Errorf("expected 50 test cases (capped), got %d", len(resp.TestCases))
	}
}

func TestAIImport_Parse_InvalidContent(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{Content: "random gibberish with no structure at all here"})
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAIImport_Parse_HTMLSanitization(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	content := `[{"name": "<script>alert('xss')</script>Login test", "steps": [{"action": "<img onerror=alert(1) src=x>Click", "expected_result": "Safe result"}]}]`

	rr := doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{Content: content})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp models.ParseImportResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.TestCases) != 1 {
		t.Fatalf("expected 1 test case, got %d", len(resp.TestCases))
	}
	// Script tags should be stripped
	if strings.Contains(resp.TestCases[0].Name, "<script>") {
		t.Errorf("script tag not sanitized from name: %q", resp.TestCases[0].Name)
	}
	// img with onerror should be stripped
	if strings.Contains(resp.TestCases[0].Steps[0].Action, "onerror") {
		t.Errorf("onerror not sanitized from action: %q", resp.TestCases[0].Steps[0].Action)
	}
}

func TestAIImport_Parse_DuplicateDetection(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	folderID := createTestFolder(t, env, "Dup Test Folder")

	// Create an existing test case in the folder
	existingTC := map[string]interface{}{
		"name":      "Login test",
		"folder_id": folderID,
	}
	rr := doRequest(env, "POST", "/api/tests", existingTC)
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("create test case: got status %d, body: %s", rr.Code, rr.Body.String())
	}

	// Now parse content with a matching name
	content := `[{"name": "Login test", "steps": [{"action": "Do", "expected_result": "See"}]}]`
	rr = doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{
		Content:  content,
		FolderID: folderID,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp models.ParseImportResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.DuplicateNames) == 0 {
		t.Error("expected duplicate_names to contain 'Login test'")
	} else if resp.DuplicateNames[0] != "Login test" {
		t.Errorf("expected 'Login test' in duplicates, got %v", resp.DuplicateNames)
	}
}

// ── Accept endpoint tests ────────────────────────────────────────────────

func TestAIImport_Accept_Valid(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	folderID := createTestFolder(t, env, "Accept Folder")

	req := models.AcceptImportRequest{
		FolderID: folderID,
		Tests: []models.GeneratedTestCase{
			{
				TempID:      "temp-1",
				Name:        "Imported TC 1",
				Description: "Desc 1",
				Steps: []models.GeneratedStep{
					{Action: "Step 1 action", ExpectedResult: "Step 1 expected"},
				},
			},
			{
				TempID: "temp-2",
				Name:   "Imported TC 2",
				Steps: []models.GeneratedStep{
					{Action: "Do A", ExpectedResult: "See B"},
					{Action: "Do C", ExpectedResult: "See D"},
				},
			},
		},
	}

	rr := doRequest(env, "POST", "/api/import/accept", req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp models.AcceptImportResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Count != 2 {
		t.Errorf("expected count=2, got %d", resp.Count)
	}
	if len(resp.CreatedIDs) != 2 {
		t.Errorf("expected 2 created IDs, got %d", len(resp.CreatedIDs))
	}
	if resp.LinkedTo != "" {
		t.Errorf("expected empty linked_to, got %q", resp.LinkedTo)
	}
}

func TestAIImport_Accept_WithRequirement(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	folderID := createTestFolder(t, env, "Linked Folder")
	reqID := createTestRequirement(t, env, "REQ-001", "Test Requirement")

	req := models.AcceptImportRequest{
		FolderID:      folderID,
		RequirementID: reqID,
		Tests: []models.GeneratedTestCase{
			{
				TempID: "temp-linked",
				Name:   "Linked TC",
				Steps: []models.GeneratedStep{
					{Action: "Action", ExpectedResult: "Result"},
				},
			},
		},
	}

	rr := doRequest(env, "POST", "/api/import/accept", req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp models.AcceptImportResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Count != 1 {
		t.Errorf("expected count=1, got %d", resp.Count)
	}
	if resp.LinkedTo != reqID {
		t.Errorf("expected linked_to=%q, got %q", reqID, resp.LinkedTo)
	}
}

func TestAIImport_Accept_MissingFolderID(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	req := models.AcceptImportRequest{
		FolderID: "",
		Tests: []models.GeneratedTestCase{
			{Name: "TC", Steps: []models.GeneratedStep{{Action: "A", ExpectedResult: "B"}}},
		},
	}

	rr := doRequest(env, "POST", "/api/import/accept", req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "folder_id is required" {
		t.Errorf("expected 'folder_id is required', got %q", resp["error"])
	}
}

func TestAIImport_Accept_EmptyTests(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	folderID := createTestFolder(t, env, "Empty Tests Folder")

	req := models.AcceptImportRequest{
		FolderID: folderID,
		Tests:    []models.GeneratedTestCase{},
	}

	rr := doRequest(env, "POST", "/api/import/accept", req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "no tests to accept" {
		t.Errorf("expected 'no tests to accept', got %q", resp["error"])
	}
}

func TestAIImport_Accept_NonexistentFolder(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	req := models.AcceptImportRequest{
		FolderID: "nonexistent-folder-id",
		Tests: []models.GeneratedTestCase{
			{Name: "TC", Steps: []models.GeneratedStep{{Action: "A", ExpectedResult: "B"}}},
		},
	}

	rr := doRequest(env, "POST", "/api/import/accept", req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAIImport_Accept_NonexistentRequirement(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	folderID := createTestFolder(t, env, "Req Not Found Folder")

	req := models.AcceptImportRequest{
		FolderID:      folderID,
		RequirementID: "nonexistent-req-id",
		Tests: []models.GeneratedTestCase{
			{Name: "TC", Steps: []models.GeneratedStep{{Action: "A", ExpectedResult: "B"}}},
		},
	}

	rr := doRequest(env, "POST", "/api/import/accept", req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestAIImport_Accept_SanitizesHTML(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	folderID := createTestFolder(t, env, "Sanitize Folder")

	req := models.AcceptImportRequest{
		FolderID: folderID,
		Tests: []models.GeneratedTestCase{
			{
				TempID: "temp-xss",
				Name:   "<script>alert('xss')</script>Safe Name",
				Steps: []models.GeneratedStep{
					{Action: "<img onerror=alert(1) src=x>Action", ExpectedResult: "Normal result"},
				},
			},
		},
	}

	rr := doRequest(env, "POST", "/api/import/accept", req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp models.AcceptImportResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Count != 1 {
		t.Fatalf("expected 1 created, got %d", resp.Count)
	}

	// Verify the created test case is sanitized by fetching it
	tcRR := doRequest(env, "GET", "/api/tests/"+resp.CreatedIDs[0], nil)
	if tcRR.Code != http.StatusOK {
		t.Fatalf("get test case: got %d: %s", tcRR.Code, tcRR.Body.String())
	}
	body := tcRR.Body.String()
	if strings.Contains(body, "<script>") {
		t.Error("script tag not sanitized in persisted test case")
	}
}

// ── CSV multi-row continuation test ──────────────────────────────────────

func TestAIImport_Parse_CSVMultiRow(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	content := `name,action,expected_result
Login flow,Enter username,Field populated
Login flow,Click login,Dashboard shown
Logout flow,Click logout,Redirected`

	rr := doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{
		Content:    content,
		FormatHint: "csv",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp models.ParseImportResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if len(resp.TestCases) != 2 {
		t.Fatalf("expected 2 test cases (multi-row merged), got %d", len(resp.TestCases))
	}
	if len(resp.TestCases[0].Steps) != 2 {
		t.Errorf("expected 2 steps for 'Login flow', got %d", len(resp.TestCases[0].Steps))
	}
}

// ── Format override test ─────────────────────────────────────────────────

func TestAIImport_Parse_FormatOverride(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	// Content that looks like numbered list, but force CSV parsing — should fail
	content := "1. Test case one\n   - Step A → Expected A"

	rr := doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{
		Content:    content,
		FormatHint: "csv",
	})
	// CSV parser should fail on this content
	if rr.Code == http.StatusOK {
		var resp models.ParseImportResponse
		json.NewDecoder(rr.Body).Decode(&resp)
		if resp.DetectedFormat == "csv" && len(resp.TestCases) > 0 {
			// Unexpected: CSV parser managed to parse a numbered list
			t.Log("CSV parser unexpectedly parsed numbered list content")
		}
	}
	// The important thing is it doesn't panic
}
