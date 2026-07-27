package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
)

// assigneeUser inserts a user and forces the active/deleted flags CreateUser does not expose.
func assigneeUser(t *testing.T, s *store.Store, email string, active, deleted bool) *models.User {
	t.Helper()
	u, err := s.CreateUser(email, "Assignee "+email, "hash", "member")
	require.NoError(t, err)
	_, err = s.UpdateUser(u.ID, map[string]interface{}{"active": active, "deleted": deleted})
	require.NoError(t, err)
	return u
}

// doJSON issues an authenticated JSON request against the mounted API and returns the recorder.
func doJSON(t *testing.T, s *store.Store, srv *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	addTestAuth(t, s, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func decodeDefect(t *testing.T, w *httptest.ResponseRecorder) models.Defect {
	t.Helper()
	var d models.Defect
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &d), "body: %s", w.Body.String())
	return d
}

// createDefectWithAssignee posts a defect through the API and returns the created row.
func createDefectWithAssignee(t *testing.T, s *store.Store, srv *Server, assigneeID string) models.Defect {
	t.Helper()
	w := doJSON(t, s, srv, http.MethodPost, "/api/defects",
		`{"title":"Checkout 500","assignee_id":"`+assigneeID+`"}`)
	require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())
	return decodeDefect(t, w)
}

func TestCreateDefect_AssigneeValidation(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	active := assigneeUser(t, s, "active@example.com", true, false)
	inactive := assigneeUser(t, s, "inactive@example.com", false, false)
	deleted := assigneeUser(t, s, "deleted@example.com", true, true)

	t.Run("active user is accepted and persisted", func(t *testing.T) {
		d := createDefectWithAssignee(t, s, srv, active.ID)
		require.NotNil(t, d.AssigneeID)
		assert.Equal(t, active.ID, *d.AssigneeID)

		stored, err := s.GetDefect(d.ID)
		require.NoError(t, err)
		require.NotNil(t, stored.AssigneeID)
		assert.Equal(t, active.ID, *stored.AssigneeID)
	})

	t.Run("empty string creates an unassigned defect", func(t *testing.T) {
		w := doJSON(t, s, srv, http.MethodPost, "/api/defects", `{"title":"No owner","assignee_id":""}`)
		require.Equal(t, http.StatusCreated, w.Code)
		assert.Nil(t, decodeDefect(t, w).AssigneeID)
	})

	for name, id := range map[string]string{
		"unknown user":  "does-not-exist",
		"inactive user": inactive.ID,
		"deleted user":  deleted.ID,
	} {
		t.Run(name+" is rejected", func(t *testing.T) {
			w := doJSON(t, s, srv, http.MethodPost, "/api/defects", `{"title":"Bad owner","assignee_id":"`+id+`"}`)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
		})
	}
}

func TestUpdateDefect_AssigneeValidation(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	active := assigneeUser(t, s, "owner@example.com", true, false)
	inactive := assigneeUser(t, s, "gone@example.com", false, false)
	deleted := assigneeUser(t, s, "removed@example.com", true, true)

	t.Run("assigning an active user succeeds", func(t *testing.T) {
		d := createDefectWithAssignee(t, s, srv, "")
		w := doJSON(t, s, srv, http.MethodPatch, "/api/defects/"+d.ID, `{"assignee_id":"`+active.ID+`"}`)
		assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	})

	t.Run("empty string is accepted as an unassign", func(t *testing.T) {
		// the store-side clearing is covered by the store tests; here we only assert the
		// handler treats "" as a valid value rather than an unknown-user 400.
		d := createDefectWithAssignee(t, s, srv, active.ID)
		w := doJSON(t, s, srv, http.MethodPatch, "/api/defects/"+d.ID, `{"assignee_id":""}`)
		assert.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	})

	t.Run("omitting assignee_id leaves the assignee untouched", func(t *testing.T) {
		d := createDefectWithAssignee(t, s, srv, active.ID)
		w := doJSON(t, s, srv, http.MethodPatch, "/api/defects/"+d.ID, `{"title":"Renamed"}`)
		require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
		got := decodeDefect(t, w)
		assert.Equal(t, "Renamed", got.Title)
		require.NotNil(t, got.AssigneeID)
		assert.Equal(t, active.ID, *got.AssigneeID)
	})

	for name, id := range map[string]string{
		"unknown user":  "does-not-exist",
		"inactive user": inactive.ID,
		"deleted user":  deleted.ID,
	} {
		t.Run(name+" is rejected", func(t *testing.T) {
			d := createDefectWithAssignee(t, s, srv, "")
			w := doJSON(t, s, srv, http.MethodPatch, "/api/defects/"+d.ID, `{"assignee_id":"`+id+`"}`)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
		})
	}

	t.Run("validation runs before the store, so an unknown defect still 400s", func(t *testing.T) {
		w := doJSON(t, s, srv, http.MethodPatch, "/api/defects/nope", `{"assignee_id":"does-not-exist"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// The create-and-link path shares ValidateCreate/DefectFromCreate with POST /defects,
// so it must validate and persist the assignee without any handler-level check of its own.
func TestCreateAndLinkResultDefect_Assignee(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := NewServer(s)

	active := assigneeUser(t, s, "linked@example.com", true, false)
	require.NoError(t, s.DB().Exec(
		`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','Login works',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, s.DB().Exec(`INSERT INTO test_runs (id,name) VALUES ('run1','R1')`).Error)
	require.NoError(t, s.DB().Exec(
		`INSERT INTO run_results (id,test_run_id,test_case_id,status) VALUES ('rr1','run1','tc1','FAIL')`).Error)

	const path = "/api/runs/run1/results/rr1/defects"

	t.Run("persists a valid assignee", func(t *testing.T) {
		w := doJSON(t, s, srv, http.MethodPost, path, `{"title":"Login 500","assignee_id":"`+active.ID+`"}`)
		require.Equal(t, http.StatusCreated, w.Code, "body: %s", w.Body.String())

		var resp struct {
			Defect models.Defect `json:"defect"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.NotNil(t, resp.Defect.AssigneeID)
		assert.Equal(t, active.ID, *resp.Defect.AssigneeID)

		stored, err := s.GetDefect(resp.Defect.ID)
		require.NoError(t, err)
		require.NotNil(t, stored.AssigneeID)
		assert.Equal(t, active.ID, *stored.AssigneeID)
	})

	t.Run("rejects an unknown assignee", func(t *testing.T) {
		w := doJSON(t, s, srv, http.MethodPost, path, `{"title":"Login 500","assignee_id":"ghost"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code, "body: %s", w.Body.String())
	})
}
