package failureanalysis

import (
	"strings"
	"testing"
	"ttgo/pkg/tracker/models"
)

// Real models drift to invented labels ("environment_or_infrastructure",
// "flaky") or borrow the defect-type vocabulary the historical "human:" lines
// inject ("automation_bug") unless the template spells out the allowed values —
// observed live: 47/63 analyses fell back to unknown before the enum line.
func TestDefaultPromptTemplateEnumeratesVerdictAndConfidenceVocabulary(t *testing.T) {
	for v := range models.ValidVerdicts {
		if !strings.Contains(DefaultPromptTemplate, v) {
			t.Errorf("default template does not list verdict %q", v)
		}
	}
	for c := range models.ValidConfidences {
		if !strings.Contains(DefaultPromptTemplate, c) {
			t.Errorf("default template does not list confidence %q", c)
		}
	}
}

func TestBuildPromptIncludesAllSectionsForSmallInput(t *testing.T) {
	in := PromptInput{
		Template:     DefaultPromptTemplate,
		TestName:     "Login",
		Categories:   "Smoke",
		Env:          "staging",
		Browser:      "chromium",
		OS:           "linux",
		AppVersion:   "1.2.3",
		Steps:        []PromptStep{{Order: 1, Action: "open login", Expected: "form visible"}},
		FailureType:  "assertion",
		ErrorMessage: "expected 401, got 500",
		StackTrace:   "line1\nline2",
		LogText:      "log tail",
	}
	got, meta, err := BuildPrompt(in)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	for _, want := range []string{"Login", "Smoke", "chromium", "line1", "log tail", "expected 401"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt missing %q\nfull:\n%s", want, got)
		}
	}
	if meta.TruncationPrefix != "" {
		t.Errorf("expected empty truncation prefix, got %q", meta.TruncationPrefix)
	}
}

func TestBuildPromptDropsLogFirstWhenTooLong(t *testing.T) {
	// StackTrace + LogText alone can't exceed cap once headN/tailN cut them
	// to 4000/2000. Use a giant step to push total past PromptCharCap so the
	// drop order is exercised; first drop must be the log.
	in := PromptInput{
		Template:     DefaultPromptTemplate,
		TestName:     "x",
		ErrorMessage: "x",
		FailureType:  "x",
		StackTrace:   strings.Repeat("s", 3000),
		LogText:      strings.Repeat("L", 30000),
		Steps:        []PromptStep{{Order: 1, Action: strings.Repeat("a", 25000), Expected: "e"}},
	}
	got, meta, err := BuildPrompt(in)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if len(got) > PromptCharCap {
		t.Errorf("prompt length %d exceeds cap %d", len(got), PromptCharCap)
	}
	if !strings.Contains(meta.TruncationPrefix, "no logs") {
		t.Errorf("expected truncation prefix to mention 'no logs', got %q", meta.TruncationPrefix)
	}
}

func TestStackTraceTruncatedToFirst4000Chars(t *testing.T) {
	// Build a trace with a distinguishable tail so we can prove the head was
	// kept and the tail was dropped, independent of other template noise.
	trace := strings.Repeat("S", 4000) + "TAIL_MARKER_DROPPED"
	in := PromptInput{
		Template:     DefaultPromptTemplate,
		TestName:     "x",
		ErrorMessage: "x",
		FailureType:  "x",
		StackTrace:   trace,
	}
	got, _, err := BuildPrompt(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, strings.Repeat("S", 4000)) {
		t.Errorf("expected prompt to retain the first 4000 S chars of the stack trace")
	}
	if strings.Contains(got, "TAIL_MARKER_DROPPED") {
		t.Errorf("expected anything past the first 4000 chars of the stack trace to be dropped")
	}
}

func TestLogTextKeepsLast2000Chars(t *testing.T) {
	// Prefix a unique marker to the front of the log so we can prove the tail
	// was kept (marker must be dropped) without depending on any char-class.
	logText := "HEAD_MARKER_DROPPED" + strings.Repeat("a", 1000) + strings.Repeat("z", 3000)
	in := PromptInput{
		Template:     DefaultPromptTemplate,
		TestName:     "x",
		ErrorMessage: "x",
		FailureType:  "x",
		LogText:      logText,
	}
	got, _, err := BuildPrompt(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, strings.Repeat("z", 2000)) {
		t.Errorf("expected tail 2000 z chars in prompt")
	}
	if strings.Contains(got, "HEAD_MARKER_DROPPED") {
		t.Errorf("expected leading chars trimmed — HEAD_MARKER should not appear")
	}
}

func TestBuildPromptRendersHumanLabelsAndRollup(t *testing.T) {
	in := PromptInput{
		Template:              DefaultPromptTemplate,
		TestName:              "Login",
		ErrorMessage:          "expected 401, got 500",
		FailureType:           "assertion",
		SimilarFailuresRollup: "3 prior failures in 30d: 2 product_bug, 1 flaky",
		SimilarFailures: []SimilarFailure{
			{Status: "FAIL", ErrorMessage: "boom", DefectType: "product_bug", DefectKey: "BUG-42"},
			{Status: "ERROR", ErrorMessage: "kapow", DefectType: "flaky"}, // DefectType, no DefectKey
			{Status: "FAIL", ErrorMessage: "thud"},                        // no human label at all
		},
	}
	got, _, err := BuildPrompt(in)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	// Rollup header line is present.
	if !strings.Contains(got, "3 prior failures in 30d: 2 product_bug, 1 flaky") {
		t.Errorf("prompt missing rollup line\nfull:\n%s", got)
	}
	// Untrusted historical error messages must be wrapped in DATA fences, like
	// the primary error_message/stack_trace/log_text — so a replayed prior error
	// can't smuggle instructions past the SECURITY preamble.
	if !strings.Contains(got, "<<<DATA boom DATA>>>") {
		t.Errorf("historical error message should be wrapped in DATA fences\nfull:\n%s", got)
	}
	// Row with DefectType + DefectKey renders the full clause with the arrow.
	if !strings.Contains(got, "(human: product_bug → BUG-42)") {
		t.Errorf("prompt missing human label with defect key\nfull:\n%s", got)
	}
	// Row with DefectType but no DefectKey renders the clause without the arrow.
	if !strings.Contains(got, "(human: flaky)") {
		t.Errorf("prompt missing human label without defect key\nfull:\n%s", got)
	}
	if strings.Contains(got, "(human: flaky →") {
		t.Errorf("row without a defect key should omit the arrow\nfull:\n%s", got)
	}
	// Row with no DefectType omits the (human: ...) clause entirely.
	if strings.Contains(got, "thud (human:") {
		t.Errorf("row without a human label should omit the (human: ...) clause\nfull:\n%s", got)
	}
}

func TestBuildPromptDropOrderHoldsWithHumanLabeledHistory(t *testing.T) {
	// The enlarged per-row history block (human labels) must not disturb the
	// drop order: logs are dropped before similar failures. A giant step keeps
	// the prompt over cap so multiple drops are exercised.
	in := PromptInput{
		Template:              DefaultPromptTemplate,
		TestName:              "x",
		ErrorMessage:          "x",
		FailureType:           "x",
		LogText:               strings.Repeat("L", 30000),
		SimilarFailuresRollup: "2 prior failures: 1 product_bug, 1 flaky",
		SimilarFailures: []SimilarFailure{
			{Status: "FAIL", ErrorMessage: "boom", DefectType: "product_bug", DefectKey: "BUG-1"},
			{Status: "ERROR", ErrorMessage: "kapow", DefectType: "flaky"},
		},
		Steps: []PromptStep{{Order: 1, Action: strings.Repeat("a", 25000), Expected: "e"}},
	}
	got, meta, err := BuildPrompt(in)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if len(got) > PromptCharCap {
		t.Errorf("prompt length %d exceeds cap %d", len(got), PromptCharCap)
	}
	logIdx := strings.Index(meta.TruncationPrefix, "no logs")
	simIdx := strings.Index(meta.TruncationPrefix, "no similar failures")
	if logIdx < 0 || simIdx < 0 {
		t.Fatalf("expected both 'no logs' and 'no similar failures' in prefix, got %q", meta.TruncationPrefix)
	}
	if logIdx > simIdx {
		t.Errorf("expected logs dropped before similar failures, got prefix %q", meta.TruncationPrefix)
	}
	// Once similar failures are dropped, the human-label rows are gone too.
	if strings.Contains(got, "(human: product_bug") {
		t.Errorf("dropped similar failures should remove human-label rows\nfull:\n%s", got)
	}
	// The rollup summarizes rows that no longer appear — it must be cleared with
	// them, otherwise the prompt claims a distribution over failures it doesn't show.
	if strings.Contains(got, "prior failures") {
		t.Errorf("dropped similar failures should also drop the rollup line\nfull:\n%s", got)
	}
}
