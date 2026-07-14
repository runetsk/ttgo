package aigen

import (
	"fmt"
	"regexp"
	"strings"

	"ttgo/pkg/tracker/models"
)

// Duplicate-candidate kinds and thresholds (spec: "Duplicate detection").
const (
	DupKindBatch    = "batch"
	DupKindExisting = "existing"

	// DupSimilarityThreshold is the floor below which candidates are noise.
	DupSimilarityThreshold = 0.55
	// DupHighConfidence marks candidates Stage 4's "Accept all clean drafts"
	// treats as unresolved duplicates.
	DupHighConfidence = 0.9
	// MaxDuplicateCandidates caps the stored per-draft candidate list.
	MaxDuplicateCandidates = 5
)

// DuplicateCandidate is one possible duplicate of a draft, either another
// draft in the batch or an existing test case.
type DuplicateCandidate struct {
	Kind          string  `json:"kind"`
	DraftPosition *int    `json:"draft_position,omitempty"`
	TestCaseID    string  `json:"test_case_id,omitempty"`
	FolderID      string  `json:"folder_id,omitempty"`
	Name          string  `json:"name"`
	Similarity    float64 `json:"similarity"`
	Reason        string  `json:"reason"`
}

// BatchDraft pairs a draft with its persistent Position so batch comparisons
// survive Stage 5 alternatives (where slice index != Position).
type BatchDraft struct {
	Position int
	Draft    models.GeneratedTestCase
}

var (
	bracketPrefixRe = regexp.MustCompile(`^\s*(\[[^\]]*\]\s*)+`)
	nonWordRe       = regexp.MustCompile(`[^a-z0-9]+`)
)

// NormalizeTestText canonicalizes a test name for comparison: lowercase,
// leading [bracketed] tags dropped, punctuation collapsed to single spaces.
func NormalizeTestText(s string) string {
	s = bracketPrefixRe.ReplaceAllString(s, "")
	s = strings.ToLower(s)
	s = nonWordRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

func tokenSet(s string) map[string]bool {
	set := map[string]bool{}
	for _, tok := range strings.Fields(NormalizeTestText(s)) {
		set[tok] = true
	}
	return set
}

// TokenSimilarity is the Jaccard index over normalized word sets — a cheap,
// order-insensitive, explainable lexical similarity in [0,1].
func TokenSimilarity(a, b string) float64 {
	sa, sb := tokenSet(a), tokenSet(b)
	if len(sa) == 0 || len(sb) == 0 {
		return 0
	}
	inter := 0
	for tok := range sa {
		if sb[tok] {
			inter++
		}
	}
	union := len(sa) + len(sb) - inter
	return float64(inter) / float64(union)
}

func similarityReason(sim float64) string {
	if sim >= 0.999 {
		return "exact name match (ignoring tags, case, and punctuation)"
	}
	return fmt.Sprintf("names share %d%% of their words", int(sim*100+0.5))
}

// FindBatchDuplicates compares every pair of drafts by name similarity and
// returns, per Position, the candidate list (descending similarity, capped).
func FindBatchDuplicates(batch []BatchDraft) map[int][]DuplicateCandidate {
	out := map[int][]DuplicateCandidate{}
	for i := range batch {
		for j := range batch {
			if i == j {
				continue
			}
			sim := TokenSimilarity(batch[i].Draft.Name, batch[j].Draft.Name)
			if sim < DupSimilarityThreshold {
				continue
			}
			pos := batch[j].Position
			out[batch[i].Position] = append(out[batch[i].Position], DuplicateCandidate{
				Kind: DupKindBatch, DraftPosition: &pos,
				Name: batch[j].Draft.Name, Similarity: sim, Reason: similarityReason(sim),
			})
		}
		if list := out[batch[i].Position]; len(list) > 0 {
			sortCandidates(list)
			if len(list) > MaxDuplicateCandidates {
				out[batch[i].Position] = list[:MaxDuplicateCandidates]
			}
		}
	}
	return out
}

// sortCandidates orders by similarity descending (stable enough for tests:
// ties keep insertion order).
func sortCandidates(list []DuplicateCandidate) {
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j].Similarity > list[j-1].Similarity; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
}
