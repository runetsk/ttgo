package runs_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Helpers ──────────────────────────────────────────────────────────────────

// createRunFolder creates a run folder via POST /api/run-folders and returns the full response.
func createRunFolder(t *testing.T, env *testEnv, name string, parentID *string) models.RunFolder {
	t.Helper()
	body := map[string]interface{}{"name": name}
	if parentID != nil {
		body["parent_id"] = *parentID
	}
	rr := doRequest(env, "POST", "/api/run-folders", body)
	require.Equal(t, http.StatusCreated, rr.Code, "create run folder %q: %s", name, rr.Body.String())
	var folder models.RunFolder
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&folder))
	require.NotEmpty(t, folder.ID)
	return folder
}

// getRunFolderTree calls GET /api/run-folders?view=tree and returns the decoded tree.
func getRunFolderTree(t *testing.T, env *testEnv) []*models.RunFolder {
	t.Helper()
	rr := doRequest(env, "GET", "/api/run-folders?view=tree", nil)
	require.Equal(t, http.StatusOK, rr.Code, "get tree: %s", rr.Body.String())
	var resp struct {
		RunFolders []*models.RunFolder `json:"run_folders"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	return resp.RunFolders
}

// getRunFoldersFlat calls GET /api/run-folders (flat list).
func getRunFoldersFlat(t *testing.T, env *testEnv) []models.RunFolder {
	t.Helper()
	rr := doRequest(env, "GET", "/api/run-folders", nil)
	require.Equal(t, http.StatusOK, rr.Code, "get flat: %s", rr.Body.String())
	var resp struct {
		RunFolders []models.RunFolder `json:"run_folders"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	return resp.RunFolders
}

// createTestRunInFolder creates a test run in the given folder.
func createTestRunInFolder(t *testing.T, env *testEnv, name, categoryID string, folderID *string) string {
	t.Helper()
	body := map[string]interface{}{"name": name, "category_id": categoryID}
	if folderID != nil {
		body["run_folder_id"] = *folderID
	}
	rr := doRequest(env, "POST", "/api/runs", body)
	require.Equal(t, http.StatusCreated, rr.Code, "create run %q: %s", name, rr.Body.String())
	var resp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.NotEmpty(t, resp.ID)
	return resp.ID
}

// createSuiteForRuns creates a test category and returns its ID.
func createCategoryForRuns(t *testing.T, env *testEnv, name string) string {
	t.Helper()
	body := map[string]string{"name": name}
	rr := doRequest(env, "POST", "/api/categories", body)
	require.Equal(t, http.StatusCreated, rr.Code, "create category %q: %s", name, rr.Body.String())
	var resp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.NotEmpty(t, resp.ID)
	return resp.ID
}

func strPtr(s string) *string { return &s }

// ── Create ───────────────────────────────────────────────────────────────────

func TestRunFolder_Create_Root(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	folder := createRunFolder(t, env, "Smoke", nil)
	assert.Equal(t, "Smoke", folder.Name)
	assert.Nil(t, folder.ParentID)
	assert.Equal(t, 10, folder.DisplayOrder)
}

func TestRunFolder_Create_WithParent(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	parent := createRunFolder(t, env, "Parent", nil)
	child := createRunFolder(t, env, "Child", &parent.ID)

	assert.Equal(t, "Child", child.Name)
	require.NotNil(t, child.ParentID)
	assert.Equal(t, parent.ID, *child.ParentID)
	assert.Equal(t, 10, child.DisplayOrder) // first child in this parent
}

func TestRunFolder_Create_NestedThreeLevels(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	root := createRunFolder(t, env, "Root", nil)
	mid := createRunFolder(t, env, "Mid", &root.ID)
	leaf := createRunFolder(t, env, "Leaf", &mid.ID)

	tree := getRunFolderTree(t, env)
	require.Len(t, tree, 1)
	assert.Equal(t, root.ID, tree[0].ID)

	require.Len(t, tree[0].SubFolders, 1)
	assert.Equal(t, mid.ID, tree[0].SubFolders[0].ID)

	require.Len(t, tree[0].SubFolders[0].SubFolders, 1)
	assert.Equal(t, leaf.ID, tree[0].SubFolders[0].SubFolders[0].ID)
}

func TestRunFolder_Create_InvalidParent(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	body := map[string]interface{}{"name": "Child", "parent_id": "nonexistent-uuid"}
	rr := doRequest(env, "POST", "/api/run-folders", body)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "parent folder not found")
}

func TestRunFolder_Create_EmptyName(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	body := map[string]interface{}{"name": "  "}
	rr := doRequest(env, "POST", "/api/run-folders", body)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "name must be non-empty")
}

func TestRunFolder_Create_DisplayOrderScopedToSiblings(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	// Two root folders
	r1 := createRunFolder(t, env, "Root-A", nil)
	r2 := createRunFolder(t, env, "Root-B", nil)
	assert.Equal(t, 10, r1.DisplayOrder)
	assert.Equal(t, 20, r2.DisplayOrder)

	// Two children of Root-A — order starts fresh
	c1 := createRunFolder(t, env, "Child-1", &r1.ID)
	c2 := createRunFolder(t, env, "Child-2", &r1.ID)
	assert.Equal(t, 10, c1.DisplayOrder)
	assert.Equal(t, 20, c2.DisplayOrder)
}

// ── Tree (GET ?view=tree) ────────────────────────────────────────────────────

func TestRunFolder_Tree_Empty(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	tree := getRunFolderTree(t, env)
	assert.Empty(t, tree)
}

func TestRunFolder_Tree_WithRuns(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	categoryID := createCategoryForRuns(t, env, "Suite")
	root := createRunFolder(t, env, "Root", nil)
	child := createRunFolder(t, env, "Child", &root.ID)

	// Put a run in the child folder
	runID := createTestRunInFolder(t, env, "Run-1", categoryID, &child.ID)

	tree := getRunFolderTree(t, env)
	require.Len(t, tree, 1)
	assert.Equal(t, root.ID, tree[0].ID)
	assert.Empty(t, tree[0].TestRuns, "root should have no direct runs")

	require.Len(t, tree[0].SubFolders, 1)
	childNode := tree[0].SubFolders[0]
	assert.Equal(t, child.ID, childNode.ID)
	require.Len(t, childNode.TestRuns, 1, "child should have 1 run")
	assert.Equal(t, runID, childNode.TestRuns[0].ID)
}

func TestRunFolder_Tree_FlatStillWorks(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	createRunFolder(t, env, "A", nil)
	parent := createRunFolder(t, env, "B", nil)
	createRunFolder(t, env, "B-child", &parent.ID)

	flat := getRunFoldersFlat(t, env)
	assert.Len(t, flat, 3, "flat endpoint should return all folders regardless of hierarchy")
}

// ── Rename ───────────────────────────────────────────────────────────────────

func TestRunFolder_Rename(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	folder := createRunFolder(t, env, "Old", nil)
	rr := doRequest(env, "PATCH", fmt.Sprintf("/api/run-folders/%s", folder.ID), map[string]string{"name": "New"})
	require.Equal(t, http.StatusOK, rr.Code)

	tree := getRunFolderTree(t, env)
	require.Len(t, tree, 1)
	assert.Equal(t, "New", tree[0].Name)
}

func TestRunFolder_Rename_NotFound(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "PATCH", "/api/run-folders/nonexistent", map[string]string{"name": "X"})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ── Move (PATCH /run-folders/{id}/parent) ────────────────────────────────────

func TestRunFolder_Move_IntoAnother(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	a := createRunFolder(t, env, "A", nil)
	b := createRunFolder(t, env, "B", nil)

	// Move B under A
	rr := doRequest(env, "PATCH", fmt.Sprintf("/api/run-folders/%s/parent", b.ID),
		map[string]interface{}{"parent_id": a.ID})
	require.Equal(t, http.StatusOK, rr.Code)

	tree := getRunFolderTree(t, env)
	require.Len(t, tree, 1, "only A at root")
	assert.Equal(t, "A", tree[0].Name)
	require.Len(t, tree[0].SubFolders, 1)
	assert.Equal(t, "B", tree[0].SubFolders[0].Name)
}

func TestRunFolder_Move_ToRoot(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	parent := createRunFolder(t, env, "Parent", nil)
	child := createRunFolder(t, env, "Child", &parent.ID)

	// Move child to root
	rr := doRequest(env, "PATCH", fmt.Sprintf("/api/run-folders/%s/parent", child.ID),
		map[string]interface{}{"parent_id": nil})
	require.Equal(t, http.StatusOK, rr.Code)

	tree := getRunFolderTree(t, env)
	assert.Len(t, tree, 2, "both should be at root")
}

func TestRunFolder_Move_CircularDetection_SelfLoop(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	a := createRunFolder(t, env, "A", nil)

	rr := doRequest(env, "PATCH", fmt.Sprintf("/api/run-folders/%s/parent", a.ID),
		map[string]interface{}{"parent_id": a.ID})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "circular")
}

func TestRunFolder_Move_CircularDetection_Descendant(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	a := createRunFolder(t, env, "A", nil)
	b := createRunFolder(t, env, "B", &a.ID)
	c := createRunFolder(t, env, "C", &b.ID)

	// Try to move A under C (its own grandchild)
	rr := doRequest(env, "PATCH", fmt.Sprintf("/api/run-folders/%s/parent", a.ID),
		map[string]interface{}{"parent_id": c.ID})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "circular")

	// Tree should be unchanged
	tree := getRunFolderTree(t, env)
	require.Len(t, tree, 1)
	assert.Equal(t, "A", tree[0].Name)
}

func TestRunFolder_Move_InvalidParent(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	a := createRunFolder(t, env, "A", nil)

	rr := doRequest(env, "PATCH", fmt.Sprintf("/api/run-folders/%s/parent", a.ID),
		map[string]interface{}{"parent_id": "nonexistent-id"})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "parent folder not found")
}

func TestRunFolder_Move_DeepChain(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	// Build chain: A > B > C > D > E
	a := createRunFolder(t, env, "A", nil)
	b := createRunFolder(t, env, "B", &a.ID)
	c := createRunFolder(t, env, "C", &b.ID)
	d := createRunFolder(t, env, "D", &c.ID)
	e := createRunFolder(t, env, "E", &d.ID)

	// Valid move: E to root
	rr := doRequest(env, "PATCH", fmt.Sprintf("/api/run-folders/%s/parent", e.ID),
		map[string]interface{}{"parent_id": nil})
	require.Equal(t, http.StatusOK, rr.Code)

	tree := getRunFolderTree(t, env)
	assert.Len(t, tree, 2, "A and E at root")

	// Invalid: try to move B under D (D is B's grandchild)
	rr = doRequest(env, "PATCH", fmt.Sprintf("/api/run-folders/%s/parent", b.ID),
		map[string]interface{}{"parent_id": d.ID})
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "circular")
}

// ── Delete ───────────────────────────────────────────────────────────────────

func TestRunFolder_Delete_Leaf(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	folder := createRunFolder(t, env, "Leaf", nil)
	rr := doRequest(env, "DELETE", fmt.Sprintf("/api/run-folders/%s", folder.ID), nil)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	tree := getRunFolderTree(t, env)
	assert.Empty(t, tree)
}

func TestRunFolder_Delete_CascadesSubfolders(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	categoryID := createCategoryForRuns(t, env, "Suite")
	root := createRunFolder(t, env, "Root", nil)
	child := createRunFolder(t, env, "Child", &root.ID)
	grandchild := createRunFolder(t, env, "Grandchild", &child.ID)

	// Put a run in grandchild
	runID := createTestRunInFolder(t, env, "Deep-Run", categoryID, &grandchild.ID)

	// Delete root — should cascade
	rr := doRequest(env, "DELETE", fmt.Sprintf("/api/run-folders/%s", root.ID), nil)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	// All folders gone
	tree := getRunFolderTree(t, env)
	assert.Empty(t, tree)

	flat := getRunFoldersFlat(t, env)
	assert.Empty(t, flat)

	// Run still exists but uncategorised
	runRR := doRequest(env, "GET", fmt.Sprintf("/api/runs/%s", runID), nil)
	require.Equal(t, http.StatusOK, runRR.Code)
	var runResp struct {
		RunFolderID *string `json:"run_folder_id"`
	}
	json.NewDecoder(runRR.Body).Decode(&runResp)
	assert.Nil(t, runResp.RunFolderID, "run should be uncategorised after cascade delete")
}

func TestRunFolder_Delete_OnlyTargetSubtree(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	a := createRunFolder(t, env, "A", nil)
	b := createRunFolder(t, env, "B", &a.ID)
	createRunFolder(t, env, "B-child", &b.ID)
	c := createRunFolder(t, env, "C", &a.ID) // sibling of B

	// Delete B subtree — A and C should survive
	rr := doRequest(env, "DELETE", fmt.Sprintf("/api/run-folders/%s", b.ID), nil)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	tree := getRunFolderTree(t, env)
	require.Len(t, tree, 1, "A survives")
	require.Len(t, tree[0].SubFolders, 1, "only C under A")
	assert.Equal(t, c.ID, tree[0].SubFolders[0].ID)
}

func TestRunFolder_Delete_NotFound(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "DELETE", "/api/run-folders/nonexistent", nil)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ── Reorder ──────────────────────────────────────────────────────────────────

func TestRunFolder_Reorder(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	a := createRunFolder(t, env, "Alpha", nil) // order=10
	b := createRunFolder(t, env, "Beta", nil)  // order=20
	createRunFolder(t, env, "Gamma", nil)      // order=30

	// Move Beta before Alpha
	rr := doRequest(env, "PATCH", fmt.Sprintf("/api/run-folders/%s/order", b.ID),
		map[string]interface{}{"display_order": 5})
	require.Equal(t, http.StatusOK, rr.Code)

	flat := getRunFoldersFlat(t, env)
	require.Len(t, flat, 3)
	assert.Equal(t, "Beta", flat[0].Name, "Beta should be first after reorder")
	assert.Equal(t, "Alpha", flat[1].Name)
	_ = a // used above
}

func TestRunFolder_Reorder_NotFound(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "PATCH", "/api/run-folders/nonexistent/order",
		map[string]interface{}{"display_order": 5})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ── Run assignment via folders ───────────────────────────────────────────────

func TestRunFolder_AssignRunToSubfolder(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	categoryID := createCategoryForRuns(t, env, "Suite")
	root := createRunFolder(t, env, "Root", nil)
	child := createRunFolder(t, env, "Child", &root.ID)

	// Create an unassigned run
	runID := createTestRunInFolder(t, env, "Free-Run", categoryID, nil)

	// Assign to child subfolder
	rr := doRequest(env, "PATCH", fmt.Sprintf("/api/runs/%s/folder", runID),
		map[string]interface{}{"run_folder_id": child.ID})
	require.Equal(t, http.StatusOK, rr.Code)

	// Verify via tree
	tree := getRunFolderTree(t, env)
	require.Len(t, tree, 1)
	childNode := tree[0].SubFolders[0]
	require.Len(t, childNode.TestRuns, 1)
	assert.Equal(t, runID, childNode.TestRuns[0].ID)
}

func TestRunFolder_UnassignRunFromSubfolder(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	categoryID := createCategoryForRuns(t, env, "Suite")
	root := createRunFolder(t, env, "Root", nil)
	child := createRunFolder(t, env, "Child", &root.ID)

	runID := createTestRunInFolder(t, env, "Run-In-Child", categoryID, &child.ID)

	// Unassign (move to uncategorised)
	rr := doRequest(env, "PATCH", fmt.Sprintf("/api/runs/%s/folder", runID),
		map[string]interface{}{"run_folder_id": nil})
	require.Equal(t, http.StatusOK, rr.Code)

	tree := getRunFolderTree(t, env)
	require.Len(t, tree, 1)
	assert.Empty(t, tree[0].SubFolders[0].TestRuns, "child should have no runs after unassign")
}

// ── Comprehensive scenario ───────────────────────────────────────────────────

func TestRunFolder_FullScenario(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	categoryID := createCategoryForRuns(t, env, "Regression Suite")

	// 1. Create folder hierarchy: Regression > Sprint-1 > API-Tests
	regression := createRunFolder(t, env, "Regression", nil)
	sprint1 := createRunFolder(t, env, "Sprint-1", &regression.ID)
	apiTests := createRunFolder(t, env, "API-Tests", &sprint1.ID)

	// 2. Create another root: Smoke
	smoke := createRunFolder(t, env, "Smoke", nil)

	// 3. Create runs at various levels
	run1 := createTestRunInFolder(t, env, "Run-Root", categoryID, &regression.ID)
	run2 := createTestRunInFolder(t, env, "Run-Sprint", categoryID, &sprint1.ID)
	run3 := createTestRunInFolder(t, env, "Run-API", categoryID, &apiTests.ID)
	run4 := createTestRunInFolder(t, env, "Run-Smoke", categoryID, &smoke.ID)

	// 4. Verify tree structure
	tree := getRunFolderTree(t, env)
	require.Len(t, tree, 2, "Regression + Smoke at root")

	regNode := tree[0]
	assert.Equal(t, "Regression", regNode.Name)
	require.Len(t, regNode.TestRuns, 1)
	assert.Equal(t, run1, regNode.TestRuns[0].ID)

	s1Node := regNode.SubFolders[0]
	assert.Equal(t, "Sprint-1", s1Node.Name)
	require.Len(t, s1Node.TestRuns, 1)
	assert.Equal(t, run2, s1Node.TestRuns[0].ID)

	apiNode := s1Node.SubFolders[0]
	assert.Equal(t, "API-Tests", apiNode.Name)
	require.Len(t, apiNode.TestRuns, 1)
	assert.Equal(t, run3, apiNode.TestRuns[0].ID)

	smokeNode := tree[1]
	assert.Equal(t, "Smoke", smokeNode.Name)
	require.Len(t, smokeNode.TestRuns, 1)
	assert.Equal(t, run4, smokeNode.TestRuns[0].ID)

	// 5. Move API-Tests to root
	rr := doRequest(env, "PATCH", fmt.Sprintf("/api/run-folders/%s/parent", apiTests.ID),
		map[string]interface{}{"parent_id": nil})
	require.Equal(t, http.StatusOK, rr.Code)

	tree = getRunFolderTree(t, env)
	assert.Len(t, tree, 3, "Regression, Smoke, API-Tests at root")

	// 6. Rename Smoke
	rr = doRequest(env, "PATCH", fmt.Sprintf("/api/run-folders/%s", smoke.ID),
		map[string]string{"name": "Quick Smoke"})
	require.Equal(t, http.StatusOK, rr.Code)

	// 7. Move run from API-Tests to Smoke
	rr = doRequest(env, "PATCH", fmt.Sprintf("/api/runs/%s/folder", run3),
		map[string]interface{}{"run_folder_id": smoke.ID})
	require.Equal(t, http.StatusOK, rr.Code)

	// 8. Delete Sprint-1 (should cascade, leaving Regression with no subfolders)
	rr = doRequest(env, "DELETE", fmt.Sprintf("/api/run-folders/%s", sprint1.ID), nil)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	tree = getRunFolderTree(t, env)
	assert.Len(t, tree, 3)

	// Find Regression in tree and verify Sprint-1 is gone
	var regFinal *models.RunFolder
	for _, f := range tree {
		if f.ID == regression.ID {
			regFinal = f
			break
		}
	}
	require.NotNil(t, regFinal)
	assert.Empty(t, regFinal.SubFolders, "Sprint-1 was deleted")
	require.Len(t, regFinal.TestRuns, 1, "root-level run still there")

	// 9. Sprint-1's run is now uncategorised
	runRR := doRequest(env, "GET", fmt.Sprintf("/api/runs/%s", run2), nil)
	require.Equal(t, http.StatusOK, runRR.Code)
	var runResp struct {
		RunFolderID *string `json:"run_folder_id"`
	}
	json.NewDecoder(runRR.Body).Decode(&runResp)
	assert.Nil(t, runResp.RunFolderID, "Sprint-1's run should be uncategorised")
}
