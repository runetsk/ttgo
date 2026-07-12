package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"ttgo/internal/api/httpx"
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

// generationSystemMessage instructs the model to skip reasoning / <think>
// blocks and return JSON directly (critical for CoT models like DeepSeek-R1).
// Shared by GenerateTests and the prompt preview.
const generationSystemMessage = "You are a JSON API that generates software test cases. " +
	"Respond with ONLY a single valid JSON object of the form {\"test_cases\": [...]}. " +
	"Do NOT include <think> tags, markdown fences, or any text outside the JSON. " +
	"Start your response with the { character."

var (
	errUnknownCoverageLevel = errors.New("coverage_level must be one of: essential, thorough, comprehensive")
	errRequirementNotFound  = errors.New("requirement not found")
)

// writePromptPlanError maps buildPromptPlan errors onto HTTP responses for
// both GenerateTests and PreviewGenerationPrompt. buildPromptPlan only returns
// the two sentinels today; anything else would be an internal failure, not a
// client error.
func writePromptPlanError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errRequirementNotFound):
		httpx.Error(w, http.StatusNotFound, err)
	case errors.Is(err, errUnknownCoverageLevel):
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	default:
		httpx.Error(w, http.StatusInternalServerError, err)
	}
}

// promptPlan is everything needed to issue (or preview) a generation request.
type promptPlan struct {
	Segments        []PromptSegment
	Prompt          string
	SystemMessage   string
	TemplateType    string // "standard" | "parent"
	TemplateWarning string
	CoverageLevel   string // resolved (post-default) — used by the audit log
	MaxTokens       int
}

// buildPromptPlan resolves defaults, requirement + children context, coverage
// guidance, template selection, and token budget — the single source of truth
// for what a generation request will send. requirementID may be empty (prompt
// preview before a requirement is linked): requirement-derived segments come
// back empty-flagged.
func (h *Handler) buildPromptPlan(requirementID, coverageLevel, detailLevel, additionalInstructions string) (*promptPlan, error) {
	if coverageLevel == "" {
		coverageLevel = "thorough"
	}
	coverageGuidance := coverageLevelGuidance(coverageLevel)
	if coverageGuidance == "" {
		return nil, errUnknownCoverageLevel
	}
	if detailLevel == "" {
		detailLevel = "Standard"
	}

	var requirement *models.Requirement
	var children []*models.Requirement
	if requirementID != "" {
		var err error
		requirement, err = h.store.GetRequirement(requirementID)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errRequirementNotFound, err)
		}
		children, _ = h.store.ListChildRequirements(requirementID)
	}
	childrenContext := buildChildrenContext(children)

	// Scale coverage guidance when children are present but standard template is used
	// (parent template already has its own children-aware instructions)
	if len(children) > 0 {
		coverageGuidance += fmt.Sprintf(
			"\n\nIMPORTANT: This requirement has %d child issues/sub-tickets. "+
				"Generate at least one test case per child issue to ensure full coverage.",
			len(children))
	}

	coverageCfg, _ := h.store.GetOrCreateCoverageConfig()
	maxTokens := coverageMaxTokens(coverageLevel, coverageCfg)

	// Load template — use parent template when children are present.
	templateWarning := ""
	templateType := "standard"
	var promptTemplate string
	tmpl, err := h.store.GetOrCreateDefaultTemplate()
	if err != nil {
		templateWarning = "Using built-in default template (custom template unavailable)"
		if len(children) > 0 {
			promptTemplate = buildBuiltinParentTemplate()
			templateType = "parent"
		} else {
			promptTemplate = buildBuiltinTemplate()
		}
	} else if len(children) > 0 && strings.TrimSpace(tmpl.ParentContent) != "" {
		promptTemplate = tmpl.ParentContent
		templateType = "parent"
	} else if strings.TrimSpace(tmpl.Content) != "" {
		promptTemplate = tmpl.Content
	} else {
		templateWarning = "Using built-in default template (custom template unavailable)"
		promptTemplate = buildBuiltinTemplate()
	}

	segments := assemblePromptSegments(promptTemplate, requirement, childrenContext, coverageGuidance, detailLevel, additionalInstructions)
	return &promptPlan{
		Segments:        segments,
		Prompt:          joinSegments(segments),
		SystemMessage:   generationSystemMessage,
		TemplateType:    templateType,
		TemplateWarning: templateWarning,
		CoverageLevel:   coverageLevel,
		MaxTokens:       maxTokens,
	}, nil
}

// PreviewGenerationPrompt returns the exact prompt GenerateTests would send
// for the given options, without calling the LLM. requirement_id is optional.
func (h *Handler) PreviewGenerationPrompt(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RequirementID          string `json:"requirement_id"`
		CoverageLevel          string `json:"coverage_level"`
		DetailLevel            string `json:"detail_level"`
		AdditionalInstructions string `json:"additional_instructions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	plan, err := h.buildPromptPlan(req.RequirementID, req.CoverageLevel, req.DetailLevel, req.AdditionalInstructions)
	if err != nil {
		writePromptPlanError(w, err)
		return
	}
	out := map[string]interface{}{
		"system_message": plan.SystemMessage,
		"segments":       plan.Segments,
		"template_type":  plan.TemplateType,
		"max_tokens":     plan.MaxTokens,
	}
	if plan.TemplateWarning != "" {
		out["template_warning"] = plan.TemplateWarning
	}
	httpx.JSON(w, http.StatusOK, out)
}
