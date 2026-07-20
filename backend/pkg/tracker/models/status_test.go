package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsFailureStatus(t *testing.T) {
	tests := []struct {
		name   string
		status ExecutionStatus
		want   bool
	}{
		{"fail is a failure", StatusFail, true},
		{"error is a failure", StatusError, true},
		{"pass is not a failure", StatusPass, false},
		{"skip is not a failure", StatusSkip, false},
		{"pending is not a failure", StatusPending, false},
		{"running is not a failure", StatusRunning, false},
		{"empty is not a failure", ExecutionStatus(""), false},
		{"garbage is not a failure", ExecutionStatus("not_a_status"), false},
		{"wrong case is not a failure", ExecutionStatus("fail"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsFailureStatus(tt.status))
		})
	}
}

// The failure set must be a subset of the valid execution statuses: a status nothing can ever be
// stored as cannot be a failure, and a SQL `status IN (...)` built from one would silently match
// no rows. Asserted as the actual containment, not as a count of how many valid statuses happen to
// be failures — a count holds just as well when the two sets have drifted apart.
func TestFailureStatusesAreValidExecutionStatuses(t *testing.T) {
	require.NotEmpty(t, FailureStatuses)
	for _, status := range FailureStatuses {
		require.True(t, ValidExecutionStatuses[status], "%q is a failure status but not a valid execution status", status)
	}
}

// FailureStatuses (what SQL binds) and IsFailureStatus (what Go branches on) are two spellings of
// one set, so they are pinned to each other here: adding a third failure status to either alone
// would make the store's triage guard and the handlers' partition disagree about which rows are
// triageable.
func TestFailureStatusesMatchesPredicate(t *testing.T) {
	inSlice := func(s ExecutionStatus) bool {
		for _, f := range FailureStatuses {
			if f == s {
				return true
			}
		}
		return false
	}
	for status := range ValidExecutionStatuses {
		require.Equal(t, IsFailureStatus(status), inSlice(status), "the two spellings of the failure set disagree on %q", status)
	}
	require.Len(t, FailureStatuses, 2, "execution status set changed - review IsFailureStatus and its tests")
}

func TestIsValidDefectType(t *testing.T) {
	tests := []struct {
		name       string
		defectType string
		want       bool
	}{
		{"product bug", "product_bug", true},
		{"automation bug", "automation_bug", true},
		{"system issue", "system_issue", true},
		{"to investigate", "to_investigate", true},
		// "" is the not-applicable value the non-failure path writes to clear defect_type;
		// rejecting it would break that clear path.
		{"empty clears the field", "", true},
		{"garbage value", "not_a_defect_type", false},
		{"whitespace only", " ", false},
		{"leading whitespace", " product_bug", false},
		{"trailing whitespace", "product_bug ", false},
		{"wrong case", "PRODUCT_BUG", false},
		{"mixed case", "Product_Bug", false},
		{"verdict passed as defect type", "flaky_test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsValidDefectType(tt.defectType))
		})
	}
}

// the canonical set is duplicated as literals in the store and the frontend; pin its size so a
// change here is a deliberate one that prompts updating those sites too.
func TestValidDefectTypesCanonicalSet(t *testing.T) {
	require.Len(t, ValidDefectTypes, 4, "defect type set changed - sync store/runs.go, store/ai_accuracy.go and the frontend")

	for defectType := range ValidDefectTypes {
		require.True(t, IsValidDefectType(defectType), "canonical defect type %q must validate", defectType)
	}
}
