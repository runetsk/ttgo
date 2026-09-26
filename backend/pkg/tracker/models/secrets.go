package models

import "errors"

// Stored-secret status reported beside every masked secret: api_token_status on the Jira and
// Confluence settings, api_key_status on each LLM provider (and, with the same values, on the
// TypeSafe settings).
const (
	SecretStatusMissing       = TypeSafeKeyStatusMissing
	SecretStatusOK            = TypeSafeKeyStatusOK
	SecretStatusUndecryptable = TypeSafeKeyStatusUndecryptable
)

// ErrSecretUndecryptable: a stored secret exists but cannot be decrypted — secret.key was lost or
// rotated, the database was restored or copied from another instance, or the value was never
// encrypted. Only re-entering the secret fixes it; retrying never does.
var ErrSecretUndecryptable = errors.New("the stored key can't be decrypted — re-enter it")

// ErrSecretNotSaved: a new secret could not be encrypted, so the save was refused instead of
// storing it in plaintext.
var ErrSecretNotSaved = errors.New("the key could not be encrypted, so nothing was saved")

// SecretError names the stored secret a call needed but could not decrypt.
// errors.Is(err, ErrSecretUndecryptable) holds for it.
type SecretError struct {
	Subject string // e.g. `Jira API token`, `LLM provider "OpenAI" API key`
}

func (e *SecretError) Error() string { return e.Subject + ": " + ErrSecretUndecryptable.Error() }

func (e *SecretError) Unwrap() error { return ErrSecretUndecryptable }

// secretStatus is the status to report: the one the store set when it read the row or, for a
// value built in memory, missing/ok from whether a plaintext is present.
func secretStatus(set, plain string) string {
	if set != "" {
		return set
	}
	if plain == "" {
		return SecretStatusMissing
	}
	return SecretStatusOK
}

// secretError is the call-path error for a secret in the given status: nil unless undecryptable.
func secretError(subject, status string) error {
	if status != SecretStatusUndecryptable {
		return nil
	}
	return &SecretError{Subject: subject}
}
