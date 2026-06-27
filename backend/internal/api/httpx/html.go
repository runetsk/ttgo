package httpx

import (
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// NormalizeEmptyHTML sanitizes s and returns an empty string if no visible text remains.
func NormalizeEmptyHTML(p *bluemonday.Policy, s string) string {
	sanitized := p.Sanitize(s)
	plain := bluemonday.StrictPolicy().Sanitize(sanitized)
	if strings.TrimSpace(plain) == "" {
		return ""
	}
	return sanitized
}
