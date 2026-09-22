package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandleResetAllData_ConfirmationContract pins the destructive-reset
// contract the frontend depends on: DELETE /api/admin/reset erases everything
// ONLY when the body carries {"confirm":"CONFIRM RESET"}. A missing or wrong
// confirmation is refused with 400 and MUST leave data untouched.
//
// Regression guard: the Settings "type ERASE → Erase Everything" flow once sent
// no body, so every reset 400'd and nothing was erased.
func TestHandleResetAllData_ConfirmationContract(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	// Arrange: load demo data so there is something to erase.
	_, err = s.SeedDemoTx(false)
	require.NoError(t, err)
	status, err := s.GetSeedStatus()
	require.NoError(t, err)
	require.True(t, status.HasDemoData, "demo data should be present before reset")

	reset := func(body string) *httptest.ResponseRecorder {
		var r *http.Request
		if body == "" {
			r = httptest.NewRequest(http.MethodDelete, "/api/admin/reset", nil)
		} else {
			r = httptest.NewRequest(http.MethodDelete, "/api/admin/reset", strings.NewReader(body))
		}
		rr := httptest.NewRecorder()
		srv.handleResetAllData(rr, r)
		return rr
	}

	// No body (what a missing confirmation looks like) → 400, data intact.
	rr := reset("")
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "CONFIRM RESET")
	status, err = s.GetSeedStatus()
	require.NoError(t, err)
	assert.True(t, status.HasDemoData, "a refused reset must not erase data")

	// Wrong confirmation string → 400, data intact.
	rr = reset(`{"confirm":"ERASE"}`)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
	status, err = s.GetSeedStatus()
	require.NoError(t, err)
	assert.True(t, status.HasDemoData, "a wrong confirmation must not erase data")

	// Correct confirmation → 200, everything erased.
	rr = reset(`{"confirm":"CONFIRM RESET"}`)
	assert.Equal(t, http.StatusOK, rr.Code)
	status, err = s.GetSeedStatus()
	require.NoError(t, err)
	assert.False(t, status.HasDemoData, "the confirmed reset must erase all data")
}

func TestHandleGetAISeedStatus_ReportsAnswerKey(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	get := func() (int, map[string]any) {
		rr := httptest.NewRecorder()
		srv.handleGetAISeedStatus(rr, httptest.NewRequest(http.MethodGet, "/api/seed/ai", nil))
		var out map[string]any
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
		return rr.Code, out
	}

	code, out := get()
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, false, out["loaded"], "nothing seeded yet")
	assert.Equal(t, store.AIDemoLatestRunID(), out["latest_run_id"])
	gt, _ := out["ground_truth"].([]any)
	require.NotEmpty(t, gt, "the answer key is static and available before seeding")
	first := gt[0].(map[string]any)
	assert.NotEmpty(t, first["template_key"])
	assert.NotEmpty(t, first["sample_message"])
	assert.NotEmpty(t, first["expected_verdict"])
	assert.NotEmpty(t, first["expected_defect_type"])

	_, err = s.SeedAIDemoTx()
	require.NoError(t, err)
	code, out = get()
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, out["loaded"])
}

func TestHandleCreateAISeed_FailureScale(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)
	post := func(body string) (int, map[string]any) {
		var r *http.Request
		if body == "" {
			r = httptest.NewRequest(http.MethodPost, "/api/seed/ai", nil)
		} else {
			r = httptest.NewRequest(http.MethodPost, "/api/seed/ai", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
		}
		rr := httptest.NewRecorder()
		srv.handleCreateAISeed(rr, r)
		var out map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return rr.Code, out
	}

	code, out := post(`{"failure_scale": 0}`)
	assert.Equal(t, http.StatusBadRequest, code, "scale below 1 is refused: %v", out)
	code, out = post(`{"failure_scale": 6}`)
	assert.Equal(t, http.StatusBadRequest, code, "scale above 5 is refused: %v", out)
	code, out = post(`{"failure_scale": "x"}`)
	assert.Equal(t, http.StatusBadRequest, code, "malformed body is refused: %v", out)

	code, base := post("")
	require.Equal(t, http.StatusCreated, code, "no body keeps the default dataset: %v", base)
	assert.EqualValues(t, 1, base["failure_scale"])

	code, scaled := post(`{"failure_scale": 2}`)
	require.Equal(t, http.StatusCreated, code, "%v", scaled)
	assert.EqualValues(t, 2, scaled["failure_scale"])
	assert.Greater(t, scaled["failing_rows"].(float64), base["failing_rows"].(float64)*1.5, "scale 2 plants many more failures")
}

func TestHandleCreateAISeed_LogWords(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)
	post := func(body string) (int, map[string]any) {
		r := httptest.NewRequest(http.MethodPost, "/api/seed/ai", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		srv.handleCreateAISeed(rr, r)
		var out map[string]any
		_ = json.Unmarshal(rr.Body.Bytes(), &out)
		return rr.Code, out
	}
	code, out := post(`{"log_words": 20001}`)
	assert.Equal(t, http.StatusBadRequest, code, "%v", out)

	code, out = post(`{"failure_scale": 1, "log_words": 1000}`)
	require.Equal(t, http.StatusCreated, code, "%v", out)
	assert.EqualValues(t, 1000, out["log_words"])
	rows, err := s.ListLatestFailingResults(store.AIDemoLatestRunID())
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	assert.GreaterOrEqual(t, len(strings.Fields(rows[0].LogText)), 800, "seeded failures carry a log of the requested size")
}
