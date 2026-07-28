package defects

import (
	"fmt"
	"net/url"
	"strings"

	"ttgo/pkg/tracker/store"
)

var validSeverity = map[string]bool{"critical": true, "major": true, "minor": true, "trivial": true}

// validStatus — "fixed" means fixed but awaiting retest; "open" stays valid for
// backward compatibility with the CLI, seed data and external scripts.
var validStatus = map[string]bool{"open": true, "fixed": true, "closed": true}

// ValidExternalURL accepts "" or an absolute http/https URL (length-capped). XSS guard.
func ValidExternalURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if len(raw) > 2048 {
		return fmt.Errorf("external_url too long")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("external_url is not a valid URL")
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("external_url must be an absolute http or https URL")
	}
	return nil
}

// ValidateAssignee returns a validation message when assigneeID names a user that cannot
// own a defect, or "" when the value is acceptable. nil and "" both mean "unassigned" and
// are always accepted. Mirrors the run-assignee rule in runs.AssignRun: a non-empty id must
// reference an existing, active, non-deleted user.
//
// The lookup error is returned SEPARATELY from the validation message on purpose: folding
// the two together (as runs.AssignRun still does) reports a transient database failure to
// the client as a 400 "assignee_id must reference an active user", which tells the caller
// their input was wrong when it was not, and hides a server fault from every caller that
// only logs 5xx.
//
// Exported and store-backed on purpose: ValidateCreate calls it, so the create-and-link path
// in the runs package (CreateAndLinkResultDefect) inherits the same check for free.
func ValidateAssignee(s *store.Store, assigneeID *string) (string, error) {
	if normalizeAssignee(assigneeID) == nil {
		return "", nil
	}
	u, err := s.GetUser(*assigneeID)
	if err != nil {
		return "", err
	}
	if u == nil || !u.Active || u.Deleted {
		return "assignee_id must reference an active user", nil
	}
	return "", nil
}

// normalizeAssignee collapses the two "unassigned" spellings — nil and "" — to nil.
// Used on create, where both mean the same thing. Update deliberately does not normalize:
// there "" clears an existing assignee while nil leaves it unchanged, so the store needs
// to see the difference.
func normalizeAssignee(assigneeID *string) *string {
	if assigneeID == nil || *assigneeID == "" {
		return nil
	}
	return assigneeID
}
