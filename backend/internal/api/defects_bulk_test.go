package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
)

const bulkPath = "/api/defects/bulk-update"

func decodeDefects(t *testing.T, w *httptest.ResponseRecorder) []models.Defect {
	t.Helper()
	var out []models.Defect
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), "body: %s", w.Body.String())
	return out
}

// byID indexes a response slice so assertions do not depend on the returned order.
func byID(rows []models.Defect) map[string]models.Defect {
	out := make(map[string]models.Defect, len(rows))
	for _, d := range rows {
		out[d.ID] = d
	}
	return out
}

// newDefect posts a plain defect and returns it.
func newDefect(t *testing.T, s *store.Store, srv *Server, title string) models.Defect {
	t.Helper()
	w := doJSON(t, s, srv, http.MethodPost, "/api/defects", `{"title":"`+title+`","severity":"minor"}`)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
	return decodeDefect(t, w)
}

func TestBulkUpdateDefects_Handler(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	owner := assigneeUser(t, s, "bulk-owner@example.com", true, false)

	t.Run("bulk status change", func(t *testing.T) {
		a, b := newDefect(t, s, srv, "Cart 500"), newDefect(t, s, srv, "Search 500")

		w := doJSON(t, s, srv, http.MethodPost, bulkPath,
			`{"ids":["`+a.ID+`","`+b.ID+`"],"status":"fixed"}`)
		require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

		got := byID(decodeDefects(t, w))
		require.Len(t, got, 2)
		assert.Equal(t, "fixed", got[a.ID].Status)
		assert.Equal(t, "fixed", got[b.ID].Status)

		stored, err := s.GetDefect(b.ID)
		require.NoError(t, err)
		assert.Equal(t, "fixed", stored.Status)
	})

	t.Run("bulk assign resolves assignee_name on the response", func(t *testing.T) {
		a, b := newDefect(t, s, srv, "Login flake"), newDefect(t, s, srv, "Logout flake")

		w := doJSON(t, s, srv, http.MethodPost, bulkPath,
			`{"ids":["`+a.ID+`","`+b.ID+`"],"assignee_id":"`+owner.ID+`"}`)
		require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

		got := byID(decodeDefects(t, w))
		for _, id := range []string{a.ID, b.ID} {
			require.NotNil(t, got[id].AssigneeID, "defect %s should be assigned", id)
			assert.Equal(t, owner.ID, *got[id].AssigneeID)
			// the response must be shaped like GET /defects, so the page can patch rows in place
			assert.Equal(t, owner.DisplayName, got[id].AssigneeName)
		}
	})

	t.Run("bulk unassign with an empty string", func(t *testing.T) {
		d := createDefectWithAssignee(t, s, srv, owner.ID)

		w := doJSON(t, s, srv, http.MethodPost, bulkPath, `{"ids":["`+d.ID+`"],"assignee_id":""}`)
		require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

		rows := decodeDefects(t, w)
		require.Len(t, rows, 1)
		assert.Nil(t, rows[0].AssigneeID)
		assert.Empty(t, rows[0].AssigneeName)
	})

	t.Run("bulk severity change", func(t *testing.T) {
		a, b := newDefect(t, s, srv, "Timeout A"), newDefect(t, s, srv, "Timeout B")

		w := doJSON(t, s, srv, http.MethodPost, bulkPath,
			`{"ids":["`+a.ID+`","`+b.ID+`"],"severity":"critical"}`)
		require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

		got := byID(decodeDefects(t, w))
		assert.Equal(t, "critical", got[a.ID].Severity)
		assert.Equal(t, "critical", got[b.ID].Severity)
	})

	t.Run("combined status, severity and assignee in one call", func(t *testing.T) {
		d := newDefect(t, s, srv, "Everything at once")

		w := doJSON(t, s, srv, http.MethodPost, bulkPath,
			`{"ids":["`+d.ID+`"],"status":"closed","severity":"major","assignee_id":"`+owner.ID+`"}`)
		require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

		rows := decodeDefects(t, w)
		require.Len(t, rows, 1)
		assert.Equal(t, "closed", rows[0].Status)
		assert.Equal(t, "major", rows[0].Severity)
		require.NotNil(t, rows[0].AssigneeID)
		assert.Equal(t, owner.ID, *rows[0].AssigneeID)
	})

	t.Run("unknown ids are tolerated alongside real ones", func(t *testing.T) {
		d := newDefect(t, s, srv, "Real one")

		w := doJSON(t, s, srv, http.MethodPost, bulkPath,
			`{"ids":["`+d.ID+`","ghost-1","ghost-2"],"status":"closed"}`)
		require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

		rows := decodeDefects(t, w)
		require.Len(t, rows, 1, "only defects that exist come back")
		assert.Equal(t, d.ID, rows[0].ID)
		assert.Equal(t, "closed", rows[0].Status)
	})

	t.Run("a selection of only unknown ids is an empty array, never null", func(t *testing.T) {
		w := doJSON(t, s, srv, http.MethodPost, bulkPath, `{"ids":["ghost"],"status":"closed"}`)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "[]", strings.TrimSpace(w.Body.String()))
	})
}

func TestBulkUpdateDefects_Validation(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	inactive := assigneeUser(t, s, "bulk-inactive@example.com", false, false)
	deleted := assigneeUser(t, s, "bulk-deleted@example.com", true, true)
	d := newDefect(t, s, srv, "Subject")

	overCap := make([]string, httpx.MaxBulkIDs+1)
	for i := range overCap {
		overCap[i] = fmt.Sprintf(`"id-%d"`, i)
	}

	for name, body := range map[string]string{
		"empty id list":     `{"ids":[],"status":"closed"}`,
		"missing ids field": `{"status":"closed"}`,
		"over the 500 cap":  `{"ids":[` + strings.Join(overCap, ",") + `],"status":"closed"}`,
		"invalid status":    `{"ids":["` + d.ID + `"],"status":"wontfix"}`,
		"empty status":      `{"ids":["` + d.ID + `"],"status":""}`,
		"invalid severity":  `{"ids":["` + d.ID + `"],"severity":"blocker"}`,
		"unknown assignee":  `{"ids":["` + d.ID + `"],"assignee_id":"does-not-exist"}`,
		"inactive assignee": `{"ids":["` + d.ID + `"],"assignee_id":"` + inactive.ID + `"}`,
		"deleted assignee":  `{"ids":["` + d.ID + `"],"assignee_id":"` + deleted.ID + `"}`,
		"malformed body":    `{ids:`,
	} {
		t.Run(name+" is rejected", func(t *testing.T) {
			w := doJSON(t, s, srv, http.MethodPost, bulkPath, body)
			require.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())

			var resp map[string]string
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.NotEmpty(t, resp["error"], "a 400 must name the problem")
		})
	}

	t.Run("nothing was written by any rejected request", func(t *testing.T) {
		stored, err := s.GetDefect(d.ID)
		require.NoError(t, err)
		assert.Equal(t, "open", stored.Status)
		assert.Equal(t, "minor", stored.Severity)
		assert.Nil(t, stored.AssigneeID)
	})
}

// The exact cap boundary: MaxBulkIDs ids is accepted, one more is not. Pinned separately because
// the check is an easy off-by-one and the rejected side is covered above.
func TestBulkUpdateDefects_AcceptsExactlyMaxBulkIDs(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	real := newDefect(t, s, srv, "Only real one")
	ids := make([]string, 0, httpx.MaxBulkIDs)
	ids = append(ids, `"`+real.ID+`"`)
	for i := len(ids); i < httpx.MaxBulkIDs; i++ {
		ids = append(ids, fmt.Sprintf(`"filler-%d"`, i))
	}

	w := doJSON(t, s, srv, http.MethodPost, bulkPath,
		`{"ids":[`+strings.Join(ids, ",")+`],"status":"fixed"}`)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	rows := decodeDefects(t, w)
	require.Len(t, rows, 1)
	assert.Equal(t, "fixed", rows[0].Status)
}
