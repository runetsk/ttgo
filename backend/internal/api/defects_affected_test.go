package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func TestGetDefectAffectedTests_EmptyForUnknownDefect(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	req := httptest.NewRequest("GET", "/api/defects/does-not-exist/tests", nil)
	addTestAuth(t, s, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	// must be an empty JSON array, never null
	assert.Equal(t, "[]", strings.TrimSpace(w.Body.String()))
}

// affectedTestRow mirrors the JSON the endpoint emits, so the assertions below fail if a field is
// renamed or dropped from store.AffectedTestCase — the row expand in the defects queue reads these
// keys to link each affected test back to the run it last failed in.
type affectedTestRow struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	LastRunID        string `json:"last_run_id"`
	LastRunName      string `json:"last_run_name"`
	LastResultStatus string `json:"last_result_status"`
}

// TestGetDefectAffectedTests_CarriesLastFailingRun proves the run enrichment survives the handler's
// JSON encoding, and that a test case with no failing result is still returned — with empty run
// fields rather than dropped from the list.
func TestGetDefectAffectedTests_CarriesLastFailingRun(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	folder, err := s.CreateFolder("Checkout", nil)
	require.NoError(t, err)
	failing := &models.TestCase{Name: "A failing test", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(failing))
	neverRun := &models.TestCase{Name: "B never run", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(neverRun))

	run := &models.TestRun{Name: "Nightly regression"}
	require.NoError(t, s.CreateTestRun(run))
	result := &models.RunResult{
		TestRunID: run.ID, TestCaseID: &failing.ID,
		Status: models.StatusFail, StartTime: time.Now(),
	}
	require.NoError(t, s.AddRunResult(result))

	d := &models.Defect{Title: "checkout 500"}
	require.NoError(t, s.CreateDefect(d))
	_, err = s.LinkDefectToResult(d.ID, result.ID, failing.ID)
	require.NoError(t, err)
	_, err = s.LinkDefectToTestCase(d.ID, neverRun.ID) // case-scoped: carries no run result at all
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/defects/"+d.ID+"/tests", nil)
	addTestAuth(t, s, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var got []affectedTestRow
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 2, "both affected test cases are listed, ordered by name")

	assert.Equal(t, failing.ID, got[0].ID)
	assert.Equal(t, run.ID, got[0].LastRunID)
	assert.Equal(t, "Nightly regression", got[0].LastRunName)
	assert.Equal(t, "FAIL", got[0].LastResultStatus)

	assert.Equal(t, neverRun.ID, got[1].ID)
	assert.Empty(t, got[1].LastRunID)
	assert.Empty(t, got[1].LastRunName)
	assert.Empty(t, got[1].LastResultStatus)
}
