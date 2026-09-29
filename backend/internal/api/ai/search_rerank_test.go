package ai

import (
	"errors"
	"testing"

	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

func threeResults() []store.SearchResult {
	return []store.SearchResult{{ID: "a", Name: "Login page layout"}, {ID: "b", Name: "Sign in with password"}, {ID: "c", Name: "Login audit log"}}
}

func TestRerankSearch_OrdersByRelevance(t *testing.T) {
	p := map[string]float64{"case_0": 0.3, "case_1": 0.95, "case_2": 0.1}
	e := newUseEnv(t, true, answerAll(nil, func(id string) float64 { return p[id] }))
	on := true
	_, err := e.s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{SearchRerankEnabled: &on})
	require.NoError(t, err)

	got, ok := e.h.RerankSearch(t.Context(), "how do users log in", threeResults())
	require.True(t, ok)
	require.Equal(t, []string{"b", "a", "c"}, []string{got[0].ID, got[1].ID, got[2].ID})
	require.InDelta(t, 0.95, *got[0].Relevance, 1e-12)
	req := e.client.requests()[0]
	require.Equal(t, "how do users log in", req.State.(map[string]any)["query"])
	require.Len(t, req.Questions, 3)
}

func TestRerankSearch_OffOrFailingKeepsTheOrder(t *testing.T) {
	off := newUseEnv(t, true, func(typesafe.Request) (*typesafe.Response, error) {
		t.Fatal("re-ranking is off by default")
		return nil, nil
	})
	got, ok := off.h.RerankSearch(t.Context(), "login", threeResults())
	require.False(t, ok)
	require.Equal(t, threeResults(), got)

	failing := newUseEnv(t, true, func(typesafe.Request) (*typesafe.Response, error) { return nil, errors.New("down") })
	on := true
	_, err := failing.s.UpdateTypeSafeSettings(models.TypeSafeSettingsPatch{SearchRerankEnabled: &on})
	require.NoError(t, err)
	got, ok = failing.h.RerankSearch(t.Context(), "login", threeResults())
	require.False(t, ok)
	require.Equal(t, threeResults(), got)
}
