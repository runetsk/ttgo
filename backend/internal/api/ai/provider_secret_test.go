package ai_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/secretbox"

	"github.com/stretchr/testify/require"
)

// plantForeignProviderKey overwrites a provider's stored key with a ciphertext made under
// another key — what a lost secret.key or a DB restored from another instance leaves behind.
func plantForeignProviderKey(t *testing.T, env *testEnv, id string) string {
	t.Helper()
	box, err := secretbox.New([]byte("0123456789abcdef0123456789abcdef"))
	require.NoError(t, err)
	foreign, err := box.Encrypt("sk-lost-9999")
	require.NoError(t, err)
	require.NoError(t, env.store.DB().Exec("UPDATE llm_provider_configs SET api_key = ? WHERE id = ?", foreign, id).Error)
	return foreign
}

func requireNoProviderCall(t *testing.T, c *fakeLLMCapture) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	require.Empty(t, c.path, "no request may reach the provider without its key")
}

func TestProviderKey_UndecryptableRecovery(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	id := createFakeProvider(t, env, fake.URL)
	foreign := plantForeignProviderKey(t, env, id)

	// The list still loads and reports the key's status, never the ciphertext.
	rr := doRequest(env, "GET", "/api/settings/llm-providers", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.NotContains(t, rr.Body.String(), foreign)
	var list []map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &list))
	require.Len(t, list, 1)
	require.Equal(t, "undecryptable", list[0]["api_key_status"])
	require.Equal(t, "", list[0]["api_key_masked"])

	// The connection test says what to do, with 422, and sends nothing.
	rr = doRequest(env, "POST", "/api/settings/llm-providers/"+id+"/test", nil)
	require.Equal(t, http.StatusUnprocessableEntity, rr.Code, rr.Body.String())
	require.Contains(t, rr.Body.String(), "the stored key can't be decrypted — re-enter it")
	require.Contains(t, rr.Body.String(), `"category":"configuration"`)
	requireNoProviderCall(t, &captured)

	// Non-secret edits work and keep the ciphertext.
	update := map[string]interface{}{
		"label": "Fake Local LLM", "provider_type": "local", "endpoint_url": fake.URL,
		"model_name": "fake-model-2", "enabled": false,
	}
	rr = doRequest(env, "PUT", "/api/settings/llm-providers/"+id, update)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Contains(t, rr.Body.String(), `"api_key_status":"undecryptable"`)
	require.Contains(t, rr.Body.String(), `"model_name":"fake-model-2"`)
	var stored string
	require.NoError(t, env.store.DB().Raw("SELECT api_key FROM llm_provider_configs WHERE id = ?", id).Scan(&stored).Error)
	require.Equal(t, foreign, stored)

	// A new key together with clear_api_key is refused; clear alone removes it.
	update["api_key"], update["clear_api_key"] = "sk-x", true
	rr = doRequest(env, "PUT", "/api/settings/llm-providers/"+id, update)
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	delete(update, "api_key")
	rr = doRequest(env, "PUT", "/api/settings/llm-providers/"+id, update)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Contains(t, rr.Body.String(), `"api_key_status":"missing"`)

	// Re-entering a key makes it usable again.
	delete(update, "clear_api_key")
	update["api_key"] = "sk-new-4321"
	rr = doRequest(env, "PUT", "/api/settings/llm-providers/"+id, update)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Contains(t, rr.Body.String(), `"api_key_status":"ok"`)
	require.Contains(t, rr.Body.String(), `"api_key_masked":"****4321"`)

	rr = doRequest(env, "DELETE", "/api/settings/llm-providers/"+id, nil)
	require.Equal(t, http.StatusNoContent, rr.Code, rr.Body.String())
}

func TestCreateGeneration_UndecryptableKeyFailsWithConfiguration(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeEnvelopeJSON)
	defer fake.Close()
	id := createFakeProvider(t, env, fake.URL)
	plantForeignProviderKey(t, env, id)
	reqID := createPreviewRequirement(t, env, "REQ-KEY-1", "Login flow", "Users must be able to log in.")

	body := map[string]string{"requirement_id": reqID, "provider_id": id, "coverage_level": "essential", "idempotency_key": "undecryptable-1"}
	rr := doRequest(env, "POST", "/api/ai-generations", body)
	require.Equal(t, http.StatusUnprocessableEntity, rr.Code, rr.Body.String())
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Equal(t, "configuration", got["category"])
	require.Contains(t, got["error"], "the stored key can't be decrypted — re-enter it")
	require.NotEmpty(t, got["run_id"])
	requireNoProviderCall(t, &captured)

	// The run is stored as failed with that category, and replaying the key reproduces the 422.
	run, err := env.store.GetGenerationRunByKey("undecryptable-1")
	require.NoError(t, err)
	require.Equal(t, models.AIGenerationRunStatusFailed, run.Status)
	require.Equal(t, "configuration", run.ErrorCategory)
	rr = doRequest(env, "POST", "/api/ai-generations", body)
	require.Equal(t, http.StatusUnprocessableEntity, rr.Code, rr.Body.String())
}

func TestParseImport_UndecryptableKeySaysConfiguration(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	var captured fakeLLMCapture
	fake := newFakeLLMServer(t, &captured, fakeDraftsJSON)
	defer fake.Close()
	id := createFakeProvider(t, env, fake.URL)
	plantForeignProviderKey(t, env, id)

	rr := doRequest(env, "POST", "/api/import/parse", models.ParseImportRequest{Content: "random gibberish with no structure at all here"})
	require.Equal(t, http.StatusUnprocessableEntity, rr.Code, rr.Body.String())
	var got map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &got))
	require.Equal(t, "configuration", got["category"])
	require.Contains(t, got["error"], "the stored key can't be decrypted — re-enter it")
	requireNoProviderCall(t, &captured)
}
