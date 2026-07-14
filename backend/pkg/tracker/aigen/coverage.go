package aigen

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"ttgo/pkg/tracker/models"
)

// Coverage-target kinds (spec: "Coverage targets").
const (
	TargetKindAcceptanceCriterion = "acceptance_criterion"
	TargetKindChildRequirement    = "child_requirement"

	// MaxCoverageTargets bounds stored coverage payloads.
	MaxCoverageTargets = 50

	// OverRepresentedThreshold: a target referenced by this many drafts or
	// more is flagged over-represented.
	OverRepresentedThreshold = 4
)

// CoverageTarget is one stable thing a generated batch should cover.
type CoverageTarget struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

// StripHTML converts rich-text requirement descriptions to plain lines.
// List items become "- " bullets; block-level tag boundaries become newlines.
// It is intentionally simple and deterministic — not a full HTML parser.
func StripHTML(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	inTag := false
	tag := strings.Builder{}
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
			tag.Reset()
		case r == '>' && inTag:
			inTag = false
			name := strings.ToLower(strings.TrimSpace(tag.String()))
			name = strings.TrimPrefix(name, "/")
			if i := strings.IndexAny(name, " \t"); i >= 0 {
				name = name[:i]
			}
			switch name {
			case "li":
				if !strings.HasPrefix(strings.TrimSpace(tag.String()), "/") {
					b.WriteString("\n- ")
				}
			case "p", "div", "br", "br/", "tr", "ul", "ol",
				"h1", "h2", "h3", "h4", "h5", "h6":
				b.WriteString("\n")
			}
		case inTag:
			tag.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	return html.UnescapeString(b.String())
}

var (
	bulletRe     = regexp.MustCompile(`^\s*(?:[-*•]|\d+[.)])\s+(.+)$`)
	acHeadingRe  = regexp.MustCompile(`(?i)^\s*#{0,6}\s*(acceptance\s+criteria|acceptance|criteria)\b`)
	endHeadingRe = regexp.MustCompile(`(?i)^\s*#{0,6}\s*(description|notes?|background|context|out\s+of\s+scope|non-goals)\b`)
	explicitACRe = regexp.MustCompile(`(?i)^AC[-_ ]?(\d+)\s*[:.)\-]?\s*(.*)$`)
	issueKeyRe   = regexp.MustCompile(`^\[?([A-Z][A-Z0-9]{1,9}-\d+)\]?\s*[:.)\-]?\s*(.*)$`)
)

// ExtractCoverageTargets deterministically derives coverage targets:
// one per child requirement, then one per acceptance-criterion bullet in the
// description. When an "Acceptance Criteria" heading exists, only bullets in
// that section count; otherwise every bullet counts. A bullet that starts
// with an explicit AC-n or issue-key token keeps that token as its ID.
func ExtractCoverageTargets(description string, children []*models.Requirement) []CoverageTarget {
	var targets []CoverageTarget
	seen := map[string]bool{}
	add := func(id, kind, text string) {
		id = strings.ToUpper(strings.TrimSpace(id))
		if id == "" || seen[id] || len(targets) >= MaxCoverageTargets {
			return
		}
		seen[id] = true
		targets = append(targets, CoverageTarget{ID: id, Kind: kind, Text: strings.TrimSpace(text)})
	}

	for _, c := range children {
		if c != nil {
			add(c.Identifier, TargetKindChildRequirement, c.Title)
		}
	}

	lines := strings.Split(StripHTML(description), "\n")
	inACSection := false
	sawACHeading := false
	var bullets []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if acHeadingRe.MatchString(trimmed) && !bulletRe.MatchString(trimmed) {
			inACSection, sawACHeading = true, true
			bullets = nil // restart: only the AC section counts now
			continue
		}
		if sawACHeading && inACSection && endHeadingRe.MatchString(trimmed) && !bulletRe.MatchString(trimmed) {
			inACSection = false
			continue
		}
		if m := bulletRe.FindStringSubmatch(trimmed); m != nil {
			if !sawACHeading || inACSection {
				bullets = append(bullets, strings.TrimSpace(m[1]))
			}
		}
	}

	acSeq := 0
	for _, bl := range bullets {
		switch {
		case explicitACRe.MatchString(bl):
			m := explicitACRe.FindStringSubmatch(bl)
			text := strings.TrimSpace(m[2])
			if text == "" {
				text = bl
			}
			add("AC-"+m[1], TargetKindAcceptanceCriterion, text)
		case issueKeyRe.MatchString(bl):
			m := issueKeyRe.FindStringSubmatch(bl)
			text := strings.TrimSpace(m[2])
			if text == "" {
				text = bl
			}
			add(m[1], TargetKindChildRequirement, text)
		default:
			acSeq++
			add(fmt.Sprintf("AC-%d", acSeq), TargetKindAcceptanceCriterion, bl)
		}
	}
	return targets
}

// Target coverage statuses.
const (
	TargetStatusUncovered       = "uncovered"
	TargetStatusCovered         = "covered"
	TargetStatusOverRepresented = "over_represented"
)

// TargetCoverage is one target with the draft positions that reference it.
type TargetCoverage struct {
	CoverageTarget
	DraftPositions []int  `json:"draft_positions"`
	Status         string `json:"status"`
}

// CoverageReport is the run-level coverage summary persisted on
// AIGenerationRun.CoverageJSON and returned as "coverage" by the API.
type CoverageReport struct {
	Targets              []TargetCoverage `json:"targets"`
	CoveredCount         int              `json:"covered_count"`
	UncoveredCount       int              `json:"uncovered_count"`
	OverRepresentedCount int              `json:"over_represented_count"`
	BatchFindings        []Finding        `json:"batch_findings,omitempty"`
}

var refSepRe = regexp.MustCompile(`[\s_]+`)

// NormalizeSourceRef canonicalizes a draft source_ref for target matching:
// trimmed, uppercased, surrounding brackets stripped, "AC 1"/"AC_1" → "AC-1".
func NormalizeSourceRef(ref string) string {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimPrefix(ref, "[")
	ref = strings.TrimSuffix(ref, "]")
	ref = strings.ToUpper(strings.TrimSpace(ref))
	ref = refSepRe.ReplaceAllString(ref, "-")
	return ref
}

// BuildCoverageReport maps draft source_refs onto targets. The index of each
// draft in the slice is treated as its Position (create-time ordering).
func BuildCoverageReport(targets []CoverageTarget, drafts []models.GeneratedTestCase) CoverageReport {
	rep := CoverageReport{Targets: make([]TargetCoverage, 0, len(targets))}
	byID := make(map[string]*TargetCoverage, len(targets))
	for _, t := range targets {
		rep.Targets = append(rep.Targets, TargetCoverage{CoverageTarget: t, DraftPositions: []int{}, Status: TargetStatusUncovered})
		byID[t.ID] = &rep.Targets[len(rep.Targets)-1]
	}
	for pos, d := range drafts {
		for _, ref := range d.SourceRefs {
			if tc, ok := byID[NormalizeSourceRef(ref)]; ok {
				tc.DraftPositions = append(tc.DraftPositions, pos)
			}
		}
	}
	for i := range rep.Targets {
		tc := &rep.Targets[i]
		switch {
		case len(tc.DraftPositions) == 0:
			rep.UncoveredCount++
		case len(tc.DraftPositions) >= OverRepresentedThreshold:
			tc.Status = TargetStatusOverRepresented
			rep.CoveredCount++
			rep.OverRepresentedCount++
		default:
			tc.Status = TargetStatusCovered
			rep.CoveredCount++
		}
	}
	return rep
}
