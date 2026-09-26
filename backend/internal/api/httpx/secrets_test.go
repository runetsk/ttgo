package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func TestWriteSecretError(t *testing.T) {
	w := httptest.NewRecorder()
	require.False(t, WriteSecretError(w, nil))
	require.False(t, WriteSecretError(w, errors.New("something else")))
	require.Zero(t, w.Body.Len(), "nothing is written for other errors")

	w = httptest.NewRecorder()
	require.True(t, WriteSecretError(w, &models.SecretError{Subject: "Jira API token"}))
	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "Jira API token: the stored key can't be decrypted — re-enter it", body["error"])
	require.Equal(t, "configuration", body["category"])

	w = httptest.NewRecorder()
	require.True(t, WriteSecretError(w, fmt.Errorf("%w: %w", models.ErrSecretNotSaved, errors.New("secret encryption is unavailable"))))
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "the key could not be encrypted, so nothing was saved: secret encryption is unavailable", body["error"],
		"the admin sees why nothing was saved, not the generic 500 text")
}
