package ai_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// The auto-on-completion hook gates on the per-provider
// allow_auto_failure_analysis opt-in, so the flag must be settable through the
// provider API — it once existed only on the model, leaving the gate
// permanently closed on real installs (no handler or UI ever wrote it).
func TestProviderAllowAutoFailureAnalysisRoundtrip(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	create := map[string]interface{}{
		"label":                       "auto-fa",
		"provider_type":               "openai",
		"endpoint_url":                "https://api.openai.com",
		"api_key":                     "sk-test",
		"model_name":                  "gpt-test",
		"is_default":                  true,
		"allow_auto_failure_analysis": true,
	}
	rr := doRequest(env, "POST", "/api/settings/llm-providers", create)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var got struct {
		ID        string `json:"id"`
		AllowAuto bool   `json:"allow_auto_failure_analysis"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.True(t, got.AllowAuto, "create must persist the opt-in")

	// Omitting the field on update leaves it unchanged (pointer semantics).
	update := map[string]interface{}{
		"label": "auto-fa", "provider_type": "openai",
		"endpoint_url": "https://api.openai.com", "model_name": "gpt-test",
		"is_default": true,
	}
	rr = doRequest(env, "PUT", "/api/settings/llm-providers/"+got.ID, update)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.True(t, got.AllowAuto, "omitted field must not reset the opt-in")

	// Explicit false revokes it.
	update["allow_auto_failure_analysis"] = false
	rr = doRequest(env, "PUT", "/api/settings/llm-providers/"+got.ID, update)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.False(t, got.AllowAuto, "explicit false must revoke the opt-in")
}
