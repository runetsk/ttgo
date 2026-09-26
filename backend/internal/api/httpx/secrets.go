package httpx

import (
	"errors"
	"log/slog"
	"net/http"
	"ttgo/pkg/tracker/models"
)

// WriteSecretError answers a stored-secret failure and reports whether err was one. A secret that
// can't be decrypted is 422 with the re-enter message and category "configuration": retrying
// won't help, an admin must enter it again. A secret that could not be encrypted is 500 with the
// reason rather than Error's generic text: nothing was saved and the admin needs to know why.
func WriteSecretError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, models.ErrSecretUndecryptable):
		JSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error(), "category": "configuration"})
		return true
	case errors.Is(err, models.ErrSecretNotSaved):
		slog.Error("secret not saved", "error", err)
		JSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return true
	}
	return false
}
