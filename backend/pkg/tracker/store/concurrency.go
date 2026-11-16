package store

import (
	"errors"
	"strings"
)

// ErrEditConflict is returned by the *Checked update methods when the caller's
// expected edit token no longer matches the current persisted content — another
// writer changed the record since the editor loaded it. The current persisted
// record is returned alongside so the caller can surface it to the editor.
var ErrEditConflict = errors.New("edit conflict: the record changed since it was loaded")

// ErrBusy is returned when the database stayed locked across a retried
// read/compare/write transaction. Callers surface it as HTTP 503.
var ErrBusy = errors.New("database busy")

// isSQLiteBusy reports whether err is a SQLite lock-contention error, matched on
// message text to avoid importing the driver here (consistent with
// isUniqueConstraintError).
func isSQLiteBusy(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "sqlite_busy")
}

// runWithBusyRetry runs fn, and if it fails with a SQLite-busy error restarts the
// whole operation once. A second busy failure is surfaced as ErrBusy so the
// caller never falls through to a successful unchecked write.
func runWithBusyRetry(fn func() error) error {
	err := fn()
	if err != nil && isSQLiteBusy(err) {
		err = fn()
		if err != nil && isSQLiteBusy(err) {
			return ErrBusy
		}
	}
	return err
}
