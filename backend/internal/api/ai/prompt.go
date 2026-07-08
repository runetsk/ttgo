package ai

import (
	"strings"

	"ttgo/pkg/tracker/models"
)

// PromptSegment is one typed piece of the assembled LLM prompt. The full
// prompt is the concatenation of segment texts in order — generation and the
// prompt preview share this representation so they can never drift.
type PromptSegment struct {
	Type  string `json:"type"` // template | title | description | children | coverage | detail | instructions
	Text  string `json:"text"`
	Empty bool   `json:"empty,omitempty"` // the segment's source value is empty (e.g. no requirement linked)
}

type placeholderSpec struct {
	token string
	typ   string
}

var promptPlaceholders = []placeholderSpec{
	{"{{TITLE}}", "title"},
	{"{{DESCRIPTION}}", "description"},
	{"{{CHILDREN}}", "children"},
	{"{{COVERAGE}}", "coverage"},
	{"{{DETAIL_LEVEL}}", "detail"},
	{"{{ADDITIONAL_INSTRUCTIONS}}", "instructions"},
}

// assemblePromptSegments tokenizes the template on its placeholders and fills
// each occurrence from the given values. Substituted values are opaque —
// placeholder-looking text inside a value is never re-substituted. req may be
// nil (preview with no linked requirement): title/description come back
// empty-flagged. Children segments are omitted entirely when childrenContext
// is empty so the UI never shows a stray legend entry; join identity holds
// because the omitted text is "".
func assemblePromptSegments(template string, req *models.Requirement, childrenContext, coverageGuidance, detailLevel, additionalInstructions string) []PromptSegment {
	title, description := "", ""
	if req != nil {
		title = req.Title
		description = req.Description
	}
	instructions := ""
	if strings.TrimSpace(additionalInstructions) != "" {
		instructions = "Additional Instructions: " + additionalInstructions
	}
	hasChildrenPlaceholder := strings.Contains(template, "{{CHILDREN}}")

	childSegs := func() []PromptSegment {
		if childrenContext == "" {
			return nil
		}
		return []PromptSegment{{Type: "children", Text: childrenContext}}
	}
	valueSegs := func(typ string) []PromptSegment {
		switch typ {
		case "title":
			return []PromptSegment{{Type: "title", Text: title, Empty: title == ""}}
		case "description":
			out := []PromptSegment{{Type: "description", Text: description, Empty: description == ""}}
			if !hasChildrenPlaceholder {
				// Matches the historical req.Description+childrenContext append.
				out = append(out, childSegs()...)
			}
			return out
		case "children":
			return childSegs()
		case "coverage":
			return []PromptSegment{{Type: "coverage", Text: coverageGuidance}}
		case "detail":
			return []PromptSegment{{Type: "detail", Text: detailLevel}}
		case "instructions":
			return []PromptSegment{{Type: "instructions", Text: instructions, Empty: instructions == ""}}
		}
		return nil
	}

	var segs []PromptSegment
	rest := template
	for {
		bestIdx, bestLen, bestType := -1, 0, ""
		for _, ph := range promptPlaceholders {
			if i := strings.Index(rest, ph.token); i != -1 && (bestIdx == -1 || i < bestIdx) {
				bestIdx, bestLen, bestType = i, len(ph.token), ph.typ
			}
		}
		if bestIdx == -1 {
			break
		}
		if bestIdx > 0 {
			segs = append(segs, PromptSegment{Type: "template", Text: rest[:bestIdx]})
		}
		segs = append(segs, valueSegs(bestType)...)
		rest = rest[bestIdx+bestLen:]
	}
	if rest != "" {
		segs = append(segs, PromptSegment{Type: "template", Text: rest})
	}
	return segs
}

// joinSegments concatenates segment texts into the final prompt string.
func joinSegments(segs []PromptSegment) string {
	var sb strings.Builder
	for _, s := range segs {
		sb.WriteString(s.Text)
	}
	return sb.String()
}

// assemblePrompt builds the final LLM prompt by substituting template
// variables. It is the join of assemblePromptSegments.
func assemblePrompt(template string, req *models.Requirement, childrenContext, coverageGuidance, detailLevel, additionalInstructions string) string {
	return joinSegments(assemblePromptSegments(template, req, childrenContext, coverageGuidance, detailLevel, additionalInstructions))
}
