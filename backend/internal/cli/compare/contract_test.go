package compare

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSentences(t *testing.T) {
	for in, want := range map[string]int{
		"":                            0,
		"One.":                        1,
		"Retry after 0.5 s.":          1,
		"Timed out. Check the proxy.": 2,
		"no terminator":               1,
		"Why? Because.":               2,
	} {
		if got := sentences(in); got != want {
			t.Fatalf("sentences(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestNextActions_ListItemsCountAsActions(t *testing.T) {
	for in, want := range map[string]int{
		"1. Restart the runner\n2. Re-run the job": 2,
		"- Check the proxy log":                    1,
		"Check the proxy log and re-run.":          1,
		"Check the log. Then re-run.":              2,
	} {
		if got := nextActions(in); got != want {
			t.Fatalf("nextActions(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestEvidenceItems_CountsQuotedSpansNotApostrophes(t *testing.T) {
	in := `The log says "ECONNRESET" and the stack shows ` + "`api.ts:42`" + `; the test's own step didn't fail.`
	if got := evidenceItems(in); got != 2 {
		t.Fatalf("got %d", got)
	}
	if got := evidenceItems(`'a' then 'b' then “c”`); got != 3 {
		t.Fatalf("single and curly quotes count: %d", got)
	}
}

func TestCheckExplanations_RepresentativesOnly(t *testing.T) {
	good := &Cell{Verdict: "environment", NarrativeStatus: "ok",
		Summary:    "The page timed out waiting for the proxy.",
		NextAction: "Check the proxy health before re-running.",
		Rationale:  `The log shows "ECONNRESET" at the gateway.`}
	loose := &Cell{Verdict: "product_bug", NarrativeStatus: "ok",
		Summary:    "Checkout failed. The API returned 500.",
		NextAction: "1. Open the API log\n2. Re-run the test",
		Rationale:  `"500" in the error, "NullPointerException" in the stack and ` + "`OrderService.java:88`" + ` in the trace.`}
	clone := *good
	clone.Clone = true
	key := "typesafe/jev@fa-verdict-v5"
	rep := Report{
		Columns: []Column{{Key: key}, {Key: "generative/minimax"}},
		Rows: []Row{
			{Cells: map[string]*Cell{key: good}},
			{Cells: map[string]*Cell{key: loose}},
			{Cells: map[string]*Cell{key: &clone}},
			{Cells: map[string]*Cell{key: {Failed: true, Summary: "analysis failed: timeout"}}},
			{Cells: map[string]*Cell{key: {NarrativeStatus: "skipped", Summary: "No explanation: explanations are switched off."}}},
		},
	}
	stats := CheckExplanations(rep)
	ts := stats[0]
	if ts.Explained != 2 || ts.OneSummary != 1 || ts.OneNextAction != 1 || ts.EvidenceMax2 != 1 || ts.AllThree != 1 {
		t.Fatalf("counts: %+v", ts)
	}
	lg := utf8.RuneCountInString(good.Summary + good.NextAction + good.Rationale)
	ll := utf8.RuneCountInString(loose.Summary + loose.NextAction + loose.Rationale)
	if ts.LengthP50 != min(lg, ll) || ts.LengthP90 != max(lg, ll) || ts.LengthMax != max(lg, ll) {
		t.Fatalf("lengths: %+v (good %d, loose %d)", ts, lg, ll)
	}
	if stats[1].Explained != 0 {
		t.Fatalf("an empty column: %+v", stats[1])
	}

	var out bytes.Buffer
	RenderContract(&out, stats)
	for _, want := range []string{key + ": 2 explanations — one-sentence summary 50%", "generative/minimax: no explanations"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("render is missing %q:\n%s", want, out.String())
		}
	}
}

func TestPivot_CarriesExplanationCloneAndHistory(t *testing.T) {
	list, err := ParseAnalyses([]byte(`[{"run_result_id":"r1","version":1,"engine":"generative","model_name":"m","verdict":"product_bug",
		"decision_status":"ok","narrative_status":"ok","summary":"S.","next_action":"N.","rationale":"R",
		"source_analysis_id":"rep-1","history_available":true}]`))
	if err != nil {
		t.Fatal(err)
	}
	rep := Pivot([]Result{{ID: "r1"}}, map[string][]Analysis{"r1": list})
	c := rep.Rows[0].Cells["generative/m"]
	if c == nil || c.Summary != "S." || c.NextAction != "N." || c.Rationale != "R" || !c.Clone || !c.History {
		t.Fatalf("cell: %+v", c)
	}
}
