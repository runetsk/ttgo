package store

import "testing"

// TestCreateWebhookConfig_RejectsUnsafeURLs verifies the SSRF guard (F-001):
// non-HTTPS and internal/loopback/metadata destinations are refused at creation.
func TestCreateWebhookConfig_RejectsUnsafeURLs(t *testing.T) {
	s, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	bad := []string{
		"http://example.com/hook",   // not https
		"https://127.0.0.1/hook",    // loopback
		"https://169.254.169.254/x", // cloud metadata
		"https://10.0.0.1/hook",     // private
		"https://192.168.1.1/hook",  // private
		"https://[::1]/hook",        // loopback v6
	}
	for _, u := range bad {
		if _, err := s.CreateWebhookConfig(u, "desc", "run.completed"); err == nil {
			t.Errorf("CreateWebhookConfig(%q) accepted, want rejection", u)
		}
	}
	// A public HTTPS literal IP is accepted (no DNS needed).
	if _, err := s.CreateWebhookConfig("https://8.8.8.8/hook", "ok", "run.completed"); err != nil {
		t.Errorf("CreateWebhookConfig(public) rejected: %v", err)
	}
}
