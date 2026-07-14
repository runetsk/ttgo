package aigen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func cleanDraft() models.GeneratedTestCase {
	return models.GeneratedTestCase{
		Name:        "[Functional] Sign in with valid credentials",
		Category:    "Functional",
		Description: "Verifies successful authentication.",
		SourceRefs:  []string{"AC-1"},
		Steps: []models.GeneratedStep{
			{Action: `Enter "user@example.com" in the Email field`, ExpectedResult: "The Email field contains user@example.com"},
			{Action: `Click the "Sign in" button`, ExpectedResult: "The dashboard page is displayed with the user menu visible"},
		},
	}
}

func dimByKey(dims []QualityDimension, key string) *QualityDimension {
	for i := range dims {
		if dims[i].Key == key {
			return &dims[i]
		}
	}
	return nil
}

func oneTarget() []CoverageTarget {
	return []CoverageTarget{{ID: "AC-1", Kind: TargetKindAcceptanceCriterion, Text: "sign in"}}
}

func TestEvaluateDraftQuality_CleanDraftHasNoDimensions(t *testing.T) {
	dims := EvaluateDraftQuality(cleanDraft(), map[string]int{}, oneTarget())
	assert.Empty(t, dims, "clean drafts return no findings-bearing dimensions")
}

func TestEvaluateDraftQuality_VagueExpectedResult(t *testing.T) {
	d := cleanDraft()
	d.Steps[1].ExpectedResult = "It works"
	dims := EvaluateDraftQuality(d, map[string]int{}, oneTarget())
	dim := dimByKey(dims, "expected_observability")
	require.NotNil(t, dim)
	require.Len(t, dim.Findings, 1)
	assert.Equal(t, "steps[1].expected_result", dim.Findings[0].Field)
	assert.Equal(t, "vague_expected", dim.Findings[0].Code)
	assert.Equal(t, SeverityWarning, dim.Findings[0].Severity)
	assert.NotEmpty(t, dim.Findings[0].Message)
}

func TestEvaluateDraftQuality_VagueAndShortActions(t *testing.T) {
	d := cleanDraft()
	d.Steps[0].Action = "Check it"
	dims := EvaluateDraftQuality(d, map[string]int{}, oneTarget())
	dim := dimByKey(dims, "action_clarity")
	require.NotNil(t, dim)
	assert.Equal(t, "steps[0].action", dim.Findings[0].Field)
	assert.Equal(t, "vague_action", dim.Findings[0].Code)
}

func TestEvaluateDraftQuality_NoConcreteData(t *testing.T) {
	d := cleanDraft()
	d.Steps = []models.GeneratedStep{
		{Action: "Open the login page", ExpectedResult: "The login page is displayed"},
		{Action: "Submit the form", ExpectedResult: "A confirmation message is displayed"},
	}
	dims := EvaluateDraftQuality(d, map[string]int{}, oneTarget())
	dim := dimByKey(dims, "specificity")
	require.NotNil(t, dim)
	assert.Equal(t, "steps", dim.Findings[0].Field)
	assert.Equal(t, "no_concrete_data", dim.Findings[0].Code)
}

func TestEvaluateDraftQuality_DuplicateNameInBatch(t *testing.T) {
	d := cleanDraft()
	counts := map[string]int{NormalizeTestText(d.Name): 2}
	dims := EvaluateDraftQuality(d, counts, oneTarget())
	dim := dimByKey(dims, "uniqueness")
	require.NotNil(t, dim)
	assert.Equal(t, "duplicate_name_in_batch", dim.Findings[0].Code)
}

func TestEvaluateDraftQuality_Traceability(t *testing.T) {
	d := cleanDraft()
	d.SourceRefs = nil
	dims := EvaluateDraftQuality(d, map[string]int{}, oneTarget())
	dim := dimByKey(dims, "traceability")
	require.NotNil(t, dim)
	assert.Equal(t, "no_source_refs", dim.Findings[0].Code)

	d.SourceRefs = []string{"AC-1", "BOGUS-7"}
	dims = EvaluateDraftQuality(d, map[string]int{}, oneTarget())
	dim = dimByKey(dims, "traceability")
	require.NotNil(t, dim)
	assert.Equal(t, "source_refs[1]", dim.Findings[0].Field)
	assert.Equal(t, "unknown_source_ref", dim.Findings[0].Code)

	// No targets derived => traceability is not evaluated at all.
	dims = EvaluateDraftQuality(d, map[string]int{}, nil)
	assert.Nil(t, dimByKey(dims, "traceability"))
}

func TestEvaluateBatchQuality_NegativePath(t *testing.T) {
	batch := []models.GeneratedTestCase{cleanDraft(), cleanDraft(), cleanDraft()}
	fs := EvaluateBatchQuality(batch)
	require.Len(t, fs, 1)
	assert.Equal(t, "batch", fs[0].Field)
	assert.Equal(t, "no_negative_case", fs[0].Code)
	assert.Equal(t, SeverityWarning, fs[0].Severity)

	neg := cleanDraft()
	neg.Category = "Negative"
	assert.Empty(t, EvaluateBatchQuality([]models.GeneratedTestCase{cleanDraft(), cleanDraft(), neg}))
	assert.Empty(t, EvaluateBatchQuality([]models.GeneratedTestCase{cleanDraft(), cleanDraft()}), "small batches are exempt")
}

func TestEvaluateDraftQuality_EveryFindingExplainsItself(t *testing.T) {
	d := models.GeneratedTestCase{
		Name:  "x",
		Steps: []models.GeneratedStep{{Action: "Do it", ExpectedResult: "ok"}},
	}
	for _, dim := range EvaluateDraftQuality(d, map[string]int{}, oneTarget()) {
		assert.NotEmpty(t, dim.Label)
		for _, f := range dim.Findings {
			assert.NotEmpty(t, f.Message, "%s:%s must explain itself", f.Field, f.Code)
			assert.Equal(t, SeverityWarning, f.Severity, "rubric findings never block acceptance")
		}
	}
}
