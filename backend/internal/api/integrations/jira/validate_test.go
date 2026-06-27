package jira

import "testing"

// TestValidJiraKey covers the key validation that blocks path-traversal and JQL
// injection through caller-supplied source keys (F-027/F-069).
func TestValidJiraKey(t *testing.T) {
	good := []string{"PROJ-1", "ABC123-9999", "AB-1"}
	for _, k := range good {
		if !ValidJiraKey(k) {
			t.Errorf("ValidJiraKey(%q) = false, want true", k)
		}
	}
	bad := []string{
		"", "proj-1", "PROJ", "PROJ-", "-1", "A-1", // shape
		"PROJ-1 OR 1=1",                     // JQL injection
		"../../../rest/api/3/myself",        // path traversal
		"PROJ-1/comment", "PROJ-1?expand=x", // path/query injection
	}
	for _, k := range bad {
		if ValidJiraKey(k) {
			t.Errorf("ValidJiraKey(%q) = true, want false", k)
		}
	}
}
