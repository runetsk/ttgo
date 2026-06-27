// Package failureanalysis implements AI-powered classification of failing
// test results. The package is self-contained; LLM calls go through the
// existing llm.Provider abstraction.
package failureanalysis

import "regexp"

var redactors = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	// Bearer tokens in any header-ish form.
	{regexp.MustCompile(`(?i)Bearer\s+[A-Za-z0-9\-_\.]{20,}`), "Bearer <REDACTED_TOKEN>"},
	// OpenAI-style keys.
	{regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`), "<REDACTED_TOKEN>"},
	// AWS access key IDs.
	{regexp.MustCompile(`AKIA[0-9A-Z]{16}`), "<REDACTED_TOKEN>"},
	// GitHub personal-access tokens.
	{regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`), "<REDACTED_TOKEN>"},
	// Basic-auth URL credentials.
	{regexp.MustCompile(`(https?://)[^:/\s]+:[^@/\s]+@`), "$1<REDACTED>@"},
	// Private key PEM blocks (multi-line; use (?s) for dot-matches-newline).
	{regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`), "<REDACTED_KEY_BLOCK>"},
	// JWTs (header.payload.signature).
	{regexp.MustCompile(`eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}`), "<REDACTED_JWT>"},
	// Generic key=value / key: value secrets — keep the key, redact the value.
	{regexp.MustCompile(`(?i)((?:password|passwd|pwd|secret|token|api[_\-]?key|access[_\-]?key)\s*[=:]\s*)["']?[^\s"',;<]{4,}`), "${1}<REDACTED>"},
	// Stripe / Slack / Google cloud-provider keys.
	{regexp.MustCompile(`(?i)(?:sk|rk|pk)_(?:live|test)_[0-9A-Za-z]{10,}`), "<REDACTED_TOKEN>"},
	{regexp.MustCompile(`xox[baprs]-[A-Za-z0-9\-]{10,}`), "<REDACTED_TOKEN>"},
	{regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`), "<REDACTED_TOKEN>"},
	// Email addresses (PII) — failure logs are shipped to a third-party LLM.
	{regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`), "<REDACTED_EMAIL>"},
}

// Redact returns the input with well-known secret patterns replaced by
// <REDACTED_*> placeholders. It is a pure function — safe for concurrent use.
func Redact(in string) string {
	if in == "" {
		return ""
	}
	out := in
	for _, r := range redactors {
		out = r.pattern.ReplaceAllString(out, r.replacement)
	}
	return out
}
