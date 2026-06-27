package failureanalysis

import (
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"time"
	"ttgo/pkg/tracker/models"
)

var (
	reNumericID   = regexp.MustCompile(`\b\d{4,}\b`)
	reHexAddr     = regexp.MustCompile(`0x[0-9a-fA-F]+`)
	reTimestamp   = regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?`)
	rePathNonBase = regexp.MustCompile(`(?:[A-Za-z]:)?(?:/[^/\s:]+)+/([^/\s:]+)`)
)

// normalize strips IDs/timestamps/hex/file paths from an error message so
// that cosmetically-different but structurally-identical errors produce the
// same Signature.
func normalize(s string) string {
	s = reTimestamp.ReplaceAllString(s, "<T>")
	s = reHexAddr.ReplaceAllString(s, "<H>")
	s = reNumericID.ReplaceAllString(s, "<N>")
	s = rePathNonBase.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

// Signature is a deterministic hash of (failure_type, normalized error_message).
// Used to group structurally-identical failures so the worker analyzes one
// representative per group and clones the verdict to siblings.
func Signature(failureType, errorMessage string) string {
	h := sha1.Sum([]byte(failureType + "||" + normalize(errorMessage)))
	return hex.EncodeToString(h[:])
}

// FailureGroup is one dedup cluster: a representative result plus any siblings
// that share its Signature.
type FailureGroup struct {
	Key            string
	Representative *models.RunResult
	Members        []*models.RunResult
}

// GroupFailures clusters results by Signature. Representative per group is the
// result with the oldest StartTime (stable tiebreaker on ID). Returns groups
// sorted by size descending — callers process the noisiest clusters first so
// a cap covers the most failures.
func GroupFailures(results []*models.RunResult) []*FailureGroup {
	byKey := make(map[string]*FailureGroup)
	for _, r := range results {
		key := Signature(r.FailureType, r.ErrorMessage)
		g, ok := byKey[key]
		if !ok {
			g = &FailureGroup{Key: key}
			byKey[key] = g
		}
		g.Members = append(g.Members, r)
	}

	out := make([]*FailureGroup, 0, len(byKey))
	for _, g := range byKey {
		sort.SliceStable(g.Members, func(i, j int) bool {
			return earlier(g.Members[i], g.Members[j])
		})
		g.Representative = g.Members[0]
		out = append(out, g)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].Members) != len(out[j].Members) {
			return len(out[i].Members) > len(out[j].Members)
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func earlier(a, b *models.RunResult) bool {
	at := startTime(a)
	bt := startTime(b)
	if !at.Equal(bt) {
		return at.Before(bt)
	}
	return a.ID < b.ID
}

func startTime(r *models.RunResult) time.Time {
	if !r.StartTime.IsZero() {
		return r.StartTime
	}
	return r.CreatedAt
}
