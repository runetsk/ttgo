package runs_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedUser creates an active user via the admin API and returns its id.
func seedUser(t *testing.T, env *testEnv, email string) string {
	t.Helper()
	u := createJSON(t, env, "/api/users", map[string]any{
		"email": email, "display_name": email, "password": "password123", "role": "member",
	})
	return u["id"].(string)
}

func TestAssignRunToUser(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	runID, _ := createRunWithCases(t, env, 1)
	userID := seedUser(t, env, "tester1@example.com")

	rr := doRequest(env, http.MethodPut, "/api/runs/"+runID+"/assignee",
		map[string]any{"assignee_id": userID})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	got := doRequest(env, http.MethodGet, "/api/runs/"+runID, nil)
	require.Equal(t, http.StatusOK, got.Code)
	assert.Contains(t, got.Body.String(), userID)
}

func TestAssignRunClears(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	runID, _ := createRunWithCases(t, env, 1)
	userID := seedUser(t, env, "tester2@example.com")
	doRequest(env, http.MethodPut, "/api/runs/"+runID+"/assignee", map[string]any{"assignee_id": userID})

	rr := doRequest(env, http.MethodPut, "/api/runs/"+runID+"/assignee",
		map[string]any{"assignee_id": nil})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

func TestAssignRunRejectsUnknownUser(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	runID, _ := createRunWithCases(t, env, 1)

	rr := doRequest(env, http.MethodPut, "/api/runs/"+runID+"/assignee",
		map[string]any{"assignee_id": "no-such-user"})
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

func TestAssignRunUnknownRun404(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	userID := seedUser(t, env, "tester3@example.com")

	rr := doRequest(env, http.MethodPut, "/api/runs/no-such-run/assignee",
		map[string]any{"assignee_id": userID})
	assert.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}

func TestListRunsFilterByAssignee(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	run1, _ := createRunWithCases(t, env, 1)
	run2, _ := createRunWithCases(t, env, 1)
	userID := seedUser(t, env, "filt@example.com")
	doRequest(env, http.MethodPut, "/api/runs/"+run1+"/assignee", map[string]any{"assignee_id": userID})

	rr := doRequest(env, http.MethodGet, "/api/runs?assignee_id="+userID, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), run1)
	assert.NotContains(t, rr.Body.String(), run2)
	assert.Contains(t, rr.Body.String(), "filt@example.com")

	un := doRequest(env, http.MethodGet, "/api/runs?assignee_id=unassigned", nil)
	require.Equal(t, http.StatusOK, un.Code)
	assert.Contains(t, un.Body.String(), run2)
	assert.NotContains(t, un.Body.String(), run1)
}

func TestListRunsAssignedToMe(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	runID, _ := createRunWithCases(t, env, 1)

	me := doRequest(env, http.MethodGet, "/api/auth/me", nil)
	require.Equal(t, http.StatusOK, me.Code)
	var meBody struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	require.NoError(t, json.Unmarshal(me.Body.Bytes(), &meBody))
	require.NotEmpty(t, meBody.User.ID)
	doRequest(env, http.MethodPut, "/api/runs/"+runID+"/assignee", map[string]any{"assignee_id": meBody.User.ID})

	rr := doRequest(env, http.MethodGet, "/api/runs?assignee_id=me", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), runID)
}
