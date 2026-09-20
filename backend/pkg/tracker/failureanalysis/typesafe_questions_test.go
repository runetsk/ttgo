package failureanalysis

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"ttgo/pkg/tracker/models"

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
	for _, q := range []any{verdictQuestion(), defectTypeQuestion(), sameCauseQuestion(0, 1)} {
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
