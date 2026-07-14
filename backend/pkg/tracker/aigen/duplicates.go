package aigen

import "strings"

// NormalizeTestText canonicalizes names/descriptions for comparison.
// Task "duplicates" extends this; the stub keeps rubric compilable.
func NormalizeTestText(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
