package compare

import (
	"bytes"
	"strings"
	"testing"
)

func s(v string) *string { return &v }

func jobFixture() ([]Result, map[string][]Analysis, []Job) {
	results := []Result{
		{ID: "r1", Name: "A", Status: "FAIL", ErrorMessage: "one"},
		{ID: "r2", Name: "B", Status: "FAIL", ErrorMessage: "two"},
	}
	analyses := map[string][]Analysis{
		"r1": {
			{Version: 1, Engine: "typesafe", ModelName: "jev", PolicyVersion: "fa-verdict-v4", Verdict: "flaky_test", Confidence: "high", DecisionStatus: "ok", JobID: s("aaaaaaaa-1111")},
			{Version: 2, Engine: "typesafe", ModelName: "jev", PolicyVersion: "fa-verdict-v4", Verdict: "flaky_test", Confidence: "high", DecisionStatus: "ok", JobID: s("bbbbbbbb-2222")},
			{Version: 3, Engine: "generative", ModelName: "minimax", Verdict: "product_bug", Confidence: "high", DecisionStatus: "ok"}, // a single re-analyze
		},
		"r2": {
			{Version: 1, Engine: "typesafe", ModelName: "jev", PolicyVersion: "fa-verdict-v4", Verdict: "unknown", Confidence: "low", DecisionStatus: "failed", ErrorCategory: "timeout", JobID: s("aaaaaaaa-1111"), TypeSafeInputTokens: 50},
			// The second job's takeover: TypeSafe was unsure, the LLM decided.
			{Version: 2, Engine: "generative", ModelName: "minimax", Verdict: "environment", Confidence: "medium", DecisionStatus: "ok", JobID: s("bbbbbbbb-2222")},
		},
	}
	jobs := []Job{ // newest first, as the API lists them
		{ID: "bbbbbbbb-2222", PipelineLabel: "TypeSafe jev, decisions only, minimax below 90%"},
		{ID: "aaaaaaaa-1111", PipelineLabel: "TypeSafe jev, no LLM"},
	}
	return results, analyses, jobs
}

func TestPivot_FailedAttemptWithModelIsAFailedCell(t *testing.T) {
	results, analyses, _ := jobFixture()
	rep := Pivot(results, analyses)
	cell := rep.Rows[1].Cells["typesafe/jev@fa-verdict-v4"]
	if cell == nil || !cell.Failed || cell.ErrorCategory != "timeout" {
		t.Fatalf("a failed TypeSafe attempt is a failed cell of its column: %+v", cell)
	}
	if rep.FailedCalls != 0 {
		t.Fatalf("an attributable failure is not an unplaced failed call: %d", rep.FailedCalls)
	}
	if rep.Rows[1].Disagree {
		t.Fatal("a failed cell has no verdict to disagree with")
	}
	sum := Summarize(rep)
	for _, cs := range sum.Columns {
		if cs.Key == "typesafe/jev@fa-verdict-v4" {
			if cs.Analyzed != 1 || cs.Failed != 1 || cs.Tokens != 50 {
				t.Fatalf("failed rows are counted apart from analyzed ones: %+v", cs)
			}
		}
	}
	for _, p := range sum.Pairs {
		if p.Compared != 1 {
			t.Fatalf("a failed cell is not compared: %+v", p)
		}
	}
}

func TestFailedAttempt_LegacyRowsWithoutStatus(t *testing.T) {
	if !failedAttempt(Analysis{Engine: "generative"}) {
		t.Fatal("a generative row with no model name and no status is a legacy failed call")
	}
	if failedAttempt(Analysis{Engine: "generative", ModelName: "m"}) {
		t.Fatal("a named model with no status is an answer")
	}
	if !failedAttempt(Analysis{Engine: "typesafe", ModelName: "jev", DecisionStatus: "failed"}) {
		t.Fatal("decision_status wins")
	}
}

func TestPivotWith_OneJob(t *testing.T) {
	results, analyses, jobs := jobFixture()
	rep := PivotWith(results, analyses, Options{Job: "bbbbbbbb-2222", Jobs: jobs})
	if rep.JobLabel != "TypeSafe jev, decisions only, minimax below 90%" {
		t.Fatalf("label: %q", rep.JobLabel)
	}
	if rep.Excluded != 3 {
		t.Fatalf("the other job's rows and the lone re-analyze are left out: %d", rep.Excluded)
	}
	if c := rep.Rows[0].Cells["typesafe/jev@fa-verdict-v4"]; c == nil || c.Version != 2 {
		t.Fatalf("only this job's version: %+v", c)
	}
	if c := rep.Rows[1].Cells["generative/minimax"]; c == nil || c.Verdict != "environment" {
		t.Fatalf("the takeover row is part of the job: %+v", c)
	}
	if c := rep.Rows[0].Cells["generative/minimax"]; c != nil {
		t.Fatalf("the re-analyze outside the job is not: %+v", c)
	}
}

func TestPivotWith_ByJob(t *testing.T) {
	results, analyses, jobs := jobFixture()
	rep := PivotWith(results, analyses, Options{ByJob: true, Jobs: jobs})
	if len(rep.Columns) != 2 || rep.Columns[0].Key != "job aaaaaaaa" || rep.Columns[1].Key != "job bbbbbbbb" {
		t.Fatalf("one column per job, oldest first: %+v", rep.Columns)
	}
	if rep.Columns[1].Label != "TypeSafe jev, decisions only, minimax below 90%" {
		t.Fatalf("labelled by pipeline: %+v", rep.Columns[1])
	}
	if rep.Excluded != 1 {
		t.Fatalf("the analysis made outside a job is left out: %d", rep.Excluded)
	}
	if c := rep.Rows[1].Cells["job bbbbbbbb"]; c == nil || c.Verdict != "environment" {
		t.Fatalf("a job's column holds whichever engine decided: %+v", c)
	}
	if c := rep.Rows[1].Cells["job aaaaaaaa"]; c == nil || !c.Failed {
		t.Fatalf("the first job's failure stays in its column: %+v", c)
	}

	var out bytes.Buffer
	Render(&out, rep, Summarize(rep))
	text := out.String()
	for _, want := range []string{"JOB BBBBBBBB = bbbbbbbb-2222 — TypeSafe jev, decisions only, minimax below 90%", "FAILED (timeout)", "1 failed (no decision; not graded)"} {
		if !strings.Contains(text, want) {
			t.Fatalf("render is missing %q:\n%s", want, text)
		}
	}
}

func TestResolveJob(t *testing.T) {
	_, _, jobs := jobFixture()
	if j, err := ResolveJob(jobs, "aaaa"); err != nil || j.ID != "aaaaaaaa-1111" {
		t.Fatalf("a unique prefix resolves: %+v %v", j, err)
	}
	if j, err := ResolveJob(jobs, "bbbbbbbb-2222"); err != nil || j.ID != "bbbbbbbb-2222" {
		t.Fatalf("a full id resolves: %+v %v", j, err)
	}
	if _, err := ResolveJob(jobs, "zzz"); err == nil {
		t.Fatal("an unknown job is an error")
	}
	both := append(jobs, Job{ID: "aaaaaaaa-9999"})
	if _, err := ResolveJob(both, "aaaaaaaa"); err == nil || !strings.Contains(err.Error(), "matches 2") {
		t.Fatalf("an ambiguous prefix is an error: %v", err)
	}
}
func TestPivotWith_ByJobShowsTimingAndRateLimits(t *testing.T) {
	results, analyses, jobs := jobFixture()
	jobs[0].Outcomes = &JobOutcomes{DecisionMsAvg: 420, DecisionMsP50: 410, DecisionMsMax: 490,
		LLMMsAvg: 10300, LLMMsP50: 9800, LLMMsMax: 15200, RateLimitHits: 2}
	jobs[0].RateLimitHits = 3 // the job row counted a 429 the outcomes did not: the larger count wins
	rep := PivotWith(results, analyses, Options{ByJob: true, Jobs: jobs})

	var out bytes.Buffer
	Render(&out, rep, Summarize(rep))
	text := out.String()
	for _, want := range []string{
		"decision avg 420 ms · p50 410 ms · max 490 ms; LLM avg 10.3 s · p50 9.8 s · max 15.2 s; 3 rate-limit hit(s)",
		"timing not recorded", // the older job has no telemetry
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("render is missing %q:\n%s", want, text)
		}
	}

	one := PivotWith(results, analyses, Options{Job: "bbbbbbbb-2222", Jobs: jobs})
	if one.JobTiming == nil || one.JobTiming.RateLimitHits != 3 {
		t.Fatalf("--job carries its job's timing: %+v", one.JobTiming)
	}

	parsed, err := ParseJobs([]byte(`[{"id":"j1","rate_limit_hits":1,"outcomes":{"groups":3,"decision_ms_avg":400,"decision_ms_p50":390,"decision_ms_max":450,"llm_ms_avg":0,"llm_ms_p50":0,"llm_ms_max":0,"rate_limit_hits":1}}]`))
	if err != nil || parsed[0].Outcomes == nil || parsed[0].Outcomes.DecisionMsP50 != 390 || parsed[0].RateLimitHits != 1 {
		t.Fatalf("parse: %+v %v", parsed, err)
	}
	if got := timingText(jobTiming(parsed[0])); got != "decision avg 400 ms · p50 390 ms · max 450 ms; no LLM time; 1 rate-limit hit(s)" {
		t.Fatalf("a TypeSafe-only job has no LLM time: %q", got)
	}
}

func TestPivotWith_ByJobShowsCallTimeoutsAndHedges(t *testing.T) {
	results, analyses, jobs := jobFixture()
	jobs[0].Outcomes = &JobOutcomes{DecisionMsAvg: 420, DecisionMsP50: 410, DecisionMsMax: 490,
		LLMMsAvg: 10300, LLMMsP50: 9800, LLMMsMax: 15200, CallTimeouts: 1, HedgesFired: 3, HedgesWon: 2}
	jobs[0].CallTimeouts = 2 // the row counted more than the outcomes: the larger count wins
	rep := PivotWith(results, analyses, Options{ByJob: true, Jobs: jobs})

	var out bytes.Buffer
	Render(&out, rep, Summarize(rep))
	want := "decision avg 420 ms · p50 410 ms · max 490 ms; LLM avg 10.3 s · p50 9.8 s · max 15.2 s; 0 rate-limit hit(s); 2 call timeout(s); 3 hedge(s) fired, 2 won"
	if !strings.Contains(out.String(), want) {
		t.Fatalf("render is missing %q:\n%s", want, out.String())
	}

	parsed, err := ParseJobs([]byte(`[{"id":"j1","call_timeouts":1,"hedges_fired":1,"hedges_won":0}]`))
	if err != nil {
		t.Fatal(err)
	}
	if got := timingText(jobTiming(parsed[0])); got != "no decision time; no LLM time; 0 rate-limit hit(s); 1 call timeout(s); 1 hedge(s) fired, 0 won" {
		t.Fatalf("latency counts on the job row alone are shown: %q", got)
	}
	if got := timingText(jobTiming(Job{ID: "old"})); got != "timing not recorded" {
		t.Fatalf("an old job: %q", got)
	}
}
