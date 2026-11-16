package models

import (
	"encoding/json"
	"testing"
	"time"
)

func sampleTestCase() *TestCase {
	return &TestCase{
		ID:          "t1",
		FolderID:    "f1",
		Name:        "Login works",
		Description: "<p>desc</p>",
		Steps: []*TestStep{
			{ID: "s1", Action: "open", ExpectedResult: "form", OrderIndex: 0},
			{ID: "s2", Action: "submit", ExpectedResult: "home", OrderIndex: 1},
		},
		Categories: []*Category{
			{ID: "c1", Name: "Smoke"},
			{ID: "c2", Name: "Regression"},
		},
		CustomValues: []*CustomFieldValue{
			{ID: "v1", CustomFieldID: "cf1", Value: json.RawMessage(`"high"`)},
		},
	}
}

func TestTestCaseEditToken_StableAndOpaque(t *testing.T) {
	tc := sampleTestCase()
	tok := TestCaseEditToken(tc)
	if tok == "" {
		t.Fatal("token must not be empty")
	}
	if TestCaseEditToken(tc) != tok {
		t.Fatal("token must be deterministic for identical content")
	}
	if TestCaseEditToken(sampleTestCase()) != tok {
		t.Fatal("token must match for an equivalent record")
	}
}

func TestTestCaseEditToken_IgnoresAssociationOrder(t *testing.T) {
	base := TestCaseEditToken(sampleTestCase())

	reordCats := sampleTestCase()
	reordCats.Categories = []*Category{{ID: "c2", Name: "Regression"}, {ID: "c1", Name: "Smoke"}}
	if TestCaseEditToken(reordCats) != base {
		t.Error("reordering categories must not change the token")
	}

	// A second custom value lets us reorder them.
	twoCV := func() *TestCase {
		tc := sampleTestCase()
		tc.CustomValues = append(tc.CustomValues, &CustomFieldValue{ID: "v2", CustomFieldID: "cf2", Value: json.RawMessage(`"x"`)})
		return tc
	}
	a := twoCV()
	b := twoCV()
	b.CustomValues = []*CustomFieldValue{b.CustomValues[1], b.CustomValues[0]}
	if TestCaseEditToken(a) != TestCaseEditToken(b) {
		t.Error("reordering custom values must not change the token")
	}
}

func TestTestCaseEditToken_StepOrderMatters(t *testing.T) {
	reordered := sampleTestCase()
	reordered.Steps = []*TestStep{
		{ID: "s2", Action: "submit", ExpectedResult: "home", OrderIndex: 0},
		{ID: "s1", Action: "open", ExpectedResult: "form", OrderIndex: 1},
	}
	if TestCaseEditToken(reordered) == TestCaseEditToken(sampleTestCase()) {
		t.Error("reordering steps (changing order_index) must change the token")
	}
}

func TestTestCaseEditToken_IgnoresPreloadStepOrder(t *testing.T) {
	// Same steps, same order_index values, but returned in a different slice order
	// (as Preload might) — the token must be invariant to that.
	shuffled := sampleTestCase()
	shuffled.Steps = []*TestStep{shuffled.Steps[1], shuffled.Steps[0]}
	if TestCaseEditToken(shuffled) != TestCaseEditToken(sampleTestCase()) {
		t.Error("slice order must not matter when order_index is unchanged")
	}
}

func TestTestCaseEditToken_DetectsContentChanges(t *testing.T) {
	base := TestCaseEditToken(sampleTestCase())
	cases := map[string]func(*TestCase){
		"name":          func(tc *TestCase) { tc.Name = "changed" },
		"description":   func(tc *TestCase) { tc.Description = "changed" },
		"folder":        func(tc *TestCase) { tc.FolderID = "f2" },
		"step action":   func(tc *TestCase) { tc.Steps[0].Action = "changed" },
		"step expected": func(tc *TestCase) { tc.Steps[0].ExpectedResult = "changed" },
		"add category":  func(tc *TestCase) { tc.Categories = append(tc.Categories, &Category{ID: "c3"}) },
		"custom value":  func(tc *TestCase) { tc.CustomValues[0].Value = json.RawMessage(`"low"`) },
	}
	for name, mutate := range cases {
		tc := sampleTestCase()
		mutate(tc)
		if TestCaseEditToken(tc) == base {
			t.Errorf("%s change must change the token", name)
		}
	}
}

func TestTestCaseEditToken_ExcludesVolatileAndDisplayFields(t *testing.T) {
	base := TestCaseEditToken(sampleTestCase())
	cases := map[string]func(*TestCase){
		"created_at":    func(tc *TestCase) { tc.CreatedAt = time.Now() },
		"updated_at":    func(tc *TestCase) { tc.UpdatedAt = time.Now().Add(time.Hour) },
		"reverif flag":  func(tc *TestCase) { tc.ReverificationFlagged = true },
		"defect counts": func(tc *TestCase) { tc.OpenDefectCount = 5; tc.ClosedDefectCount = 3 },
		"steps count":   func(tc *TestCase) { tc.StepsCount = 99 },
		"category name": func(tc *TestCase) { tc.Categories[0].Name = "Renamed" },
		"linked reqs":   func(tc *TestCase) { tc.LinkedRequirements = []*Requirement{{ID: "r9"}} },
	}
	for name, mutate := range cases {
		tc := sampleTestCase()
		mutate(tc)
		if TestCaseEditToken(tc) != base {
			t.Errorf("%s must not affect the token (it is not editable content)", name)
		}
	}
}

func TestRequirementEditToken(t *testing.T) {
	base := &Requirement{ID: "r1", Identifier: "PROJ-1", Title: "T", Description: "D"}
	tok := RequirementEditToken(base)
	if tok == "" {
		t.Fatal("token must not be empty")
	}
	if RequirementEditToken(&Requirement{ID: "r1", Identifier: "PROJ-1", Title: "T", Description: "D"}) != tok {
		t.Fatal("token must be deterministic")
	}

	for name, mutate := range map[string]func(*Requirement){
		"identifier":  func(r *Requirement) { r.Identifier = "PROJ-2" },
		"title":       func(r *Requirement) { r.Title = "T2" },
		"description": func(r *Requirement) { r.Description = "D2" },
	} {
		r := &Requirement{ID: "r1", Identifier: "PROJ-1", Title: "T", Description: "D"}
		mutate(r)
		if RequirementEditToken(r) == tok {
			t.Errorf("%s change must change the token", name)
		}
	}

	for name, mutate := range map[string]func(*Requirement){
		"source type": func(r *Requirement) { r.SourceType = "jira" },
		"updated_at":  func(r *Requirement) { r.UpdatedAt = time.Now() },
		"parent":      func(r *Requirement) { p := "p1"; r.ParentID = &p },
	} {
		r := &Requirement{ID: "r1", Identifier: "PROJ-1", Title: "T", Description: "D"}
		mutate(r)
		if RequirementEditToken(r) != tok {
			t.Errorf("%s must not affect the token (not editable content)", name)
		}
	}
}
