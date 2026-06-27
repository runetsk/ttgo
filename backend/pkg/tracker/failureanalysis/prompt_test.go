package failureanalysis

import (
	"strings"
	"testing"
)

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
