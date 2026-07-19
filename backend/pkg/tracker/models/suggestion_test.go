package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSuggestedDefectType(t *testing.T) {
	tests := []struct {
		name    string
		verdict string
		want    string
	}{
		{"product bug maps directly", VerdictProductBug, "product_bug"},
		{"flaky test is an automation bug", VerdictFlakyTest, "automation_bug"},
		{"test data is an automation bug", VerdictTestData, "automation_bug"},
		{"environment is a system issue", VerdictEnvironment, "system_issue"},
		{"infrastructure is a system issue", VerdictInfrastructure, "system_issue"},
		{"unknown yields no suggestion", VerdictUnknown, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, SuggestedDefectType(tt.verdict))
		})
	}
}

// every verdict the parser accepts must be covered by the mapping switch, so a newly added
// verdict constant cannot silently fall through to "" unnoticed.
func TestSuggestedDefectTypeCoversAllValidVerdicts(t *testing.T) {
	require.Len(t, ValidVerdicts, 6, "verdict set changed - update SuggestedDefectType and its tests")

	for verdict := range ValidVerdicts {
		got := SuggestedDefectType(verdict)
		if verdict == VerdictUnknown {
			require.Empty(t, got, "unknown must not produce a suggestion")
			continue
		}
		require.Contains(t, []string{"product_bug", "automation_bug", "system_issue"}, got,
			"verdict %q mapped to unexpected defect type %q", verdict, got)
	}
}

func TestSuggestedDefectTypeUnrecognized(t *testing.T) {
	tests := []struct {
		name    string
		verdict string
	}{
		{"empty string", ""},
		{"garbage value", "not_a_verdict"},
		{"wrong case", "PRODUCT_BUG"},
		{"leading whitespace", " product_bug"},
		{"defect type passed as verdict", "automation_bug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Empty(t, SuggestedDefectType(tt.verdict),
				"unrecognized verdict must not produce an accidental suggestion")
		})
	}
}
