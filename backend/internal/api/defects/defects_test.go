package defects

import (
	"testing"

	"ttgo/pkg/tracker/models"
)

func TestValidExternalURL(t *testing.T) {
	ok := []string{"", "https://x.atlassian.net/browse/PROJ-1", "http://gh.example/issues/3"}
	bad := []string{"javascript:alert(1)", "/relative/path", "ftp://h/x", "notaurl"}
	for _, u := range ok {
		if err := ValidExternalURL(u); err != nil {
			t.Errorf("expected %q valid, got %v", u, err)
		}
	}
	for _, u := range bad {
		if err := ValidExternalURL(u); err == nil {
			t.Errorf("expected %q invalid", u)
		}
	}
}

func TestValidateCreateStatus(t *testing.T) {
	// "" means "use the store default"; "open" must stay valid for the CLI, seed data and external scripts.
	for _, st := range []string{"", "open", "fixed", "closed"} {
		if msg := ValidateCreate(models.CreateDefectRequest{Title: "boom", Status: st}); msg != "" {
			t.Errorf("status %q: expected accepted, got %q", st, msg)
		}
	}
	for _, st := range []string{"Fixed", "fixed ", "reopened", "in_progress", "triage", "bogus"} {
		if msg := ValidateCreate(models.CreateDefectRequest{Title: "boom", Status: st}); msg != "invalid status" {
			t.Errorf("status %q: expected %q, got %q", st, "invalid status", msg)
		}
	}
}

func TestValidateUpdateStatus(t *testing.T) {
	for _, st := range []string{"open", "fixed", "closed"} {
		if msg := validateUpdate(models.UpdateDefectRequest{Status: &st}); msg != "" {
			t.Errorf("status %q: expected accepted, got %q", st, msg)
		}
	}
	for _, st := range []string{"", "Fixed", "reopened", "bogus"} {
		if msg := validateUpdate(models.UpdateDefectRequest{Status: &st}); msg != "invalid status" {
			t.Errorf("status %q: expected %q, got %q", st, "invalid status", msg)
		}
	}
	if msg := validateUpdate(models.UpdateDefectRequest{}); msg != "" {
		t.Errorf("nil status: expected accepted, got %q", msg)
	}
}
