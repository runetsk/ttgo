package compare

import (
	"bytes"
	"strings"
	"testing"
)

func modesFixture() ([]Result, map[string][]Analysis, []GroundTruth) {
	results := []Result{
		{ID: "r1", Name: "A", Status: "FAIL", ErrorMessage: "timeout waiting for #1"},
		{ID: "r2", Name: "B", Status: "FAIL", ErrorMessage: "expected 200 got 500"},
	}
	analyses := map[string][]Analysis{
		"r1": {
			{Version: 1, Engine: "typesafe", ModelName: "jev", PolicyVersion: "fa-verdict-v5", Verdict: "environment", Confidence: "high", SuggestedDefectType: "", DecisionStatus: "ok", HistoryAvailable: true},
			{Version: 2, Engine: "generative", ModelName: "minimax", Verdict: "environment", Confidence: "high", SuggestedDefectType: "system_issue", DecisionStatus: "ok", HistoryAvailable: true},
		},
		"r2": {
			{Version: 1, Engine: "typesafe", ModelName: "jev", PolicyVersion: "fa-verdict-v5", Verdict: "product_bug", Confidence: "high", SuggestedDefectType: "product_bug", SuggestedDefectTypeConfidence: f(0.9), DecisionStatus: "ok"},
			{Version: 2, Engine: "generative", ModelName: "minimax", Verdict: "flaky_test", Confidence: "medium", SuggestedDefectType: "automation_bug", DecisionStatus: "ok"},
		},
	}
	gt := []GroundTruth{
		{TemplateKey: "nav-timeout", SampleMessage: "timeout waiting for #7", ExpectedVerdict: "environment", ExpectedDefectType: "system_issue"},
		{TemplateKey: "api-500", SampleMessage: "expected 200 got 500", ExpectedVerdict: "product_bug", ExpectedDefectType: "product_bug"},
	}
	return results, analyses, gt
}

func column(t *testing.T, s Summary, key string) ColumnStats {
	t.Helper()
	for _, cs := range s.Columns {
		if cs.Key == key {
			return cs
		}
	}
	t.Fatalf("no column %q in %+v", key, s.Columns)
	return ColumnStats{}
}

func TestPivotWith_MappingModeRegradesEveryEngineOnItsVerdict(t *testing.T) {
	results, analyses, gt := modesFixture()
	native := PivotWith(results, analyses, Options{})
	Grade(&native, gt)
	mapped := PivotWith(results, analyses, Options{DefectTypeMode: DefectTypeModeMapping})
	Grade(&mapped, gt)

	if c := native.Rows[0].Cells["typesafe/jev@fa-verdict-v5"]; c.DefectType != "" {
		t.Fatalf("native keeps the stored (withheld) suggestion: %+v", c)
	}
	if c := mapped.Rows[0].Cells["typesafe/jev@fa-verdict-v5"]; c.DefectType != "system_issue" || c.DefectScore != nil {
		t.Fatalf("mapping derives the defect type from the verdict and drops the question's score: %+v", c)
	}
	if mapped.DefectTypeMode != DefectTypeModeMapping || native.DefectTypeMode != "" {
		t.Fatalf("the report names its mode: %q / %q", mapped.DefectTypeMode, native.DefectTypeMode)
	}
	if got := column(t, Summarize(native), "typesafe/jev@fa-verdict-v5").DefectCorrect; got != 1 {
		t.Fatalf("native: the withheld suggestion on r1 is wrong, r2 right: %d", got)
	}
	if got := column(t, Summarize(mapped), "typesafe/jev@fa-verdict-v5").DefectCorrect; got != 2 {
		t.Fatalf("mapping: both verdicts map to the expected type: %d", got)
	}

	var out bytes.Buffer
	Render(&out, mapped, Summarize(mapped))
	if !strings.Contains(out.String(), "--defect-type-mode mapping") {
		t.Fatalf("the rendered report says how defect types were graded:\n%s", out.String())
	}
}

func TestParseDefectTypeMode(t *testing.T) {
	for in, want := range map[string]string{"": "native", "native": "native", "Mapping": "mapping"} {
		if got, err := ParseDefectTypeMode(in); err != nil || got != want {
			t.Fatalf("%q: %q %v", in, got, err)
		}
	}
	if _, err := ParseDefectTypeMode("verdict"); err == nil {
		t.Fatal("an unknown mode is an error")
	}
}

func TestSummarize_SplitsGradingByHistory(t *testing.T) {
	results, analyses, gt := modesFixture()
	rep := Pivot(results, analyses)
	Grade(&rep, gt)
	cs := column(t, Summarize(rep), "generative/minimax")
	// r1 had history and is right; r2 had none and is wrong (flaky_test vs product_bug).
	if cs.WithHistory != (HistoryStats{Graded: 1, VerdictGraded: 1, VerdictCorrect: 1, DefectCorrect: 1}) {
		t.Fatalf("with history: %+v", cs.WithHistory)
	}
	if cs.WithoutHistory != (HistoryStats{Graded: 1, VerdictGraded: 1, VerdictCorrect: 0, DefectCorrect: 0}) {
		t.Fatalf("without history: %+v", cs.WithoutHistory)
	}
	var out bytes.Buffer
	Render(&out, rep, Summarize(rep))
	want := "with history: verdict 100% of 1, defect type 100% of 1 · without: verdict 0% of 1, defect type 0% of 1"
	if !strings.Contains(out.String(), want) {
		t.Fatalf("render is missing %q:\n%s", want, out.String())
	}
}
