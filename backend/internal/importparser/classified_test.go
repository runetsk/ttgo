package importparser

import (
	"strings"
	"testing"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

const (
	tT = failureanalysis.ImportRoleTitle
	tP = failureanalysis.ImportRolePrecondition
	tS = failureanalysis.ImportRoleStep
	tE = failureanalysis.ImportRoleExpected
	tN = failureanalysis.ImportRoleNoise
)

func TestImportLines_TrimsSkipsBlankAndCaps(t *testing.T) {
	lines, trunc := ImportLines("  Login \r\n\n\t- open page  \n" + strings.Repeat("x", 500))
	require.False(t, trunc)
	require.Equal(t, []string{"Login", "- open page", strings.Repeat("x", failureanalysis.ImportLineChars)}, lines)

	many := strings.Repeat("line\n", failureanalysis.ImportMaxLines+5)
	lines, trunc = ImportLines(many)
	require.True(t, trunc)
	require.Len(t, lines, failureanalysis.ImportMaxLines)
}

func TestAssembleClassified_BuildsCasesFromRolesWithoutInventingSteps(t *testing.T) {
	lines := []string{
		"Test plan v2 — Payments", // noise
		"## TC-1: Pay with a saved card",
		"Precondition: user has a saved Visa",
		"1. Open the checkout page",
		"2. Click Pay -> Payment form opens",
		"Expected: the order is confirmed",
		"Also a receipt email arrives",
		"**TC-2: Refund an order**",
		"- Open the order",
		"- Click Refund",
		"The order shows Refunded",
		"Page 3 of 7", // noise
	}
	roles := []string{tN, tT, tP, tS, tS, tE, tE, tT, tS, tS, tE, tN}
	got := AssembleClassified(lines, roles)
	require.Equal(t, []models.GeneratedTestCase{
		{Name: "Pay with a saved card", Description: "Preconditions: user has a saved Visa", Steps: []models.GeneratedStep{
			{Action: "Open the checkout page"},
			{Action: "Click Pay", ExpectedResult: "Payment form opens\nthe order is confirmed\nAlso a receipt email arrives"},
		}},
		{Name: "Refund an order", Steps: []models.GeneratedStep{
			{Action: "Open the order"},
			{Action: "Click Refund", ExpectedResult: "The order shows Refunded"},
		}},
	}, got)
}

func TestAssembleClassified_UntitledAndEmptyCases(t *testing.T) {
	got := AssembleClassified(
		[]string{"Expected: a banner shows", "Click Save", "TC: Only a title", "Another"},
		[]string{tE, tS, tT, tT})
	require.Len(t, got, 1, "a title with neither steps nor preconditions is dropped")
	require.Equal(t, "Imported test case 1", got[0].Name)
	require.Equal(t, []models.GeneratedStep{{ExpectedResult: "a banner shows"}, {Action: "Click Save"}}, got[0].Steps)

	require.Empty(t, AssembleClassified([]string{"a", "b"}, []string{tN, tN}))
	require.Empty(t, AssembleClassified([]string{"a"}, nil), "a line without a role is noise")
}

func TestStripMarkers(t *testing.T) {
	for in, want := range map[string]string{
		"1. Open the page":       "Open the page",
		"2) Click":               "Click",
		"- Enter the code":       "Enter the code",
		"* Submit":               "Submit",
		"Step 3: Verify":         "Verify",
		"### Login":              "Login",
		"**TC-2: Refund**":       "Refund",
		"1. **Step 2:** Log out": "Log out",
		"Plain text":             "Plain text",
	} {
		require.Equal(t, want, stripMarkers(in), in)
	}
}
