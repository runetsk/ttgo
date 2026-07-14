package aigen

import (
	"fmt"
	"regexp"
	"strings"

	"ttgo/pkg/tracker/models"
)

// QualityDimension groups rubric findings under an explainable axis
// (spec: "Explainable quality rubric"). Only dimensions that produced
// findings are returned; absence of a dimension means it is clean.
type QualityDimension struct {
	Key      string    `json:"key"`
	Label    string    `json:"label"`
	Findings []Finding `json:"findings"`
}

// QualityDimensionLabels enumerates every deterministic dimension.
// Stage 5 adds the LLM "critic" dimension on top.
var QualityDimensionLabels = map[string]string{
	"specificity":            "Test-data specificity",
	"action_clarity":         "Action clarity",
	"expected_observability": "Expected-result observability",
	"uniqueness":             "Scenario uniqueness",
	"traceability":           "Coverage traceability",
}

var (
	vagueExpectedRe = regexp.MustCompile(`(?i)^\s*(it\s+)?(works?|passes?|succeeds?|ok(ay)?|fine|correct(ly)?|as\s+expected|properly|successfull?y?|no\s+errors?|nothing\s+happens?)\s*[.!]?\s*$`)
	vagueActionRe   = regexp.MustCompile(`(?i)^\s*(do|check|verify|test|try)\s+(it|this|that|things?)\b`)
	concreteDataRe  = regexp.MustCompile(`\d|['"@=/]`)
)

// EvaluateDraftQuality runs every deterministic rubric rule against one draft.
// batchNameCounts maps NormalizeTestText(name) -> occurrences in the batch.
// targets gate the traceability dimension (no targets -> not evaluated).
func EvaluateDraftQuality(d models.GeneratedTestCase, batchNameCounts map[string]int, targets []CoverageTarget) []QualityDimension {
	var dims []QualityDimension
	dim := func(key string, findings []Finding) {
		if len(findings) > 0 {
			dims = append(dims, QualityDimension{Key: key, Label: QualityDimensionLabels[key], Findings: findings})
		}
	}
	warn := func(fs *[]Finding, field, code, msg string) {
		*fs = append(*fs, Finding{Field: field, Code: code, Message: msg, Severity: SeverityWarning})
	}

	// specificity: at least one step should carry concrete test data.
	var spec []Finding
	hasData := false
	for _, st := range d.Steps {
		if concreteDataRe.MatchString(st.Action) {
			hasData = true
			break
		}
	}
	if len(d.Steps) > 0 && !hasData {
		warn(&spec, "steps", "no_concrete_data",
			"no step action references concrete test data (values, accounts, inputs)")
	}
	dim("specificity", spec)

	// action_clarity
	var act []Finding
	for i, st := range d.Steps {
		a := strings.TrimSpace(st.Action)
		field := fmt.Sprintf("steps[%d].action", i)
		switch {
		case a == "":
			// structural validator owns required-field errors; skip here
		case vagueActionRe.MatchString(a):
			warn(&act, field, "vague_action", fmt.Sprintf("%q does not say what exactly to do", a))
		case len(a) < 12:
			warn(&act, field, "short_action", fmt.Sprintf("%q is too short to be actionable", a))
		}
	}
	dim("action_clarity", act)

	// expected_observability
	var exp []Finding
	for i, st := range d.Steps {
		e := strings.TrimSpace(st.ExpectedResult)
		field := fmt.Sprintf("steps[%d].expected_result", i)
		switch {
		case e == "":
			// structural validator owns required-field errors
		case vagueExpectedRe.MatchString(e) || len(e) < 10:
			warn(&exp, field, "vague_expected",
				fmt.Sprintf("%q is not an observable, verifiable outcome", e))
		}
	}
	dim("expected_observability", exp)

	// uniqueness (against batch names)
	var uniq []Finding
	if batchNameCounts[NormalizeTestText(d.Name)] > 1 {
		warn(&uniq, "name", "duplicate_name_in_batch", "another draft in this batch has the same (normalized) name")
	}
	dim("uniqueness", uniq)

	// traceability (only when targets exist)
	if len(targets) > 0 {
		known := make(map[string]bool, len(targets))
		for _, t := range targets {
			known[t.ID] = true
		}
		var tr []Finding
		if len(d.SourceRefs) == 0 {
			warn(&tr, "source_refs", "no_source_refs", "no source criterion linked — this draft maps to no coverage target")
		}
		for i, ref := range d.SourceRefs {
			if !known[NormalizeSourceRef(ref)] {
				warn(&tr, fmt.Sprintf("source_refs[%d]", i), "unknown_source_ref",
					fmt.Sprintf("%q does not match any derived coverage target", ref))
			}
		}
		dim("traceability", tr)
	}

	return dims
}

// EvaluateBatchQuality runs batch-level rules. Current rule: a batch of 3+
// drafts with no Negative-category case gets a negative-path warning.
func EvaluateBatchQuality(drafts []models.GeneratedTestCase) []Finding {
	if len(drafts) < 3 {
		return nil
	}
	for _, d := range drafts {
		if d.Category == "Negative" {
			return nil
		}
	}
	return []Finding{{
		Field: "batch", Code: "no_negative_case",
		Message:  "no negative-path test case in this batch",
		Severity: SeverityWarning,
	}}
}
