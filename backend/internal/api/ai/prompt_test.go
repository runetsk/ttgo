package ai

import (
	"strings"
	"testing"
	"ttgo/pkg/tracker/models"
)

// legacyAssemblePrompt is a frozen copy of the pre-refactor ReplaceAll
// implementation, kept only to prove join(segments) is identical for normal
// inputs. req must be non-nil here (the legacy code never handled nil).
func legacyAssemblePrompt(template string, req *models.Requirement, childrenContext, coverageGuidance, detailLevel, additionalInstructions string) string {
	additionalInstr := ""
	if strings.TrimSpace(additionalInstructions) != "" {
		additionalInstr = "Additional Instructions: " + additionalInstructions
	}
	prompt := template
	prompt = strings.ReplaceAll(prompt, "{{COVERAGE}}", coverageGuidance)
	prompt = strings.ReplaceAll(prompt, "{{TITLE}}", req.Title)
	if strings.Contains(prompt, "{{CHILDREN}}") {
		prompt = strings.ReplaceAll(prompt, "{{CHILDREN}}", childrenContext)
		prompt = strings.ReplaceAll(prompt, "{{DESCRIPTION}}", req.Description)
	} else {
		prompt = strings.ReplaceAll(prompt, "{{DESCRIPTION}}", req.Description+childrenContext)
	}
	prompt = strings.ReplaceAll(prompt, "{{DETAIL_LEVEL}}", detailLevel)
	prompt = strings.ReplaceAll(prompt, "{{ADDITIONAL_INSTRUCTIONS}}", additionalInstr)
	return prompt
}

func segTypes(segs []PromptSegment) []string {
	out := make([]string, len(segs))
	for i, s := range segs {
		out[i] = s.Type
	}
	return out
}

func findSeg(t *testing.T, segs []PromptSegment, typ string) PromptSegment {
	t.Helper()
	for _, s := range segs {
		if s.Type == typ {
			return s
		}
	}
	t.Fatalf("no segment of type %q in %v", typ, segTypes(segs))
	return PromptSegment{}
}

func TestAssemblePromptSegments_AllPlaceholders(t *testing.T) {
	req := &models.Requirement{Title: "Login flow", Description: "Users must log in."}
	tpl := "Intro {{TITLE}} mid {{DESCRIPTION}} c:{{CHILDREN}} cov:{{COVERAGE}} d:{{DETAIL_LEVEL}} extra:{{ADDITIONAL_INSTRUCTIONS}} end"
	segs := assemblePromptSegments(tpl, req, "\nCHILD-CTX", "cover everything", "Standard", "focus on edge cases")

	want := []string{"template", "title", "template", "description", "template", "children", "template", "coverage", "template", "detail", "template", "instructions", "template"}
	got := segTypes(segs)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("segment order:\n got %v\nwant %v", got, want)
	}
	if s := findSeg(t, segs, "title"); s.Text != "Login flow" || s.Empty {
		t.Fatalf("title segment: %+v", s)
	}
	if s := findSeg(t, segs, "instructions"); s.Text != "Additional Instructions: focus on edge cases" {
		t.Fatalf("instructions segment: %+v", s)
	}
	if got := joinSegments(segs); got != legacyAssemblePrompt(tpl, req, "\nCHILD-CTX", "cover everything", "Standard", "focus on edge cases") {
		t.Fatalf("join != legacy:\n%s", got)
	}
}

func TestAssemblePromptSegments_RepeatedPlaceholder(t *testing.T) {
	req := &models.Requirement{Title: "T"}
	segs := assemblePromptSegments("{{TITLE}} and again {{TITLE}}", req, "", "cov", "Standard", "")
	count := 0
	for _, s := range segs {
		if s.Type == "title" {
			count++
			if s.Text != "T" {
				t.Fatalf("title text: %q", s.Text)
			}
		}
	}
	if count != 2 {
		t.Fatalf("want 2 title segments, got %d", count)
	}
}

func TestAssemblePromptSegments_NoPlaceholders(t *testing.T) {
	segs := assemblePromptSegments("static only", &models.Requirement{}, "", "cov", "Standard", "")
	if len(segs) != 1 || segs[0].Type != "template" || segs[0].Text != "static only" {
		t.Fatalf("got %+v", segs)
	}
}

func TestAssemblePromptSegments_ChildrenOmittedWhenEmpty(t *testing.T) {
	req := &models.Requirement{Title: "T", Description: "D"}
	// {{CHILDREN}} present but no children → segment omitted entirely
	segs := assemblePromptSegments("a{{CHILDREN}}b {{DESCRIPTION}}", req, "", "cov", "Standard", "")
	for _, s := range segs {
		if s.Type == "children" {
			t.Fatalf("children segment should be omitted: %+v", segs)
		}
	}
	if joinSegments(segs) != "ab D" {
		t.Fatalf("join: %q", joinSegments(segs))
	}
}

func TestAssemblePromptSegments_ChildrenAppendedAfterDescription(t *testing.T) {
	req := &models.Requirement{Title: "T", Description: "D"}
	// no {{CHILDREN}} in template → children ride along right after description
	segs := assemblePromptSegments("x {{DESCRIPTION}} y", req, "+KIDS", "cov", "Standard", "")
	want := []string{"template", "description", "children", "template"}
	if strings.Join(segTypes(segs), ",") != strings.Join(want, ",") {
		t.Fatalf("order: %v", segTypes(segs))
	}
	if joinSegments(segs) != "x D+KIDS y" {
		t.Fatalf("join: %q", joinSegments(segs))
	}
}

func TestAssemblePromptSegments_NilRequirement(t *testing.T) {
	segs := assemblePromptSegments("t:{{TITLE}} d:{{DESCRIPTION}}", nil, "", "cov", "Standard", "")
	if s := findSeg(t, segs, "title"); !s.Empty || s.Text != "" {
		t.Fatalf("title should be empty-flagged: %+v", s)
	}
	if s := findSeg(t, segs, "description"); !s.Empty || s.Text != "" {
		t.Fatalf("description should be empty-flagged: %+v", s)
	}
	if joinSegments(segs) != "t: d:" {
		t.Fatalf("join: %q", joinSegments(segs))
	}
}

func TestAssemblePromptSegments_EmptyInstructions(t *testing.T) {
	segs := assemblePromptSegments("i:{{ADDITIONAL_INSTRUCTIONS}}", &models.Requirement{}, "", "cov", "Standard", "   ")
	if s := findSeg(t, segs, "instructions"); !s.Empty || s.Text != "" {
		t.Fatalf("instructions should be empty-flagged: %+v", s)
	}
}

// Identity across realistic templates and values.
func TestAssemblePrompt_MatchesLegacy(t *testing.T) {
	cases := []struct {
		name                              string
		template                          string
		req                               *models.Requirement
		children, coverage, detail, instr string
	}{
		{"builtin no children", buildBuiltinTemplate(), &models.Requirement{Title: "Reset password", Description: "As a user…"}, "", "Cover happy paths.", "Detailed", "include a11y"},
		{"builtin with children", buildBuiltinTemplate(), &models.Requirement{Title: "Epic", Description: "Parent"}, "\n\nChild Issues / Sub-tickets:\n\n1. [C-1] Child", "Exhaust all categories.", "Standard", ""},
		{"builtin parent template", buildBuiltinParentTemplate(), &models.Requirement{Title: "Epic", Description: "Parent"}, "\n1. [C-1] Child", "guidance", "Simplified", "x"},
		{"empty description", buildBuiltinTemplate(), &models.Requirement{Title: "T"}, "", "g", "Standard", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := assemblePrompt(c.template, c.req, c.children, c.coverage, c.detail, c.instr)
			want := legacyAssemblePrompt(c.template, c.req, c.children, c.coverage, c.detail, c.instr)
			if got != want {
				t.Fatalf("mismatch:\n got: %q\nwant: %q", got, want)
			}
		})
	}
}

// Documents the intentional divergence: substituted values are now opaque, so
// placeholder-looking text inside a requirement no longer gets re-substituted.
func TestAssemblePrompt_ValuesAreOpaque(t *testing.T) {
	req := &models.Requirement{Title: "T", Description: "sneaky {{DETAIL_LEVEL}} text"}
	got := assemblePrompt("{{DESCRIPTION}}", req, "", "cov", "Standard", "")
	if got != "sneaky {{DETAIL_LEVEL}} text" {
		t.Fatalf("value should stay literal, got %q", got)
	}
	// legacy would have produced "sneaky Standard text" — that injection is gone.
}
