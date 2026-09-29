package failureanalysis

import (
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// eligibleResult is a direct TypeSafe decision that qualifies at a 0.95 threshold.
func eligibleResult() *AnalyzeResult {
	return &AnalyzeResult{
		Engine: models.AnalysisEngineTypeSafe, DecisionStatus: models.DecisionStatusOK,
		Verdict: models.VerdictProductBug, SuggestedDefectType: "product_bug",
		SuggestedDefectTypeConfidence: f64ptr(0.97), Signals: SignalsJSON(Signals{Injection: f64ptr(0.02)}),
	}
}

func TestAutoApplyValue_Eligibility(t *testing.T) {
	cases := []struct {
		name string
		edit func(r *AnalyzeResult)
		min  float64
		want string
		ok   bool
	}{
		{"direct TypeSafe answer above the threshold", nil, 0.95, "product_bug", true},
		{"exactly at the threshold", func(r *AnalyzeResult) { r.SuggestedDefectTypeConfidence = f64ptr(0.95) }, 0.95, "product_bug", true},
		{"below the threshold", func(r *AnalyzeResult) { r.SuggestedDefectTypeConfidence = f64ptr(0.949) }, 0.95, "", false},
		{"no defect-type confidence", func(r *AnalyzeResult) { r.SuggestedDefectTypeConfidence = nil }, 0.95, "", false},
		{"generative engine", func(r *AnalyzeResult) { r.Engine = models.AnalysisEngineGenerative }, 0.95, "", false},
		{"failed attempt", func(r *AnalyzeResult) { r.DecisionStatus = models.DecisionStatusFailed }, 0.95, "", false},
		{"takeover", func(r *AnalyzeResult) { r.TakeoverFromVerdict = models.VerdictFlakyTest }, 0.95, "", false},
		{"derived from the verdict", func(r *AnalyzeResult) { r.SuggestionSource = models.SuggestionSourceVerdict }, 0.95, "", false},
		{"injection flagged", func(r *AnalyzeResult) { r.Signals = SignalsJSON(Signals{Injection: f64ptr(InjectionMin)}) }, 0.95, "", false},
		{"injection just below the guard", func(r *AnalyzeResult) { r.Signals = SignalsJSON(Signals{Injection: f64ptr(0.79)}) }, 0.95, "product_bug", true},
		{"no signals recorded", func(r *AnalyzeResult) { r.Signals = "" }, 0.95, "product_bug", true},
		{"abstained", func(r *AnalyzeResult) { r.SuggestedDefectType = "" }, 0.95, "", false},
		{"untriaged default is never applied", func(r *AnalyzeResult) { r.SuggestedDefectType = "to_investigate" }, 0.95, "", false},
		{"insufficient evidence", func(r *AnalyzeResult) { r.SuggestedDefectType = DefectTypeInsufficient }, 0.95, "", false},
		{"system issue", func(r *AnalyzeResult) { r.SuggestedDefectType = "system_issue" }, 0.95, "system_issue", true},
		{"a threshold below the settings floor is refused", nil, 0.50, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := eligibleResult()
			if tc.edit != nil {
				tc.edit(r)
			}
			got, ok := AutoApplyValue(r, tc.min)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
	got, ok := AutoApplyValue(nil, 0.95)
	require.False(t, ok)
	require.Equal(t, "", got)
}

func TestJobDepsAutoApplyTarget(t *testing.T) {
	_, ok := JobDeps{}.AutoApplyTarget(eligibleResult())
	require.False(t, ok, "auto-apply off or paused: nothing is labelled")
	got, ok := JobDeps{AutoApply: &AutoApplyDeps{MinConfidence: 0.95}}.AutoApplyTarget(eligibleResult())
	require.True(t, ok)
	require.Equal(t, "product_bug", got)
	_, ok = JobDeps{AutoApply: &AutoApplyDeps{MinConfidence: 0.99}}.AutoApplyTarget(eligibleResult())
	require.False(t, ok)
}

// Explain judges the stored representative, not a fresh result.
func TestJobDepsAutoApplyTargetRow(t *testing.T) {
	on := JobDeps{AutoApply: &AutoApplyDeps{MinConfidence: 0.95}}
	row := &models.RunResultAnalysis{Engine: models.AnalysisEngineTypeSafe, DecisionStatus: models.DecisionStatusOK,
		SuggestedDefectType: "system_issue", SuggestedDefectTypeConfidence: f64ptr(0.97)}
	got, ok := on.AutoApplyTargetRow(row)
	require.True(t, ok)
	require.Equal(t, "system_issue", got)

	row.Signals = SignalsJSON(Signals{Injection: f64ptr(0.9)})
	_, ok = on.AutoApplyTargetRow(row)
	require.False(t, ok, "a flagged decision never labels")
	row.Signals, row.SuggestionSource = "", models.SuggestionSourceVerdict
	_, ok = on.AutoApplyTargetRow(row)
	require.False(t, ok)
	_, ok = on.AutoApplyTargetRow(nil)
	require.False(t, ok)
	_, ok = JobDeps{}.AutoApplyTargetRow(&models.RunResultAnalysis{Engine: models.AnalysisEngineTypeSafe,
		DecisionStatus: models.DecisionStatusOK, SuggestedDefectType: "product_bug", SuggestedDefectTypeConfidence: f64ptr(0.99)})
	require.False(t, ok, "auto-apply off or paused")
}

func TestNewGateStatus_Arithmetic(t *testing.T) {
	policies := []string{PolicyVersionNoExamples, PolicyVersionWithExamples}
	cases := []struct {
		graded, agreed int
		open           bool
		accuracy       float64
	}{
		{0, 0, false, 0},
		{49, 49, false, 1},     // too few graded
		{50, 48, true, 0.96},   // enough, accurate
		{50, 47, false, 0.94},  // enough, not accurate
		{100, 95, true, 0.95},  // exactly at the accuracy floor
		{100, 94, false, 0.94}, // just below it
	}
	for _, tc := range cases {
		g := NewGateStatus(tc.graded, tc.agreed, 0.95, policies)
		require.Equal(t, tc.open, g.Open, "%d/%d", tc.agreed, tc.graded)
		require.InDelta(t, tc.accuracy, g.Accuracy, 1e-9)
		require.Equal(t, tc.graded, g.Graded)
		require.Equal(t, tc.agreed, g.Agreed)
	}
	g := NewGateStatus(10, 9, 0.9, policies)
	require.Equal(t, policies, g.Policies)
	require.InDelta(t, 0.9, g.MinConfidence, 1e-12)
	require.Equal(t, AutoApplyGateWindowDays, g.WindowDays)
	require.Equal(t, AutoApplyGateMinGraded, g.MinGraded)
	require.InDelta(t, AutoApplyGateMinAccuracy, g.MinAccuracy, 1e-12)
}

func TestValidateAutoApplyMinConfidence(t *testing.T) {
	for _, ok := range []int{80, 95, 99} {
		require.NoError(t, models.ValidateAutoApplyMinConfidence(ok), ok)
	}
	for _, bad := range []int{0, 79, 100} {
		require.Error(t, models.ValidateAutoApplyMinConfidence(bad), bad)
	}
}
