package ai

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
	"ttgo/pkg/tracker/typesafe"

	"github.com/microcosm-cc/bluemonday"
	"github.com/stretchr/testify/require"
)

// useClient is a fake TypeSafe client for the uses beyond failure analysis: fn answers each
// request, and every request is kept.
type useClient struct {
	mu   sync.Mutex
	fn   func(req typesafe.Request) (*typesafe.Response, error)
	reqs []typesafe.Request
}

func (c *useClient) Evaluate(_ context.Context, req typesafe.Request) (*typesafe.Response, error) {
	c.mu.Lock()
	c.reqs = append(c.reqs, req)
	c.mu.Unlock()
	return c.fn(req)
}
func (c *useClient) ListModels(context.Context) ([]typesafe.Model, error) { return nil, nil }

func (c *useClient) requests() []typesafe.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]typesafe.Request(nil), c.reqs...)
}

type useEnv struct {
	s      *store.Store
	h      *Handler
	client *useClient
}

// newUseEnv builds a handler on a fresh store; enabled switches TypeSafe on with a key.
func newUseEnv(t *testing.T, enabled bool, fn func(req typesafe.Request) (*typesafe.Response, error)) *useEnv {
	t.Helper()
	t.Chdir(t.TempDir())
	s, err := store.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	if enabled {
		key, on := "ts-key-0001", true
		_, err = s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{APIKey: &key, Enabled: &on})
		require.NoError(t, err)
	}
	h := NewHandler(s, bluemonday.UGCPolicy())
	c := &useClient{fn: fn}
	h.SetTypeSafeClientFactory(func(string, time.Duration) typesafe.Client { return c })
	return &useEnv{s: s, h: h, client: c}
}

// answerAll answers every question: choices with pick(id) at 0.9, nouls with noul(id).
func answerAll(pick func(id string, q typesafe.Question) string, noul func(id string) float64) func(req typesafe.Request) (*typesafe.Response, error) {
	return func(req typesafe.Request) (*typesafe.Response, error) {
		resp := &typesafe.Response{Model: "jev-1.13.0", Answers: map[string]typesafe.Answer{}, Usage: typesafe.Usage{InputTokens: 1000}}
		for id, q := range req.Questions {
			if q.Type == "choice" {
				c := pick(id, q)
				probs := map[string]float64{}
				for k := range q.Criteria.(map[string]any) {
					probs[k] = 0.1 / float64(len(q.Criteria.(map[string]any))-1)
				}
				probs[c] = 0.9
				resp.Answers[id] = typesafe.Answer{Type: "choice", Choice: c, Confidence: 0.9, Probabilities: probs}
				continue
			}
			resp.Answers[id] = typesafe.Answer{Type: "noul", Noul: noul(id)}
		}
		return resp, nil
	}
}

func TestTypeSafeUses_Availability(t *testing.T) {
	off := newUseEnv(t, false, nil)
	rr := httptest.NewRecorder()
	off.h.GetTypeSafeUses(rr, httptest.NewRequest("GET", "/api/ai/typesafe/features", nil))
	var got map[string]bool
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Equal(t, map[string]bool{"import_structure": false, "draft_review": false, "defect_assist": false, "search_rerank": false}, got)

	on := newUseEnv(t, true, nil)
	rr = httptest.NewRecorder()
	on.h.GetTypeSafeUses(rr, httptest.NewRequest("GET", "/api/ai/typesafe/features", nil))
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Equal(t, map[string]bool{"import_structure": true, "draft_review": true, "defect_assist": true, "search_rerank": false}, got,
		"search re-ranking is off by default: it costs a request per search")

	no := false
	_, err := on.s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{DefectAssistEnabled: &no})
	require.NoError(t, err)
	u, why := on.h.TypeSafeFor(TypeSafeUseDefectAssist)
	require.Nil(t, u)
	require.Contains(t, why, "switched off in its settings")

	_, err = on.s.UpdateAIFeatureSettings(false)
	require.NoError(t, err)
	u, why = on.h.TypeSafeFor(TypeSafeUseImportStructure)
	require.Nil(t, u)
	require.Equal(t, aiOffMessage, why)
}

func TestRecordTypeSafeUse_BillsWithoutARun(t *testing.T) {
	e := newUseEnv(t, true, nil)
	u, _ := e.h.TypeSafeFor(TypeSafeUseImportStructure)
	require.NotNil(t, u)
	e.h.recordTypeSafeUse(models.AnalysisCostKindImport, u, &typesafe.Response{Model: "jev-1.13.0", Usage: typesafe.Usage{InputTokens: 2000}})
	var rows []models.AIAnalysisCostEvent
	require.NoError(t, e.s.DB().Find(&rows).Error)
	require.Len(t, rows, 1)
	require.Equal(t, models.AnalysisCostKindImport, rows[0].Kind)
	require.Equal(t, "", rows[0].RunID)
	require.InDelta(t, 2000*models.TypeSafeDefaultPricePerMTok/1e6, *rows[0].EstimatedCost, 1e-12)
}
