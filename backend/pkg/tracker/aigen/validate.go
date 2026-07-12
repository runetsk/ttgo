package aigen

import (
	"fmt"
	"strings"
	"ttgo/pkg/tracker/models"
)

// Severity of a validation finding. Errors block acceptance; warnings do not.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Finding is one field-level validation result with a human-readable reason.
type Finding struct {
	Field    string   `json:"field"`
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Severity Severity `json:"severity"`
}

// Deterministic limits (bound stored payloads; see spec "Privacy and Security").
const (
	MaxNameLen        = 200
	MaxCategoryLen    = 60
	MaxDescriptionLen = 5000
	MaxStepFieldLen   = 4000
	MaxSteps          = 50
	MaxSourceRefs     = 20
	MaxSourceRefLen   = 100
)

// KnownCategories mirrors the category list in the default prompt template
// (store/ai_generation.go). Unknown categories are warnings, not errors,
// because the template explicitly allows custom category names.
var KnownCategories = map[string]bool{
	"Functional": true, "Negative": true, "Boundary": true, "Edge Case": true,
	"Security": true, "Performance": true, "API": true,
	"Mobile/Responsive": true, "Accessibility": true,
}

// ValidateDraft runs every deterministic rule against one draft and returns
// field-level findings. An empty slice means the draft is clean.
func ValidateDraft(d models.GeneratedTestCase) []Finding {
	var fs []Finding
	err := func(field, code, msg string) {
		fs = append(fs, Finding{Field: field, Code: code, Message: msg, Severity: SeverityError})
	}
	warn := func(field, code, msg string) {
		fs = append(fs, Finding{Field: field, Code: code, Message: msg, Severity: SeverityWarning})
	}

	if strings.TrimSpace(d.Name) == "" {
		err("name", "required", "test case name is required")
	} else if len(d.Name) > MaxNameLen {
		err("name", "too_long", fmt.Sprintf("name exceeds %d characters", MaxNameLen))
	}

	switch {
	case strings.TrimSpace(d.Category) == "":
		warn("category", "missing_category", "no category assigned; the draft will land directly in the target folder")
	case len(d.Category) > MaxCategoryLen:
		err("category", "too_long", fmt.Sprintf("category exceeds %d characters", MaxCategoryLen))
	case !KnownCategories[d.Category]:
		warn("category", "unknown_category", fmt.Sprintf("%q is not one of the standard categories", d.Category))
	}

	if len(d.Description) > MaxDescriptionLen {
		err("description", "too_long", fmt.Sprintf("description exceeds %d characters", MaxDescriptionLen))
	}

	switch {
	case len(d.Steps) == 0:
		err("steps", "no_steps", "a test case needs at least one step")
	case len(d.Steps) > MaxSteps:
		err("steps", "too_many_steps", fmt.Sprintf("more than %d steps", MaxSteps))
	}
	for i, st := range d.Steps {
		if strings.TrimSpace(st.Action) == "" {
			err(fmt.Sprintf("steps[%d].action", i), "required", "step action is required")
		} else if len(st.Action) > MaxStepFieldLen {
			err(fmt.Sprintf("steps[%d].action", i), "too_long", fmt.Sprintf("action exceeds %d characters", MaxStepFieldLen))
		}
		if strings.TrimSpace(st.ExpectedResult) == "" {
			err(fmt.Sprintf("steps[%d].expected_result", i), "required", "expected result is required")
		} else if len(st.ExpectedResult) > MaxStepFieldLen {
			err(fmt.Sprintf("steps[%d].expected_result", i), "too_long", fmt.Sprintf("expected result exceeds %d characters", MaxStepFieldLen))
		}
	}

	if len(d.SourceRefs) > MaxSourceRefs {
		warn("source_refs", "too_many_refs", fmt.Sprintf("more than %d source refs", MaxSourceRefs))
	}
	for i, ref := range d.SourceRefs {
		if len(ref) > MaxSourceRefLen {
			warn(fmt.Sprintf("source_refs[%d]", i), "ref_too_long", fmt.Sprintf("source ref exceeds %d characters", MaxSourceRefLen))
		}
	}
	return fs
}

// HasErrors reports whether any finding is error-severity (blocks acceptance).
func HasErrors(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity == SeverityError {
			return true
		}
	}
	return false
}
