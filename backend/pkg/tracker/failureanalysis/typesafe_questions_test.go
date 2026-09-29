package failureanalysis

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

func criteriaKeys(t *testing.T, q any) []string {
	t.Helper()
	m, ok := q.(map[string]any)
	require.True(t, ok, "criteria must be a map")
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestVerdictQuestionOptionsMatchModelEnum(t *testing.T) {
	q := verdictQuestion()
	require.Equal(t, "choice", q.Type)
	want := make([]string, 0, len(models.ValidVerdicts))
	for v := range models.ValidVerdicts {
		want = append(want, v)
	}
	sort.Strings(want)
	require.Equal(t, want, criteriaKeys(t, q.Criteria))
}

func TestDefectTypeQuestionOptionsAreCanonicalPlusInsufficient(t *testing.T) {
	q := defectTypeQuestion()
	require.Equal(t, "choice", q.Type)
	require.Equal(t, []string{"automation_bug", DefectTypeInsufficient, "product_bug", "system_issue"}, criteriaKeys(t, q.Criteria))
	for _, k := range []string{"product_bug", "automation_bug", "system_issue"} {
		require.True(t, models.IsValidDefectType(k))
	}
}

func TestQuestionsContainNoTemplatePlaceholders(t *testing.T) {
	for _, q := range []any{verdictQuestion(), defectTypeQuestion(), sameCauseQuestion(0, 1), injectionQuestion(),
		flakyHistoryQuestion(), recurringQuestion(), outsideAppQuestion(), knownDefectQuestion(3)} {
		b, err := json.Marshal(q)
		require.NoError(t, err)
		s := string(b)
		require.NotContains(t, s, "{{")
		require.NotContains(t, s, "<verdict>")
		require.NotContains(t, s, "TODO")
	}
}

func TestSameCauseQuestionCarriesChunkLocalIndexes(t *testing.T) {
	q := sameCauseQuestion(3, 7)
	require.Equal(t, "noul", q.Type)
	b, _ := json.Marshal(q.Instructions)
	require.Contains(t, string(b), "`failures[3]`")
	require.Contains(t, string(b), "`failures[7]`")
	require.Contains(t, string(b), `"compare":[3,7]`)
	require.True(t, strings.Contains(string(b), "same failing operation"))
}

func TestConfidenceBucketBoundaries(t *testing.T) {
	require.Equal(t, models.ConfidenceHigh, confidenceBucket(0.90))
	require.Equal(t, models.ConfidenceHigh, confidenceBucket(1.0))
	require.Equal(t, models.ConfidenceMedium, confidenceBucket(0.899))
	require.Equal(t, models.ConfidenceMedium, confidenceBucket(0.50))
	require.Equal(t, models.ConfidenceLow, confidenceBucket(0.499))
	require.Equal(t, models.ConfidenceLow, confidenceBucket(0))
}

func TestCompanionQuestionsAreNoulWithTrueFalseCriteria(t *testing.T) {
	for key, q := range map[string]typesafe.Question{"injection": injectionQuestion(), "flaky_history": flakyHistoryQuestion(),
		"recurring": recurringQuestion(), "outside_app": outsideAppQuestion()} {
		require.Equal(t, "noul", q.Type, key)
		require.Equal(t, []string{"false", "true"}, criteriaKeys(t, q.Criteria), key)
	}
	b, _ := json.Marshal(injectionQuestion())
	for _, part := range []string{"`test`", "steps", "`failure`", "`history`", "`examples`", "`linked_defects`", "`group.related_failures`"} {
		require.Contains(t, string(b), part, "the guard covers every free-text field of the state (R8)")
	}
	b, _ = json.Marshal(flakyHistoryQuestion())
	require.Contains(t, string(b), "`history.recent_outcomes`")
	b, _ = json.Marshal(recurringQuestion())
	require.Contains(t, string(b), "`history.similar_failures`")
	require.Contains(t, string(b), "A redaction placeholder carries no information")
}

func TestKnownDefectQuestionUsesIndexOptions(t *testing.T) {
	q := knownDefectQuestion(3)
	require.Equal(t, "choice", q.Type)
	require.Equal(t, []string{"defect_0", "defect_1", "defect_2", KnownDefectNone}, criteriaKeys(t, q.Criteria))
	b, _ := json.Marshal(q)
	require.Contains(t, string(b), "`linked_defects[2]`")
	require.Contains(t, string(b), "`defect_2`")
	for i := 0; i < 3; i++ {
		got, ok := knownDefectIndex(knownDefectOption(i))
		require.True(t, ok)
		require.Equal(t, i, got)
	}
	for _, bad := range []string{"none", "defect_", "defect_x", "defect_01", "defect_+1", "defect_-1", "PAY-42"} {
		_, ok := knownDefectIndex(bad)
		require.False(t, ok, bad)
	}
}

func TestKnownDefectAskable(t *testing.T) {
	require.False(t, knownDefectAskable(nil))
	require.True(t, knownDefectAskable([]LinkedDefect{{Key: "PAY-42"}}))
	require.False(t, knownDefectAskable([]LinkedDefect{{Key: "<REDACTED_EMAIL>"}, {Key: "<REDACTED_EMAIL>"}}),
		"two defects that render to one key cannot be told apart (R5)")
	require.False(t, knownDefectAskable([]LinkedDefect{{Key: ""}}))
	many := make([]LinkedDefect, KnownDefectMaxOptions+1)
	for i := range many {
		many[i].Key = fmt.Sprintf("D-%d", i)
	}
	require.False(t, knownDefectAskable(many))
	require.True(t, knownDefectAskable(many[:KnownDefectMaxOptions]))
}

func TestDecisionQuestionsFollowTheSentEvidence(t *testing.T) {
	keys := func(qs map[string]typesafe.Question) []string {
		out := make([]string, 0, len(qs))
		for k := range qs {
			out = append(out, k)
		}
		sort.Strings(out)
		return out
	}
	always := []string{"defect_type", "injection", "outside_app", "verdict"}
	require.Equal(t, always, keys(decisionQuestions(Evidence{})))
	require.Equal(t, always, keys(decisionQuestions(Evidence{RecentOutcomes: "PF"})), "two outcomes cannot alternate")

	full := Evidence{RecentOutcomes: "PFP", SimilarFailures: []SimilarFailure{{ErrorMessage: "x"}},
		LinkedDefects: []LinkedDefect{{Key: "PAY-42"}, {Key: "PAY-43"}}}
	qs := decisionQuestions(full)
	require.Equal(t, []string{"defect_type", "flaky_history", "injection", "known_defect", "outside_app", "recurring", "verdict"}, keys(qs))
	require.Equal(t, []string{"defect_0", "defect_1", KnownDefectNone}, criteriaKeys(t, qs["known_defect"].Criteria))
	b, _ := json.Marshal(qs["known_defect"])
	require.NotContains(t, string(b), "PAY-42", "raw keys are never option text (R5)")
	require.True(t, strings.Contains(string(b), "`linked_defects[1]`"))
}
