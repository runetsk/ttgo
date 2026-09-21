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

// probeClient lists no models (what a gateway such as OpenRouter returns for
// /v1/models) and records the Evaluate request the handler falls back to.
type probeClient struct {
	got *typesafe.Request
	err error
}

func (p *probeClient) ListModels(context.Context) ([]typesafe.Model, error) { return nil, nil }
func (p *probeClient) Evaluate(_ context.Context, req typesafe.Request) (*typesafe.Response, error) {
	p.got = &req
	if p.err != nil {
		return nil, p.err
	}
	answers := map[string]typesafe.Answer{}
	for id := range req.Questions {
		answers[id] = typesafe.Answer{Type: "noul", Noul: 0.99, Confidence: 0.9}
	}
	return &typesafe.Response{Model: req.Model, Answers: answers}, nil
}

func TestTestTypeSafeConnection_ProbesEvaluateWhenNoModelsListed(t *testing.T) {
	t.Chdir(t.TempDir())
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()
	key, model := "or-key-0001", "jev-1.13"
	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{APIKey: &key, Model: &model})
	require.NoError(t, err)

	h := NewHandler(s, bluemonday.UGCPolicy())
	pc := &probeClient{}
	h.SetTypeSafeClientFactory(func(string, time.Duration) typesafe.Client { return pc })
	rr := httptest.NewRecorder()
	h.TestTypeSafeConnection(rr, httptest.NewRequest("POST", "/api/settings/typesafe/test", nil))
	require.Equal(t, 200, rr.Code)
	var out map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	require.Equal(t, true, out["ok"])
	require.Len(t, out["models"], 1, "the probed model is reported as the only model")
	require.Equal(t, "jev-1.13", out["models"].([]any)[0].(map[string]any)["name"])
	require.NotNil(t, pc.got, "an Evaluate probe must be sent when the model list is empty")
	require.Equal(t, "jev-1.13", pc.got.Model)
	require.Len(t, pc.got.Questions, 1, "the probe asks exactly one question")
}

func TestTestTypeSafeConnection_ProbeFailureIsReported(t *testing.T) {
	t.Chdir(t.TempDir())
	s, err := store.New(":memory:")
	require.NoError(t, err)
	defer s.Close()
	key := "or-key-0001"
	_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{APIKey: &key})
	require.NoError(t, err)

	h := NewHandler(s, bluemonday.UGCPolicy())
	pc := &probeClient{err: &typesafe.Error{Status: 401, Category: typesafe.CategoryAuth, Message: "bad key"}}
	h.SetTypeSafeClientFactory(func(string, time.Duration) typesafe.Client { return pc })
	rr := httptest.NewRecorder()
	h.TestTypeSafeConnection(rr, httptest.NewRequest("POST", "/api/settings/typesafe/test", nil))
	require.Equal(t, 200, rr.Code)
	var out map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &out))
	require.Equal(t, false, out["ok"], "an empty model list must not pass as connected")
	require.Equal(t, string(typesafe.CategoryAuth), out["category"])
	require.Equal(t, "bad key", out["message"])
}
