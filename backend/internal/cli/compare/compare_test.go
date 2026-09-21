package compare

import (
	"testing"
)

func f(v float64) *float64 { return &v }

func TestNormalize_CollapsesDigitsAndWhitespace(t *testing.T) {
	a := Normalize("AssertionError: expected order total 1499.00 to equal cart line total 1499.99 (order 55504)")
	b := Normalize("AssertionError: expected order total 1499.00 to equal   cart line total 1499.99 (order 50007)")
	if a != b {
		t.Fatalf("same template must normalize equal:\n%s\n%s", a, b)
	}
	if a == Normalize("POST /api/v1/profile/preferences returned 500 Internal Server Error (request req-85504436)") {
		t.Fatal("different templates must not collide")
	}
	ts := Normalize("page.goto: Timeout 45000ms exceeded navigating to https://x/login (at 2026-09-21T15:04:05Z)")
	if ts != Normalize("page.goto: Timeout 45000ms exceeded navigating to https://x/login (at 2026-07-01T00:00:00Z)") {
		t.Fatal("timestamps must normalize away")
	}
}

func sampleResults() []Result {
	return []Result{
		{ID: "r1", Name: "Checkout #1", Status: "FAIL", ErrorMessage: "AssertionError: expected order total 1499.00 to equal cart line total 1499.99 (order 55504)"},
		{ID: "r2", Name: "Profile #2", Status: "ERROR", ErrorMessage: "POST /api/v1/profile/preferences returned 500 Internal Server Error (request req-81234567)"},
		{ID: "r3", Name: "Toast #3", Status: "FAIL", ErrorMessage: "TimeoutError: waiting for selector .toast-success failed: timeout 10000ms exceeded"},
	}
}

func sampleAnalyses() map[string][]Analysis {
	return map[string][]Analysis{
		"r1": {
			{Version: 1, Engine: "typesafe", ModelName: "jev-1.13", Verdict: "product_bug", Confidence: "high", ConfidenceScore: f(0.91), SuggestedDefectType: "product_bug", TypeSafeInputTokens: 800},
			{Version: 2, Engine: "generative", ModelName: "openai/gpt-4o", Verdict: "flaky_test", Confidence: "medium", SuggestedDefectType: "automation_bug", TokenUsagePrompt: 1200, TokenUsageCompletion: 150},
			{Version: 3, Engine: "generative", ModelName: "openai/gpt-4o", Verdict: "product_bug", Confidence: "high", SuggestedDefectType: "product_bug", TokenUsagePrompt: 1300, TokenUsageCompletion: 160},
		},
		"r2": {
			{Version: 1, Engine: "typesafe", ModelName: "jev-1.13", Verdict: "environment", Confidence: "low", ConfidenceScore: f(0.41), SuggestedDefectType: "insufficient_evidence", TypeSafeInputTokens: 700},
			{Version: 2, Engine: "generative", ModelName: "openai/gpt-4o", Verdict: "product_bug", Confidence: "high", SuggestedDefectType: "product_bug", TokenUsagePrompt: 1000, TokenUsageCompletion: 100},
		},
		"r3": {
			{Version: 1, Engine: "typesafe", ModelName: "jev-1.13", Verdict: "flaky_test", Confidence: "high", ConfidenceScore: f(0.88), SuggestedDefectType: "automation_bug", TypeSafeInputTokens: 600},
		},
	}
}

func TestPivot_LatestVersionPerEngineModelColumn(t *testing.T) {
	rep := Pivot(sampleResults(), sampleAnalyses())
	if len(rep.Columns) != 2 || rep.Columns[0].Key != "typesafe/jev-1.13" || rep.Columns[1].Key != "generative/openai/gpt-4o" {
		t.Fatalf("columns: %+v", rep.Columns)
	}
	if len(rep.Rows) != 3 {
		t.Fatalf("rows: %d", len(rep.Rows))
	}
	r1 := rep.Rows[0]
	gen := r1.Cells["generative/openai/gpt-4o"]
	if gen == nil || gen.Version != 3 || gen.Verdict != "product_bug" {
		t.Fatalf("r1 generative cell must be the latest version: %+v", gen)
	}
	if r1.Disagree {
		t.Fatal("r1 columns agree on the verdict")
	}
	r2 := rep.Rows[1]
	if !r2.Disagree {
		t.Fatal("r2 verdicts differ (environment vs product_bug)")
	}
	r3 := rep.Rows[2]
	if r3.Cells["generative/openai/gpt-4o"] != nil || r3.Disagree {
		t.Fatal("a row with a single column has no disagreement")
	}
}

func TestGrade_MatchesTemplatesAndScoresPerColumn(t *testing.T) {
	rep := Pivot(sampleResults(), sampleAnalyses())
	gt := []GroundTruth{
		{TemplateKey: "checkout-total-mismatch", SampleMessage: "AssertionError: expected order total 1499.00 to equal cart line total 1499.99 (order 50001)", ExpectedVerdict: "product_bug", ExpectedDefectType: "product_bug"},
		{TemplateKey: "profile-preferences-500", SampleMessage: "POST /api/v1/profile/preferences returned 500 Internal Server Error (request req-80000001)", ExpectedVerdict: "product_bug", ExpectedDefectType: "product_bug"},
	}
	Grade(&rep, gt)
	if rep.Rows[0].Template != "checkout-total-mismatch" || rep.Rows[1].Template != "profile-preferences-500" {
		t.Fatalf("templates: %q %q", rep.Rows[0].Template, rep.Rows[1].Template)
	}
	if rep.Rows[2].Template != "" {
		t.Fatal("unmatched rows stay ungraded")
	}
	s := Summarize(rep)
	ts := s.Columns[0]
	if ts.Graded != 2 || ts.VerdictGraded != 2 || ts.VerdictCorrect != 1 || ts.DefectCorrect != 1 {
		t.Fatalf("typesafe grading: %+v", ts)
	}
	if ts.ByConfidence["high"].Graded != 1 || ts.ByConfidence["high"].VerdictCorrect != 1 || ts.ByConfidence["low"].VerdictCorrect != 0 {
		t.Fatalf("typesafe buckets: %+v", ts.ByConfidence)
	}
	gen := s.Columns[1]
	if gen.Graded != 2 || gen.VerdictCorrect != 2 || gen.DefectCorrect != 2 {
		t.Fatalf("generative grading: %+v", gen)
	}
	if s.Ungraded != 1 {
		t.Fatalf("ungraded rows: %d", s.Ungraded)
	}
}

func TestGrade_EmptyExpectedVerdictScoresDefectTypeOnly(t *testing.T) {
	rep := Pivot(sampleResults(), sampleAnalyses())
	gt := []GroundTruth{
		{TemplateKey: "checkout-total-mismatch", SampleMessage: sampleResults()[0].ErrorMessage, ExpectedVerdict: "product_bug", ExpectedDefectType: "product_bug"},
		// A stale locator has no verdict in the taxonomy: the key leaves the verdict
		// empty and the template is graded on its defect type alone.
		{TemplateKey: "toast-wait-timeout", SampleMessage: sampleResults()[2].ErrorMessage, ExpectedVerdict: "", ExpectedDefectType: "automation_bug"},
	}
	Grade(&rep, gt)
	s := Summarize(rep)
	ts := s.Columns[0]
	if ts.Graded != 2 || ts.DefectCorrect != 2 {
		t.Fatalf("both matched rows are graded on defect type: %+v", ts)
	}
	if ts.VerdictGraded != 1 || ts.VerdictCorrect != 1 {
		t.Fatalf("only the row with an expected verdict counts toward verdict accuracy: %+v", ts)
	}
	high := ts.ByConfidence["high"]
	if high.Graded != 2 || high.VerdictGraded != 1 || high.VerdictCorrect != 1 || high.DefectCorrect != 2 {
		t.Fatalf("bucket keeps the two denominators apart: %+v", high)
	}
	if s.Ungraded != 1 {
		t.Fatalf("ungraded rows: %d", s.Ungraded)
	}
}

func TestPivot_PolicyVersionSplitsTypeSafeColumns(t *testing.T) {
	results := []Result{{ID: "r1", Name: "A", Status: "FAIL", ErrorMessage: "boom"}}
	analyses := map[string][]Analysis{"r1": {
		{Version: 1, Engine: "typesafe", ModelName: "jev-1.13", Verdict: "unknown", Confidence: "medium", PolicyVersion: "fa-verdict-v1"},
		{Version: 2, Engine: "generative", ModelName: "gpt", Verdict: "product_bug", Confidence: "high"},
		{Version: 3, Engine: "typesafe", ModelName: "jev-1.13", Verdict: "product_bug", Confidence: "high", PolicyVersion: "fa-verdict-v2"},
	}}
	rep := Pivot(results, analyses)
	keys := []string{}
	for _, c := range rep.Columns {
		keys = append(keys, c.Key)
	}
	want := []string{"typesafe/jev-1.13@fa-verdict-v1", "typesafe/jev-1.13@fa-verdict-v2", "generative/gpt"}
	if len(keys) != 3 || keys[0] != want[0] || keys[1] != want[1] || keys[2] != want[2] {
		t.Fatalf("columns: %v, want %v", keys, want)
	}
	if rep.Rows[0].Cells[want[1]].Verdict != "product_bug" || rep.Rows[0].Cells[want[0]].Verdict != "unknown" {
		t.Fatalf("each policy keeps its own cell: %+v", rep.Rows[0].Cells)
	}
}

func TestSummarize_AgreementAbstentionsAndTokens(t *testing.T) {
	rep := Pivot(sampleResults(), sampleAnalyses())
	s := Summarize(rep)
	if s.Rows != 3 {
		t.Fatalf("rows: %d", s.Rows)
	}
	if len(s.Pairs) != 1 {
		t.Fatalf("pairs: %+v", s.Pairs)
	}
	p := s.Pairs[0]
	if p.Compared != 2 || p.VerdictAgree != 1 || p.DefectAgree != 1 {
		t.Fatalf("pair agreement: %+v", p)
	}
	ts := s.Columns[0]
	if ts.Analyzed != 3 || ts.Abstained != 1 || ts.Tokens != 2100 || ts.Verdicts["flaky_test"] != 1 {
		t.Fatalf("typesafe stats: %+v", ts)
	}
	if ts.MeanScore < 0.73 || ts.MeanScore > 0.74 {
		t.Fatalf("mean score: %v", ts.MeanScore)
	}
	gen := s.Columns[1]
	if gen.Analyzed != 2 || gen.Tokens != 2560 || gen.Abstained != 0 {
		t.Fatalf("generative stats: %+v", gen)
	}
}

func TestParseRun_KeepsFailingRowsOnly(t *testing.T) {
	raw := []byte(`{"id":"run1","name":"Nightly","run_results":[
		{"id":"a","status":"PASS","error_message":"","test_case":{"name":"ok"}},
		{"id":"b","status":"FAIL","error_message":"boom","test_case":{"name":"Checkout #1"}},
		{"id":"c","status":"ERROR","error_message":"crash","test_case":null,"test_case_id":null}
	]}`)
	name, rows, err := ParseRun(raw)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Nightly" || len(rows) != 2 || rows[0].ID != "b" || rows[0].Name != "Checkout #1" || rows[1].Name != "(deleted test case)" {
		t.Fatalf("parsed: %q %+v", name, rows)
	}
}

func TestParseAnalyses_ReadsHistory(t *testing.T) {
	raw := []byte(`[{"run_result_id":"b","version":2,"engine":"typesafe","model_name":"jev-1.13","verdict":"product_bug","confidence":"high","confidence_score":0.9,"suggested_defect_type":"product_bug","typesafe_input_tokens":12},
	{"run_result_id":"b","version":1,"engine":"generative","model_name":"gpt","verdict":"unknown","confidence":"low","suggested_defect_type":"","token_usage_prompt":5,"token_usage_completion":6}]`)
	list, err := ParseAnalyses(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Version != 2 || *list[0].ConfidenceScore != 0.9 || list[1].TokenUsageCompletion != 6 {
		t.Fatalf("parsed: %+v", list)
	}
}

func TestParseGroundTruth_ReadsSeedStatusOrManifest(t *testing.T) {
	status := []byte(`{"loaded":true,"latest_run_id":"x","ground_truth":[{"template_key":"k","sample_message":"m 1","expected_verdict":"product_bug","expected_defect_type":"product_bug"}]}`)
	gt, err := ParseGroundTruth(status)
	if err != nil || len(gt) != 1 || gt[0].TemplateKey != "k" || gt[0].ExpectedDefectType != "product_bug" {
		t.Fatalf("status shape: %v %+v", err, gt)
	}
	bare := []byte(`[{"template_key":"k2","sample_message":"m","expected_verdict":"flaky_test","expected_defect_type":"automation_bug"}]`)
	gt, err = ParseGroundTruth(bare)
	if err != nil || len(gt) != 1 || gt[0].TemplateKey != "k2" {
		t.Fatalf("bare array shape: %v %+v", err, gt)
	}
}

func TestGradingNote_OnlyWhenAnAnswerKeyMatchedNothing(t *testing.T) {
	rep := Pivot(sampleResults(), sampleAnalyses())
	s := Summarize(rep)
	if GradingNote(s, false) != "" {
		t.Fatal("no answer key, no note")
	}
	if GradingNote(s, true) != NoMatchNote {
		t.Fatalf("an answer key that matched nothing must say so: %q", GradingNote(s, true))
	}
	Grade(&rep, []GroundTruth{{TemplateKey: "k", SampleMessage: sampleResults()[0].ErrorMessage, ExpectedVerdict: "product_bug", ExpectedDefectType: "product_bug"}})
	if GradingNote(Summarize(rep), true) != "" {
		t.Fatal("graded rows need no note")
	}
}
