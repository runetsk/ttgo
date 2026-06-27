package failureanalysis

import (
	"strings"
	"testing"
)

func TestRedactTokens(t *testing.T) {
	cases := []struct {
		name           string
		in             string
		mustContain    []string
		mustNotContain []string
	}{
		{"bearer", "Auth: Bearer abcdefghijklmnopqrst12345", []string{"<REDACTED_TOKEN>"}, []string{"abcdefghijklmnopqrst12345"}},
		{"sk-", "key=sk-1234567890ABCDEFGHIJabcd", []string{"<REDACTED_TOKEN>"}, []string{"sk-1234567890ABCDEFGHIJabcd"}},
		{"aws", "id=AKIAIOSFODNN7EXAMPLE", []string{"<REDACTED_TOKEN>"}, []string{"AKIAIOSFODNN7EXAMPLE"}},
		{"ghp", "token=ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ", []string{"<REDACTED_TOKEN>"}, []string{"ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ"}},
		{"basic-auth url", "connect https://alice:p4ss@svc.local/api", []string{"<REDACTED>@"}, []string{"alice:p4ss"}},
		{"private key", "-----BEGIN RSA PRIVATE KEY-----\nblah\n-----END RSA PRIVATE KEY-----", []string{"<REDACTED_KEY_BLOCK>"}, []string{"blah"}},
		{"non-secret", "expected 401, got 500 at /api/users/42", []string{"expected 401"}, []string{"<REDACTED"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Redact(tc.in)
			for _, s := range tc.mustContain {
				if !strings.Contains(got, s) {
					t.Errorf("output %q should contain %q", got, s)
				}
			}
			for _, s := range tc.mustNotContain {
				if strings.Contains(got, s) {
					t.Errorf("output %q should NOT contain %q", got, s)
				}
			}
		})
	}
}

func TestRedactEmptyString(t *testing.T) {
	if got := Redact(""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
