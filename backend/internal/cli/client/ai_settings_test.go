package client

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAISettingsClient_PathsAndBodies(t *testing.T) {
	type seen struct{ method, path, body string }
	var got seen
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = seen{r.Method, r.URL.Path, strings.TrimSpace(string(b))}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "tok")

	for _, tc := range []struct {
		name string
		call func() error
		want seen
	}{
		{"typesafe get", func() error { _, err := c.GetTypeSafeSettings(); return err }, seen{"GET", "/api/settings/typesafe", ""}},
		{"typesafe set", func() error {
			_, err := c.UpdateTypeSafeSettings(map[string]interface{}{"narrative_enabled": false})
			return err
		}, seen{"PUT", "/api/settings/typesafe", `{"narrative_enabled":false}`}},
		{"typesafe test", func() error { _, err := c.TestTypeSafeConnection(); return err }, seen{"POST", "/api/settings/typesafe/test", ""}},
		{"features get", func() error { _, err := c.GetAIFeatureSettings(); return err }, seen{"GET", "/api/settings/ai-features", ""}},
		{"features set", func() error { _, err := c.SetAIFeatureSettings(true); return err }, seen{"PUT", "/api/settings/ai-features", `{"enabled":true}`}},
	} {
		if err := tc.call(); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}
