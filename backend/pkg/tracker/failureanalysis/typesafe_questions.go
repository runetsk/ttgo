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
	PolicyVersion         = "fa-verdict-v1"  // verdict + defect_type questions and thresholds
	SemanticPolicyVersion = "fa-semantic-v1" // same-cause question and grouping thresholds
)

// Verdict-path thresholds (spec §6).
const (
	VerdictHighMin       = 0.90 // confidence >= → bucket "high"
	VerdictMediumMin     = 0.50 // confidence >= → bucket "medium"; below → "low"
	DefectTypeSuggestMin = 0.50 // defect_type confidence below this → abstain (no suggestion)
	RunnerUpMargin       = 0.15 // narrative mentions the runner-up verdict only when top-two gap < this
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
// It says "other", not "earlier": the query window ends at analysis time and excludes only
// the current run, so a re-analysis of an old result can include later failures.
const HistoryNote = "Other FAILED or ERRORED runs of this same test from the last 30 days, excluding this run. Passing runs are not listed. Labels are what a person assigned to those other failures, not to this one."

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
			"Entries in `history` are other failures of the same test and their labels describe those failures, not this one. " +
			"Choose `unknown` when no listed cause is identified, or when the evidence supports more than one listed cause and does not distinguish which one caused this failure.",
		Criteria: map[string]any{
			models.VerdictProductBug:     "The evidence identifies incorrect application behavior against an expectation the test states, and attributes the failure to the application. A failed assertion, an application error, or the absence of evidence for another cause is alone insufficient. A linked defect in `linked_defects` that describes this same incorrect behavior is supporting evidence.",
			models.VerdictFlakyTest:      "The evidence identifies a test-side synchronization, timing, or stale-handle fault that caused this failure: the test acted before the application reached the state the test assumed, reused an element or handle after the page changed, or asserted on timing-dependent data. A successful retry or intermittent execution alone does not identify a test fault.",
			models.VerdictEnvironment:    "The evidence identifies a specific missing or incorrect configuration of the test environment as the cause: an environment variable, browser, driver, credential, feature flag, base URL, or a dependency not deployed in this environment. A configuration value merely appearing in the evidence is insufficient.",
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
			"Labels in `history` describe other failures, not this one. " +
			"Choose `insufficient_evidence` when no source is identified, or when the evidence supports more than one source and does not distinguish which one caused this failure.",
		Criteria: map[string]any{
			"product_bug":          "The evidence identifies the application under test behaving wrongly as the cause, and the fix would be made in the application.",
			"automation_bug":       "The evidence identifies a fault in the test itself as the cause: a wrong assertion, a wrong locator, a wrong timing assumption, or wrong test data or fixtures, and the fix would be made in the test automation.",
			"system_issue":         "The evidence identifies the environment, shared infrastructure, or an external system failing or misconfigured as the cause, and the fix would be made outside both the application code and the test code.",
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
