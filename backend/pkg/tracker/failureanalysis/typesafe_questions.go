package failureanalysis

import (
	"fmt"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"
)

// This file is the ONLY place TypeSafe questions, criteria and thresholds live.
// Bump the matching policy version whenever any of them changes: stored analyses
// record it, so calibration can be read per policy.
const (
	PolicyVersion         = "fa-verdict-v5"  // verdict + defect_type questions, thresholds and the suggestion rule (v5: a confident verdict decides the suggestion)
	SemanticPolicyVersion = "fa-semantic-v1" // same-cause question and grouping thresholds
)

// Verdict-path thresholds (spec §6).
const (
	VerdictHighMin       = 0.90 // confidence >= → bucket "high"
	VerdictMediumMin     = 0.50 // confidence >= → bucket "medium"; below → "low"
	DefectTypeSuggestMin = 0.50 // defect_type confidence below this → abstain (no suggestion)
	// VerdictDecidesSuggestionMin: a verdict at or above this (and not unknown) decides the
	// suggestion through the verdict-to-defect mapping, whatever the defect_type question said.
	// Set below VerdictHighMin in v5: on the benchmark the question withheld a runner
	// out-of-memory suggestion under an infrastructure verdict at 0.88.
	VerdictDecidesSuggestionMin = 0.85
	RunnerUpMargin              = 0.15 // narrative mentions the runner-up verdict only when top-two gap < this
)

// DefectTypeInsufficient is the explicit "none of the three" option of the defect_type question.
const DefectTypeInsufficient = "insufficient_evidence"

// Semantic-grouping thresholds and caps (spec §7).
const (
	SameCauseMin           = 0.80  // noul >= → the pair is an edge
	MaxRepsPerBlock        = 25    // representatives considered per failure_type block
	LexicalMin             = 0.15  // token Jaccard of normalized messages to become a candidate pair
	MaxPairsPerJob         = 400   // global candidate cap per job
	MaxQuestionsPerRequest = 100   // pairs per TypeSafe request
	ChunkCharBudget        = 40000 // state JSON + questions, characters (heuristic: <= ~20k tokens)
	MaxGroupsPerCluster    = 8     // original groups per merged cluster
	ExcerptErrorChars      = 600   // per-representative error excerpt
	ExcerptStackChars      = 600   // per-representative stack excerpt
)

// HistoryNote is a constant sent inside the state so the model reads history correctly.
// It says "other" rather than "earlier" for historical reasons: until 2026-09-23 the query
// window ended at analysis time, so a re-analysis of an old result could include later
// failures. The window now ends at the analyzed result (BuildContext), so every entry is
// earlier; the wording is kept because it is part of the state the policy was tuned on.
const HistoryNote = "Other FAILED or ERRORED runs of this same test from the last 30 days, excluding this run. Passing runs are not listed. Labels are what a person assigned to those other failures; an entry with the same error condition as this failure, and its label, bear on this one."

// verdictOptions returns the six verdicts in a stable order.
func verdictOptions() []string {
	return []string{models.VerdictProductBug, models.VerdictFlakyTest, models.VerdictEnvironment,
		models.VerdictTestData, models.VerdictInfrastructure, models.VerdictUnknown}
}

// defectTypeOptions returns the three canonical defect types plus the abstain option.
func defectTypeOptions() []string {
	return []string{"product_bug", "automation_bug", "system_issue", DefectTypeInsufficient}
}

func verdictQuestion() typesafe.Question {
	return typesafe.Question{
		Type: "choice",
		Instructions: "Which listed cause does the evidence in `failure`, `test`, `history`, and `linked_defects` identify as the cause of this failure? " +
			"A timeout, a failed assertion, an error message, a connection error, or a successful retry on its own does not identify a cause. " +
			"Entries in `history` are other failures of the same test. When an entry shows the same error condition as this failure, that entry and its human label are evidence about this failure; otherwise they describe a different failure. " +
			"Choose `unknown` when no listed cause is identified, or when the evidence supports more than one listed cause and does not distinguish which one caused this failure.",
		Criteria: map[string]any{
			models.VerdictProductBug:     "The evidence identifies incorrect application behavior as the cause: the application produced a result that contradicts an expectation the test states, or the application answered an operation with a server error or exception and `history` shows the same operation failing the same way in other runs. A single failed assertion or application error with no stated expectation and no recurrence is insufficient. A linked defect describing this same incorrect behavior, or `history` entries with the same error labeled product_bug, are supporting evidence.",
			models.VerdictFlakyTest:      "The evidence identifies a test-side synchronization, timing, or stale-handle fault as the cause: the test acted before the application reached the state the test assumed, reused an element or handle after the page changed, asserted on timing-dependent data such as an animation or a transient notification, or its wait for such an element expired while the operation itself is not shown to have failed. A wait or selector timeout that `history` shows recurring on this test with the same error, or whose matching `history` entries are labeled automation_bug, identifies such a fault. A successful retry alone is insufficient.",
			models.VerdictEnvironment:    "The evidence identifies the environment under test as the cause: its base URL or an application host that does not respond or times out on navigation or connection, a dependency not deployed there, or a specific missing or incorrect configuration such as an environment variable, browser, driver, credential, feature flag, or base URL. A navigation timeout or connection failure against the application's own URL identifies the environment unless the evidence names a runner, network, or CI fault instead. A configuration value merely appearing in the evidence is insufficient.",
			models.VerdictTestData:       "The evidence identifies missing, stale, already-consumed, duplicated, or malformed fixture or input data as the cause. A record absent from application output alone is insufficient.",
			models.VerdictInfrastructure: "The evidence identifies an outage or resource failure of a shared system as the cause: a service reported as down, a host unreachable across operations, disk full, out of memory, a container, runner, or CI agent lost. Connection, DNS, and TLS errors alone do not distinguish an outage from a configuration or application fault.",
			models.VerdictUnknown:        "No other option's cause is identified, or the evidence supports more than one listed cause without distinguishing them. This includes a deterministic wrong assertion or wrong locator in the test code, which this vocabulary has no option for.",
		},
	}
}

func defectTypeQuestion() typesafe.Question {
	return typesafe.Question{
		Type: "choice",
		Instructions: "Which source of this failure does the evidence in `failure`, `test`, `history`, and `linked_defects` identify: the application code, the test automation or its fixtures, the environment or an external system, or is the evidence insufficient? " +
			"Labels in `history` describe other failures; an entry with the same error condition as this failure, and its label, bear on this one. " +
			"Choose `insufficient_evidence` when no source is identified, or when the evidence supports more than one source and does not distinguish which one caused this failure.",
		Criteria: map[string]any{
			"product_bug":          "The evidence identifies the application under test behaving wrongly as the cause, and the fix would be made in the application: a result that contradicts an expectation the test states, or a server error or exception on an operation that `history` shows failing the same way in other runs.",
			"automation_bug":       "The evidence identifies a fault in the test itself as the cause, and the fix would be made in the test automation: a wrong assertion, a wrong locator, a wrong timing assumption, a wait for a transient element or notification that expired, a stale element handle reused after the page changed, an assertion on timing-dependent data such as an animation, or wrong test data or fixtures. A wait or selector timeout that `history` shows recurring on this test with the same error, or whose matching `history` entries are labeled automation_bug, identifies such a fault.",
			"system_issue":         "The evidence identifies the environment under test, shared infrastructure, or an external system as the cause, and the fix would be made outside both the application code and the test code: the application's own URL or host not responding or timing out, a dependency not deployed, a misconfiguration of the environment, or an outage or resource failure of a shared system such as a service down, a host unreachable, disk full, out of memory, or a runner or CI agent lost.",
			DefectTypeInsufficient: "The evidence identifies none of the other three, or supports more than one of them without distinguishing which caused this failure.",
		},
	}
}

// sameCauseQuestion asks about two entries of the chunk-local `failures` array. a and b are
// positions in that array (NOT global ids); the instructions carry them because question ids
// are never shown to the model.
func sameCauseQuestion(a, b int) typesafe.Question {
	return typesafe.Question{
		Type: "noul",
		Instructions: map[string]any{
			"question": fmt.Sprintf("Do `failures[%d]` and `failures[%d]` describe the same failing operation and the same error condition, with differences limited to incidental execution details?", a, b),
			"compare":  []int{a, b},
		},
		Criteria: map[string]any{
			"true":  "Both excerpts name the same operation, endpoint, or component and the same error condition or assertion meaning. A difference is incidental only when it does not change the operation, the resource, the assertion, or the error condition: run identifiers, timestamps, memory addresses, and sentence wording are incidental; durations and identifiers are not incidental when they are the asserted value or identify the failing resource.",
			"false": "The excerpts differ in something that carries the failure's meaning: a different asserted value, a different operation, endpoint, or component, a different error type, a different resource path when the path identifies what was being operated on, or one excerpt does not contain enough to tell. A redaction placeholder carries no information: two placeholders in the same position do not show that the underlying values match.",
		},
	}
}

// confidenceBucket maps a TypeSafe confidence to the stored low|medium|high string.
func confidenceBucket(score float64) string {
	switch {
	case score >= VerdictHighMin:
		return models.ConfidenceHigh
	case score >= VerdictMediumMin:
		return models.ConfidenceMedium
	default:
		return models.ConfidenceLow
	}
}
