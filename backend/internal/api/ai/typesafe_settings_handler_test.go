package ai

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
	"ttgo/pkg/tracker/typesafe"

	"github.com/microcosm-cc/bluemonday"
	"github.com/stretchr/testify/require"
)

type listOnlyClient struct{ models []typesafe.Model }

func (l listOnlyClient) Evaluate(context.Context, typesafe.Request) (*typesafe.Response, error) {
	return nil, nil
}
func (l listOnlyClient) ListModels(context.Context) ([]typesafe.Model, error) { return l.models, nil }

func TestTestTypeSafeConnection_ListsModels(t *testing.T) {
	t.Chdir(t.TempDir())
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()
	key := "ts-key-0001"
	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{APIKey: &key})
	require.NoError(t, err)

	h := NewHandler(s, bluemonday.UGCPolicy())
	var gotKey string
	h.SetTypeSafeClientFactory(func(apiKey string, _ time.Duration) typesafe.Client {
		gotKey = apiKey
		return listOnlyClient{models: []typesafe.Model{{Name: "jev-latest"}}}
	})
	rr := httptest.NewRecorder()
	h.TestTypeSafeConnection(rr, httptest.NewRequest("POST", "/api/settings/typesafe/test", nil))
	require.Equal(t, 200, rr.Code)
	var out map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	require.Equal(t, true, out["ok"])
	require.Equal(t, "ts-key-0001", gotKey)
	require.Len(t, out["models"], 1)
}

type failingClient struct{ err error }

func (f failingClient) Evaluate(context.Context, typesafe.Request) (*typesafe.Response, error) {
	return nil, f.err
}
func (f failingClient) ListModels(context.Context) ([]typesafe.Model, error) { return nil, f.err }

func TestTestTypeSafeConnection_MapsVendorErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()
	key := "ts-key-0001"
	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{APIKey: &key})
	require.NoError(t, err)

	h := NewHandler(s, bluemonday.UGCPolicy())
	h.SetTypeSafeClientFactory(func(string, time.Duration) typesafe.Client {
		return failingClient{err: &typesafe.Error{Status: 429, Category: typesafe.CategoryRateLimit, Message: "slow down"}}
	})
	rr := httptest.NewRecorder()
	h.TestTypeSafeConnection(rr, httptest.NewRequest("POST", "/api/settings/typesafe/test", nil))
	require.Equal(t, 200, rr.Code, "vendor errors never become 5xx")
	var out map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	require.Equal(t, false, out["ok"])
	require.Equal(t, "rate_limit", out["category"])
	require.Equal(t, "slow down", out["message"])
}
