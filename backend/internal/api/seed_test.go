package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
