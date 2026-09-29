package importparser

import (
	"fmt"
	"regexp"
	"strings"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"unicode/utf8"
)

// ImportLines is the content split into the lines TypeSafe classifies (spec Wave 5 §1): the
// non-empty lines, trimmed and cut to failureanalysis.ImportLineChars, at most ImportMaxLines.
// truncated reports that lines were left out.
func ImportLines(content string) (lines []string, truncated bool) {
	for _, raw := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		l := strings.TrimSpace(raw)
		if l == "" {
			continue
		}
		if len(lines) == failureanalysis.ImportMaxLines {
			return lines, true
		}
		if utf8.RuneCountInString(l) > failureanalysis.ImportLineChars {
			l = string([]rune(l)[:failureanalysis.ImportLineChars])
		}
		lines = append(lines, l)
	}
	return lines, false
}

var (
	leadMarker = regexp.MustCompile(`(?i)^\s*(?:#{1,6}\s*|\*\*|(?:step|test\s*case|tc)\s*[-#]?\d*\s*[:.)-]\s*|\d+[.)]\s*|[-*•]\s+)`)
	trailBold  = regexp.MustCompile(`\*\*\s*$`)
	// The role's own label, which the assembly already expresses ("Preconditions: …", the
	// expected-result field).
	preLabel = regexp.MustCompile(`(?i)^(?:pre-?conditions?|setup|given)\s*[:\-–]\s*`)
	expLabel = regexp.MustCompile(`(?i)^(?:expected(?:\s+results?)?|then|result)\s*[:\-–]\s*`)
)

// stripMarkers removes a line's list, numbering and heading markers ("1.", "-", "Step 3:", "##").
func stripMarkers(s string) string {
	for i := 0; i < 6; i++ { // "1. **Step 2:** …" carries several
		next := strings.TrimSpace(leadMarker.ReplaceAllString(s, ""))
		if next == s {
			break
		}
		s = next
	}
	return strings.TrimSpace(trailBold.ReplaceAllString(s, ""))
}

// AssembleClassified builds test cases from lines and their roles (one per line, the
// failureanalysis.ImportRole* values), copying text only: no step is invented. A title starts a
// case; preconditions join its description; a step appends a step (split on "->"/"=>"/
// "Expected:"); an expected line fills the last step's expected result (or becomes a step with no
// action); noise is dropped. A case with no title is named "Imported test case N"; one with
// neither steps nor a description is dropped.
func AssembleClassified(lines, roles []string) []models.GeneratedTestCase {
	var out []models.GeneratedTestCase
	var cur *models.GeneratedTestCase
	var pre []string
	untitled := 0
	flush := func() {
		if cur == nil {
			return
		}
		if len(pre) > 0 {
			cur.Description = "Preconditions: " + strings.Join(pre, "; ")
		}
		if len(cur.Steps) > 0 || cur.Description != "" {
			out = append(out, *cur)
		}
		cur, pre = nil, nil
	}
	open := func(name string) {
		flush()
		if name == "" {
			untitled++
			name = fmt.Sprintf("Imported test case %d", untitled)
		}
		cur = &models.GeneratedTestCase{Name: name}
	}
	for i, line := range lines {
		role := failureanalysis.ImportRoleNoise
		if i < len(roles) {
			role = roles[i]
		}
		text := stripMarkers(line)
		if text == "" {
			continue
		}
		switch role {
		case failureanalysis.ImportRoleTitle:
			open(text)
		case failureanalysis.ImportRolePrecondition:
			if cur == nil {
				open("")
			}
			pre = append(pre, preLabel.ReplaceAllString(text, ""))
		case failureanalysis.ImportRoleStep:
			if cur == nil {
				open("")
			}
			cur.Steps = append(cur.Steps, splitStep(text))
		case failureanalysis.ImportRoleExpected:
			if cur == nil {
				open("")
			}
			text = expLabel.ReplaceAllString(text, "")
			if n := len(cur.Steps); n > 0 {
				if cur.Steps[n-1].ExpectedResult == "" {
					cur.Steps[n-1].ExpectedResult = text
				} else {
					cur.Steps[n-1].ExpectedResult += "\n" + text
				}
			} else {
				cur.Steps = append(cur.Steps, models.GeneratedStep{ExpectedResult: text})
			}
		}
	}
	flush()
	return out
}

// splitStep splits a step line into action and expected result on " -> " as well as the
// separators SplitActionExpected knows.
func splitStep(text string) models.GeneratedStep {
	if i := strings.Index(text, " -> "); i > 0 {
		return models.GeneratedStep{Action: strings.TrimSpace(text[:i]), ExpectedResult: strings.TrimSpace(text[i+4:])}
	}
	return SplitActionExpected(text)
}
