package defects

import "testing"

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
