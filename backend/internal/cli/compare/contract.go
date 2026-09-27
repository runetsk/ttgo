package compare

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Explanation contract (spec B5 #28). The narrator is asked for one summary sentence, one next
// step and a rationale citing at most two pieces of evidence, quoted briefly
// (failureanalysis.narrativeSystemMessage). These checks are heuristics over the stored text,
// not a parser: an abbreviation such as "e.g. " reads as a sentence break, and "check X and
// re-run" reads as one action.
var (
	sentenceEndRe = regexp.MustCompile(`[.!?]+(?:\s+|$)`)
	listItemRe    = regexp.MustCompile(`(?m)^\s*(?:[-*•]|\d+[.)])\s+\S`)
	quotedRe      = regexp.MustCompile("\"[^\"\\n]+\"|“[^”\\n]+”|`[^`\\n]+`|(?:^|\\s)'[^'\\n]+'")
)

// sentences counts runs of text ended by . ! or ? followed by whitespace or the end, plus a
// trailing run with no terminator. "0.5" is not a break.
func sentences(s string) int {
	n := 0
	for _, part := range sentenceEndRe.Split(strings.TrimSpace(s), -1) {
		if strings.TrimSpace(part) != "" {
			n++
		}
	}
	return n
}

// nextActions counts list items when the text is a list, sentences otherwise.
func nextActions(s string) int {
	if items := listItemRe.FindAllString(s, -1); len(items) > 0 {
		return len(items)
	}
	return sentences(s)
}

// evidenceItems counts quoted spans; an apostrophe inside a word does not open a quote.
func evidenceItems(s string) int {
	return len(quotedRe.FindAllString(s, -1))
}

// ContractStats is one column's explanations against the contract.
type ContractStats struct {
	Key           string `json:"key"`
	Explained     int    `json:"explained"` // representative cells with an explanation
	OneSummary    int    `json:"one_sentence_summary"`
	OneNextAction int    `json:"one_next_action"`
	EvidenceMax2  int    `json:"evidence_at_most_two"`
	AllThree      int    `json:"all_three"`
	LengthP50     int    `json:"length_p50"` // characters of summary + next action + rationale
	LengthP90     int    `json:"length_p90"`
	LengthMax     int    `json:"length_max"`
}

// explained reports whether a cell holds an explanation of its own: not a failed attempt, not a
// clone (which repeats its representative's text) and not a decision stored without one.
func explained(c *Cell) bool {
	if c == nil || c.Failed || c.Clone {
		return false
	}
	if c.NarrativeStatus != "" && c.NarrativeStatus != "ok" {
		return false
	}
	return strings.TrimSpace(c.Summary+c.NextAction+c.Rationale) != ""
}

// CheckExplanations checks every column's explanations against the contract.
func CheckExplanations(rep Report) []ContractStats {
	out := make([]ContractStats, 0, len(rep.Columns))
	for _, col := range rep.Columns {
		st := ContractStats{Key: col.Key}
		var lengths []int
		for _, row := range rep.Rows {
			c := row.Cells[col.Key]
			if !explained(c) {
				continue
			}
			st.Explained++
			one := sentences(c.Summary) == 1
			act := nextActions(c.NextAction) == 1
			ev := evidenceItems(c.Rationale) <= 2
			if one {
				st.OneSummary++
			}
			if act {
				st.OneNextAction++
			}
			if ev {
				st.EvidenceMax2++
			}
			if one && act && ev {
				st.AllThree++
			}
			lengths = append(lengths, utf8.RuneCountInString(c.Summary)+utf8.RuneCountInString(c.NextAction)+utf8.RuneCountInString(c.Rationale))
		}
		sort.Ints(lengths)
		st.LengthP50, st.LengthP90 = percentile(lengths, 0.5), percentile(lengths, 0.9)
		if len(lengths) > 0 {
			st.LengthMax = lengths[len(lengths)-1]
		}
		out = append(out, st)
	}
	return out
}

// percentile is the nearest-rank percentile of a sorted slice (0 for an empty one).
func percentile(sorted []int, p float64) int {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Ceil(p*float64(len(sorted)))) - 1
	if i < 0 {
		i = 0
	}
	return sorted[i]
}

// RenderContract writes the contract check under the compare report.
func RenderContract(w io.Writer, stats []ContractStats) {
	fmt.Fprintln(w, "Explanation contract: one summary sentence, exactly one next action, at most two cited evidence items (representatives only; clones repeat their representative's text)")
	for _, st := range stats {
		if st.Explained == 0 {
			fmt.Fprintf(w, "%s: no explanations\n", st.Key)
			continue
		}
		fmt.Fprintf(w, "%s: %d explanations — one-sentence summary %s, one next action %s, evidence ≤2 %s, all three %s; length p50 %d · p90 %d · max %d chars\n",
			st.Key, st.Explained, pct(st.OneSummary, st.Explained), pct(st.OneNextAction, st.Explained),
			pct(st.EvidenceMax2, st.Explained), pct(st.AllThree, st.Explained), st.LengthP50, st.LengthP90, st.LengthMax)
	}
}
