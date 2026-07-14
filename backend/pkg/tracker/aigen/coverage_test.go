package aigen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func TestStripHTML(t *testing.T) {
	html := `<h2>Acceptance Criteria</h2><ul><li>User can sign in</li><li>Errors are shown</li></ul><p>Note &amp; done</p>`
	text := StripHTML(html)
	assert.Contains(t, text, "Acceptance Criteria")
	assert.Contains(t, text, "- User can sign in")
	assert.Contains(t, text, "- Errors are shown")
	assert.Contains(t, text, "Note & done")
	assert.NotContains(t, text, "<")
}

func TestExtractCoverageTargets_ChildrenBecomeTargets(t *testing.T) {
	children := []*models.Requirement{
		{Identifier: "PROJ-2", Title: "Password reset"},
		{Identifier: "PROJ-3", Title: "Session expiry"},
	}
	targets := ExtractCoverageTargets("", children)
	require.Len(t, targets, 2)
	assert.Equal(t, "PROJ-2", targets[0].ID)
	assert.Equal(t, TargetKindChildRequirement, targets[0].Kind)
	assert.Equal(t, "Password reset", targets[0].Text)
}

func TestExtractCoverageTargets_BulletsUnderACHeading(t *testing.T) {
	desc := `<h2>Background</h2><ul><li>ignore this context bullet</li></ul>` +
		`<h2>Acceptance Criteria</h2><ul><li>User can sign in with email</li><li>Wrong password shows an error</li></ul>` +
		`<h2>Notes</h2><ul><li>ignore this too</li></ul>`
	targets := ExtractCoverageTargets(desc, nil)
	require.Len(t, targets, 2, "only bullets under the AC heading count when the heading exists")
	assert.Equal(t, "AC-1", targets[0].ID)
	assert.Equal(t, TargetKindAcceptanceCriterion, targets[0].Kind)
	assert.Equal(t, "User can sign in with email", targets[0].Text)
	assert.Equal(t, "AC-2", targets[1].ID)
}

func TestExtractCoverageTargets_AllBulletsWithoutHeading(t *testing.T) {
	desc := `<p>Login form.</p><ul><li>Field validation</li><li>Remember me</li></ul>`
	targets := ExtractCoverageTargets(desc, nil)
	require.Len(t, targets, 2)
	assert.Equal(t, "AC-1", targets[0].ID)
	assert.Equal(t, "Field validation", targets[0].Text)
}

func TestExtractCoverageTargets_ExplicitIDsAndDedup(t *testing.T) {
	desc := `<ul><li>AC-7: Lockout after 5 attempts</li><li>PROJ-9 Reset link expires</li><li>Plain bullet</li></ul>`
	children := []*models.Requirement{{Identifier: "PROJ-9", Title: "Reset"}}
	targets := ExtractCoverageTargets(desc, children)
	ids := make([]string, len(targets))
	for i, tg := range targets {
		ids[i] = tg.ID
	}
	// Children come first; the PROJ-9 bullet dedupes against the child target.
	assert.Equal(t, []string{"PROJ-9", "AC-7", "AC-1"}, ids)
	assert.Equal(t, "Lockout after 5 attempts", targets[1].Text)
}

func TestExtractCoverageTargets_PlainTextBulletsAndNumbers(t *testing.T) {
	desc := "Acceptance criteria\n1. First rule\n2) Second rule\n* Third rule\n"
	targets := ExtractCoverageTargets(desc, nil)
	require.Len(t, targets, 3)
	assert.Equal(t, "First rule", targets[0].Text)
}

func TestExtractCoverageTargets_CapAndEmpty(t *testing.T) {
	assert.Empty(t, ExtractCoverageTargets("", nil))
	assert.Empty(t, ExtractCoverageTargets("<p>prose only, no bullets</p>", nil))

	long := ""
	for i := 0; i < MaxCoverageTargets+10; i++ {
		long += fmt.Sprintf("<li>rule %d</li>", i)
	}
	targets := ExtractCoverageTargets("<ul>"+long+"</ul>", nil)
	assert.Len(t, targets, MaxCoverageTargets)
}

func draftWithRefs(name string, refs ...string) models.GeneratedTestCase {
	return models.GeneratedTestCase{
		Name: name, Category: "Functional", Description: "d",
		SourceRefs: refs,
		Steps:      []models.GeneratedStep{{Action: "a", ExpectedResult: "e"}},
	}
}

func TestNormalizeSourceRef(t *testing.T) {
	assert.Equal(t, "AC-1", NormalizeSourceRef(" ac 1 "))
	assert.Equal(t, "AC-2", NormalizeSourceRef("AC_2"))
	assert.Equal(t, "PROJ-9", NormalizeSourceRef("[proj-9]"))
	assert.Equal(t, "", NormalizeSourceRef("  "))
}

func TestBuildCoverageReport(t *testing.T) {
	targets := []CoverageTarget{
		{ID: "AC-1", Kind: TargetKindAcceptanceCriterion, Text: "sign in"},
		{ID: "AC-2", Kind: TargetKindAcceptanceCriterion, Text: "error shown"},
		{ID: "PROJ-2", Kind: TargetKindChildRequirement, Text: "reset"},
	}
	drafts := []models.GeneratedTestCase{
		draftWithRefs("t0", "AC-1"),
		draftWithRefs("t1", "ac 1"), // normalizes to AC-1
		draftWithRefs("t2", "AC-1"),
		draftWithRefs("t3", "AC-1", "UNKNOWN-99"), // unknown ref is ignored here (rubric flags it)
	}
	rep := BuildCoverageReport(targets, drafts)

	require.Len(t, rep.Targets, 3)
	ac1 := rep.Targets[0]
	assert.Equal(t, TargetStatusOverRepresented, ac1.Status, "4 drafts >= threshold")
	assert.Equal(t, []int{0, 1, 2, 3}, ac1.DraftPositions)
	assert.Equal(t, TargetStatusUncovered, rep.Targets[1].Status)
	assert.Equal(t, TargetStatusUncovered, rep.Targets[2].Status)
	assert.Equal(t, 1, rep.CoveredCount, "over-represented still counts as covered")
	assert.Equal(t, 2, rep.UncoveredCount)
	assert.Equal(t, 1, rep.OverRepresentedCount)
	assert.Empty(t, rep.BatchFindings, "BuildCoverageReport itself adds no batch findings")
}

func TestBuildCoverageReport_NoTargets(t *testing.T) {
	rep := BuildCoverageReport(nil, []models.GeneratedTestCase{draftWithRefs("t0")})
	assert.Empty(t, rep.Targets)
	assert.Zero(t, rep.UncoveredCount)
}
