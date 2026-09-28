package failureanalysis

import "time"

// TriageExample is one past human triage decision on a failing result the AI had suggested a
// defect type for: a few-shot example of how people here classify failures. The fields are
// stored values, raw and uncapped; BuildEvidence redacts and caps them like the rest of the evidence.
//
// It is declared here, not in store, because store imports failureanalysis (the default prompt
// template) and EnrichmentSource must name the store method's exact types; store aliases it.
type TriageExample struct {
	ResultID            string // the triaged run result (never sent; for tracing and tests)
	ErrorMessage        string
	FailureType         string
	SuggestedDefectType string // what the AI suggested when the person decided
	HumanDefectType     string // what the person decided
	Corrected           bool   // the person chose differently from the suggestion
}

// TriageExampleFilter selects and ranks examples for one analyzed failure. Before and Since are
// compared as instants (callers pass UTC). The analyzed run is excluded, and ranking prefers
// the same test case, then the same failure type, then the newest decision.
type TriageExampleFilter struct {
	TestCaseID   string    // "" = no test-case preference (a deleted test case)
	FailureType  string    // "" = no failure-type preference
	ExcludeRunID string    // the analyzed result's run
	Before       time.Time // decided strictly before this instant (when the analyzed failure happened)
	Since        time.Time // decided at or after this instant (the lookback floor)
	Limit        int       // at most this many; <= 0 returns nothing
}
