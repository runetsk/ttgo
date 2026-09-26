package ai_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTypeSafeSettings_PricePerMTokRoundTrip(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	read := func(body []byte) map[string]any {
		var got map[string]any
		require.NoError(t, json.Unmarshal(body, &got))
		return got
	}

	rr := doRequest(env, "GET", "/api/settings/typesafe", nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.InDelta(t, 0.042, read(rr.Body.Bytes())["price_per_mtok"], 1e-12)

	rr = doRequest(env, "PUT", "/api/settings/typesafe", map[string]any{"price_per_mtok": -0.01})
	require.Equal(t, http.StatusUnprocessableEntity, rr.Code, rr.Body.String())
	require.Equal(t, "price_per_mtok must be >= 0", read(rr.Body.Bytes())["error"])

	rr = doRequest(env, "PUT", "/api/settings/typesafe", map[string]any{"price_per_mtok": 0.1})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.InDelta(t, 0.1, read(rr.Body.Bytes())["price_per_mtok"], 1e-12)
}
