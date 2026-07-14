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
