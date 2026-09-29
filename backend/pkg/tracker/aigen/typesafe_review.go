package aigen

import (
	"encoding/json"
	"fmt"
	"sort"

	"ttgo/pkg/tracker/failureanalysis"
)

// TypeSafe draft review (spec Wave 5 §2): TypeSafe.ai rates each draft on three aspects and
// judges its duplicate candidates. These helpers turn its answers into the draft's findings and
// duplicate list; the handler makes the call.

// ReviewDimensionKey is the quality dimension TypeSafe's findings go to.
const ReviewDimensionKey = "typesafe"

func init() { QualityDimensionLabels[ReviewDimensionKey] = "TypeSafe.ai review" }

// Rating is TypeSafe's answer to one rating question.
type Rating struct {
	Value      string // failureanalysis.DraftRating*
	Confidence float64
}

// reviewAspects are the rated aspects: question-id prefix → the words a finding uses.
var reviewAspects = []struct{ key, what string }{
	{"clarity", "the actions for clarity"},
	{"observable", "the expected results for observability"},
	{"specific", "the test data for specificity"},
}

// ReviewFindings turns a draft's ratings (keyed clarity / observable / specific) into findings: a
// weak or poor rating at failureanalysis.DraftReviewMinConf or more (a warning; the message says
// which). Aspects without an answer say nothing.
func ReviewFindings(ratings map[string]Rating) []Finding {
	var out []Finding
	for _, a := range reviewAspects {
		r, ok := ratings[a.key]
		if !ok || r.Confidence < failureanalysis.DraftReviewMinConf {
			continue
		}
		if r.Value != failureanalysis.DraftRatingPoor && r.Value != failureanalysis.DraftRatingWeak {
			continue
		}
		out = append(out, Finding{Field: "draft", Code: "ts_" + a.key, Severity: SeverityWarning,
			Message: fmt.Sprintf("TypeSafe.ai rates %s %s (%d%%)", a.what, r.Value, int(r.Confidence*100+0.5))})
	}
	return out
}

// WithReviewDimension returns qualityJSON with the TypeSafe review dimension replaced by findings
// (dropped when there are none).
func WithReviewDimension(qualityJSON string, findings []Finding) string {
	var dims []QualityDimension
	if qualityJSON != "" {
		_ = json.Unmarshal([]byte(qualityJSON), &dims)
	}
	kept := dims[:0]
	for _, d := range dims {
		if d.Key != ReviewDimensionKey {
			kept = append(kept, d)
		}
	}
	if len(findings) > 0 {
		kept = append(kept, QualityDimension{Key: ReviewDimensionKey, Label: QualityDimensionLabels[ReviewDimensionKey], Findings: findings})
	}
	b, _ := json.Marshal(kept)
	return string(b)
}

// DupKey identifies a duplicate candidate: an existing test case, or a draft of the same batch.
func DupKey(c DuplicateCandidate) string {
	if c.Kind == DupKindExisting {
		return "tc:" + c.TestCaseID
	}
	if c.DraftPosition != nil {
		return fmt.Sprintf("draft:%d", *c.DraftPosition)
	}
	return "name:" + c.Name
}

// AskCandidates picks the candidates TypeSafe is asked about: the shown ones and the wider
// search's, once each, highest name similarity first, at most failureanalysis.DraftDupPerDraft.
func AskCandidates(shown, wider []DuplicateCandidate) []DuplicateCandidate {
	seen := map[string]bool{}
	var all []DuplicateCandidate
	for _, c := range append(append([]DuplicateCandidate(nil), shown...), wider...) {
		if k := DupKey(c); !seen[k] {
			seen[k] = true
			all = append(all, c)
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Similarity > all[j].Similarity })
	if len(all) > failureanalysis.DraftDupPerDraft {
		all = all[:failureanalysis.DraftDupPerDraft]
	}
	return all
}

// MergeDuplicateVerdicts applies TypeSafe's answers (p by DupKey) to the shown candidates: an asked
// candidate at failureanalysis.DraftDupSameMin or more is shown as a duplicate with similarity p;
// below DraftDupDifferentMax it is removed; in between the name-similarity verdict stands (shown
// only if it already was). Candidates TypeSafe was not asked about are left as they were.
func MergeDuplicateVerdicts(shown, asked []DuplicateCandidate, p map[string]float64) []DuplicateCandidate {
	var out []DuplicateCandidate
	done := map[string]bool{}
	decide := func(c DuplicateCandidate, wasShown bool) {
		k := DupKey(c)
		if done[k] {
			return
		}
		done[k] = true
		v, ok := p[k]
		switch {
		case !ok:
			if wasShown {
				out = append(out, c)
			}
		case v >= failureanalysis.DraftDupSameMin:
			c.Similarity = v
			c.Reason = fmt.Sprintf("TypeSafe.ai: same behaviour (%d%%)", int(v*100+0.5))
			out = append(out, c)
		case v < failureanalysis.DraftDupDifferentMax:
		default:
			if wasShown {
				out = append(out, c)
			}
		}
	}
	for _, c := range shown {
		decide(c, true)
	}
	for _, c := range asked {
		decide(c, false)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Similarity > out[j].Similarity })
	if len(out) > MaxDuplicateCandidates {
		out = out[:MaxDuplicateCandidates]
	}
	return out
}
