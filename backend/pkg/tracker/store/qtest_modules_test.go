package store

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListQTestModules_BuildsTreeFromFlatResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/projects/42/modules", r.URL.Path)
		require.Equal(t, "descendants", r.URL.Query().Get("expand"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"Platform"},
			{"id":2,"name":"Web","parent_id":1},
			{"id":3,"name":"Auth","parent_id":2}
		]`))
	}))
	defer server.Close()

	s := newTestStore(t)
	modules, err := s.ListQTestModules(&models.QTestConfig{
		BaseURL:  server.URL,
		APIToken: "token",
	}, 42)
	require.NoError(t, err)
	require.Len(t, modules, 1)

	root := modules[0]
	assert.Equal(t, int64(1), root.ID)
	assert.Equal(t, "Platform", root.Path)
	require.Len(t, root.Children, 1)
	assert.Equal(t, int64(2), root.Children[0].ID)
	assert.Equal(t, "Platform / Web", root.Children[0].Path)
	require.Len(t, root.Children[0].Children, 1)
	assert.Equal(t, int64(3), root.Children[0].Children[0].ID)
	assert.Equal(t, "Platform / Web / Auth", root.Children[0].Children[0].Path)
}

func TestListQTestModules_PreservesNestedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"Platform","children":[
				{"id":2,"name":"Web","parent_id":1,"children":[
					{"id":3,"name":"Auth","parent_id":2}
				]}
			]}
		]`))
	}))
	defer server.Close()

	s := newTestStore(t)
	modules, err := s.ListQTestModules(&models.QTestConfig{
		BaseURL:  server.URL,
		APIToken: "token",
	}, 42)
	require.NoError(t, err)
	require.Len(t, modules, 1)
	require.Len(t, modules[0].Children, 1)
	require.Len(t, modules[0].Children[0].Children, 1)
	assert.Equal(t, int64(3), modules[0].Children[0].Children[0].ID)
	assert.Equal(t, "Platform / Web / Auth", modules[0].Children[0].Children[0].Path)
}

func TestListQTestModules_NormalizesMixedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"Platform","children":[{"id":2,"name":"Web","parent_id":1}]},
			{"id":3,"name":"Auth","parent_id":2}
		]`))
	}))
	defer server.Close()

	s := newTestStore(t)
	modules, err := s.ListQTestModules(&models.QTestConfig{
		BaseURL:  server.URL,
		APIToken: "token",
	}, 42)
	require.NoError(t, err)
	require.Len(t, modules, 1)
	require.Len(t, modules[0].Children, 1)
	require.Len(t, modules[0].Children[0].Children, 1)
	assert.Equal(t, int64(3), modules[0].Children[0].Children[0].ID)
	assert.Equal(t, "Platform / Web / Auth", modules[0].Children[0].Children[0].Path)
}

func TestListQTestModules_TreatsOrphansAsRoots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":1,"name":"Platform"},
			{"id":9,"name":"Orphan","parent_id":999}
		]`))
	}))
	defer server.Close()

	s := newTestStore(t)
	modules, err := s.ListQTestModules(&models.QTestConfig{
		BaseURL:  server.URL,
		APIToken: "token",
	}, 42)
	require.NoError(t, err)
	require.Len(t, modules, 2)
	assert.Equal(t, int64(1), modules[0].ID)
	assert.Equal(t, int64(9), modules[1].ID)
	assert.Equal(t, "Orphan", modules[1].Path)
}

func TestListQTestModules_EmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	s := newTestStore(t)
	modules, err := s.ListQTestModules(&models.QTestConfig{
		BaseURL:  server.URL,
		APIToken: "token",
	}, 42)
	require.NoError(t, err)
	assert.Nil(t, modules)
}

func TestFetchQTestTestCases_PaginatesAllPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v3/projects/42/test-cases", r.URL.Path)
		require.Equal(t, "77", r.URL.Query().Get("parentId"))
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "1":
			_, _ = w.Write([]byte(buildQTestCasePageJSON(1, qtestPageSize)))
		case "2":
			_, _ = w.Write([]byte(buildQTestCasePageJSON(101, 2)))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()

	s := newTestStore(t)
	testCases, err := s.FetchQTestTestCases(&models.QTestConfig{
		BaseURL:  server.URL,
		APIToken: "token",
	}, 42, 77)
	require.NoError(t, err)
	require.Len(t, testCases, 102)
	assert.Equal(t, int64(1), testCases[0].ID)
	assert.Equal(t, int64(102), testCases[101].ID)
}

func buildQTestCasePageJSON(startID, count int) string {
	items := make([]string, 0, count)
	for i := 0; i < count; i++ {
		id := startID + i
		items = append(items, fmt.Sprintf(`{"id":%d,"pid":"TC-%d","name":"Case %d","description":"","parent_id":77,"properties":[],"test_steps":[]}`, id, id, id))
	}
	return "[" + strings.Join(items, ",") + "]"
}
