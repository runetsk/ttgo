package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
