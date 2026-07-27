package defects

import (
	"testing"

	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return s
}

// mkUser inserts a user and forces the active/deleted flags CreateUser does not expose.
func mkUser(t *testing.T, s *store.Store, email string, active, deleted bool) *models.User {
	t.Helper()
	u, err := s.CreateUser(email, "Test User", "hash", "member")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := s.UpdateUser(u.ID, map[string]interface{}{"active": active, "deleted": deleted}); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	return u
}

func ptr(s string) *string { return &s }

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
	s := newTestStore(t)
	// "" means "use the store default"; "open" must stay valid for the CLI, seed data and external scripts.
	for _, st := range []string{"", "open", "fixed", "closed"} {
		if msg := ValidateCreate(s, models.CreateDefectRequest{Title: "boom", Status: st}); msg != "" {
			t.Errorf("status %q: expected accepted, got %q", st, msg)
		}
	}
	for _, st := range []string{"Fixed", "fixed ", "reopened", "in_progress", "triage", "bogus"} {
		if msg := ValidateCreate(s, models.CreateDefectRequest{Title: "boom", Status: st}); msg != "invalid status" {
			t.Errorf("status %q: expected %q, got %q", st, "invalid status", msg)
		}
	}
}

func TestValidateAssignee(t *testing.T) {
	s := newTestStore(t)
	active := mkUser(t, s, "active@example.com", true, false)
	inactive := mkUser(t, s, "inactive@example.com", false, false)
	deleted := mkUser(t, s, "deleted@example.com", true, true)

	// nil and "" both mean "unassigned" and are always accepted
	if msg := ValidateAssignee(s, nil); msg != "" {
		t.Errorf("nil assignee: expected accepted, got %q", msg)
	}
	if msg := ValidateAssignee(s, ptr("")); msg != "" {
		t.Errorf("empty assignee: expected accepted, got %q", msg)
	}
	if msg := ValidateAssignee(s, &active.ID); msg != "" {
		t.Errorf("active user: expected accepted, got %q", msg)
	}

	const want = "assignee_id must reference an active user"
	for name, id := range map[string]string{
		"unknown":  "no-such-user",
		"inactive": inactive.ID,
		"deleted":  deleted.ID,
	} {
		if msg := ValidateAssignee(s, &id); msg != want {
			t.Errorf("%s user: expected %q, got %q", name, want, msg)
		}
	}
}

// ValidateCreate must carry the assignee check so CreateAndLinkResultDefect inherits it.
func TestValidateCreateAssignee(t *testing.T) {
	s := newTestStore(t)
	active := mkUser(t, s, "creator@example.com", true, false)

	if msg := ValidateCreate(s, models.CreateDefectRequest{Title: "boom", AssigneeID: &active.ID}); msg != "" {
		t.Errorf("active assignee: expected accepted, got %q", msg)
	}
	if msg := ValidateCreate(s, models.CreateDefectRequest{Title: "boom", AssigneeID: ptr("ghost")}); msg == "" {
		t.Error("unknown assignee: expected rejection, got accepted")
	}
	// title validation still wins over the (more expensive) assignee lookup
	if msg := ValidateCreate(s, models.CreateDefectRequest{Title: "  ", AssigneeID: ptr("ghost")}); msg != "title is required" {
		t.Errorf("expected %q, got %q", "title is required", msg)
	}
}

func TestDefectFromCreateAssignee(t *testing.T) {
	if d := DefectFromCreate(models.CreateDefectRequest{Title: "boom", AssigneeID: ptr("u1")}); d.AssigneeID == nil || *d.AssigneeID != "u1" {
		t.Errorf("expected assignee u1, got %v", d.AssigneeID)
	}
	// on create, "" and nil are the same thing: unassigned (a NULL column, never an empty string)
	for _, req := range []models.CreateDefectRequest{
		{Title: "boom", AssigneeID: ptr("")},
		{Title: "boom"},
	} {
		if d := DefectFromCreate(req); d.AssigneeID != nil {
			t.Errorf("expected nil assignee, got %q", *d.AssigneeID)
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
