package store

import (
	"testing"
)

// TestDemoDatasetStructure validates that the demo dataset assembles without
// panicking and that the new entities (defect links, AI analyses, provider
// config) are populated with deterministic IDs that line up with the
// RunResults and TestCases they reference.
func TestDemoDatasetStructure(t *testing.T) {
	ds := demoDataset()

	if len(ds.DefectLinks) == 0 {
		t.Fatalf("expected demo DefectLinks to be populated")
	}
	if len(ds.RunResultAnalyses) == 0 {
		t.Fatalf("expected demo RunResultAnalyses to be populated")
	}
	if len(ds.LLMProviders) != 1 {
		t.Fatalf("expected exactly one demo LLMProvider, got %d", len(ds.LLMProviders))
	}
	if len(ds.ReverificationFlaggedTCIDs) == 0 {
		t.Fatalf("expected at least one reverification-flagged test case")
	}

	// Every DefectLink with a RunResultID should point to an actual RunResult
	// in the dataset, and every TestCaseID should exist.
	runResultByID := map[string]bool{}
	for _, rr := range ds.RunResults {
		runResultByID[rr.ID] = true
	}
	testCaseByID := map[string]bool{}
	for _, tc := range ds.TestCases {
		testCaseByID[tc.ID] = true
	}
	for _, dl := range ds.DefectLinks {
		if !testCaseByID[dl.TestCaseID] {
			t.Errorf("DefectLink %s references unknown TestCaseID %s", dl.JiraIssueKey, dl.TestCaseID)
		}
		if dl.RunResultID != nil && !runResultByID[*dl.RunResultID] {
			t.Errorf("DefectLink %s references unknown RunResultID %s", dl.JiraIssueKey, *dl.RunResultID)
		}
	}

	// Every RunResultAnalysis should point to an existing RunResult.
	for _, a := range ds.RunResultAnalyses {
		if !runResultByID[a.RunResultID] {
			t.Errorf("RunResultAnalysis %s references unknown RunResultID %s", a.ID, a.RunResultID)
		}
	}

	// Retry chain: at least one RunResult should have AttemptNumber > 1.
	found := false
	for _, rr := range ds.RunResults {
		if rr.AttemptNumber > 1 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected at least one RunResult with AttemptNumber > 1 (retry chain)")
	}

	// At least one FAIL result should have rich artefacts populated.
	enriched := 0
	for _, rr := range ds.RunResults {
		if rr.Screenshots != "" && rr.LogText != "" && len(rr.Steps) > 0 {
			enriched++
		}
	}
	if enriched == 0 {
		t.Errorf("expected at least one enriched RunResult with screenshots/log/steps")
	}

	// Reverification flag target must exist in the dataset.
	for _, id := range ds.ReverificationFlaggedTCIDs {
		if !testCaseByID[id] {
			t.Errorf("ReverificationFlagged TestCaseID %s not in demo TestCases", id)
		}
	}
}
