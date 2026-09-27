package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// runCLI runs the root command against srvURL with a throwaway home (no real ~/.ttgo config).
func runCLI(t *testing.T, srvURL, stdin string, args ...string) (string, error) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("TTGO_SERVER_URL", "")
	t.Setenv("TTGO_API_TOKEN", "")
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetIn(strings.NewReader(stdin))
	root.SetArgs(append([]string{"--server", srvURL, "--token", "tok", "-o", "json"}, args...))
	err := root.Execute()
	return out.String(), err
}

// recorder answers every request with body and remembers the last one.
type recorder struct {
	method, path string
	body         map[string]interface{}
}

func (rec *recorder) server(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method, rec.path, rec.body = r.Method, r.URL.Path, nil
		_ = json.NewDecoder(r.Body).Decode(&rec.body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestTypeSafePatch_SendsOnlyTheFlagsThatWereSet(t *testing.T) {
	cmd := newAITypeSafeSetCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--enabled=false", "--price-per-mtok", "0.05", "--escalate-below-pct", "90", "--model", "jev-latest"}))
	patch, err := typeSafePatch(cmd, strings.NewReader(""))
	require.NoError(t, err)
	require.Equal(t, map[string]interface{}{"enabled": false, "price_per_mtok": 0.05, "escalate_below_pct": 90, "model": "jev-latest"}, patch)
}

func TestTypeSafePatch_RejectsNothingAndConflicts(t *testing.T) {
	cmd := newAITypeSafeSetCmd()
	require.NoError(t, cmd.ParseFlags(nil))
	_, err := typeSafePatch(cmd, strings.NewReader(""))
	require.ErrorContains(t, err, "nothing to change")

	cmd = newAITypeSafeSetCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--api-key-stdin", "--clear-api-key"}))
	_, err = typeSafePatch(cmd, strings.NewReader("sk-x"))
	require.ErrorContains(t, err, "cannot be combined")

	cmd = newAITypeSafeSetCmd()
	require.NoError(t, cmd.ParseFlags([]string{"--api-key-stdin"}))
	_, err = typeSafePatch(cmd, strings.NewReader("  \n"))
	require.ErrorContains(t, err, "stdin was empty")
}

func TestAITypeSafeSet_PutsThePatchWithTheKeyFromStdin(t *testing.T) {
	rec := &recorder{}
	srv := rec.server(t, http.StatusOK, `{"enabled":true,"api_key_status":"ok","price_per_mtok":0.05}`)
	out, err := runCLI(t, srv.URL, "sk-test-123\n", "ai", "typesafe", "set", "--api-key-stdin", "--price-per-mtok", "0.05")
	require.NoError(t, err, out)
	require.Equal(t, http.MethodPut, rec.method)
	require.Equal(t, "/api/settings/typesafe", rec.path)
	require.Equal(t, map[string]interface{}{"api_key": "sk-test-123", "price_per_mtok": 0.05}, rec.body)
	require.NotContains(t, out, "sk-test-123", "the key is never echoed")
	require.Contains(t, out, `"api_key_status": "ok"`)
}

func TestAITypeSafeTest_FailsWhenTheConnectionTestFails(t *testing.T) {
	rec := &recorder{}
	srv := rec.server(t, http.StatusOK, `{"ok":false,"category":"auth","message":"invalid API key"}`)
	out, err := runCLI(t, srv.URL, "", "ai", "typesafe", "test")
	require.ErrorContains(t, err, "auth: invalid API key")
	require.Equal(t, "/api/settings/typesafe/test", rec.path)
	require.Contains(t, out, `"ok": false`)
}

func TestAIFeaturesSet_RequiresEnabledAndSendsIt(t *testing.T) {
	_, err := runCLI(t, "http://127.0.0.1:1", "", "ai", "features", "set")
	require.ErrorContains(t, err, "--enabled")

	rec := &recorder{}
	srv := rec.server(t, http.StatusOK, `{"enabled":false}`)
	_, err = runCLI(t, srv.URL, "", "ai", "features", "set", "--enabled=false")
	require.NoError(t, err)
	require.Equal(t, http.MethodPut, rec.method)
	require.Equal(t, "/api/settings/ai-features", rec.path)
	require.Equal(t, map[string]interface{}{"enabled": false}, rec.body)
}

func TestAdminWritesExplainTheTokenRefusal(t *testing.T) {
	rec := &recorder{}
	srv := rec.server(t, http.StatusForbidden, `{"error":"admin session required"}`)
	_, err := runCLI(t, srv.URL, "", "ai", "features", "set", "--enabled=true")
	require.ErrorContains(t, err, "admin session required")
	require.ErrorContains(t, err, "signed in to the web UI")
}
