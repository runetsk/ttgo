package failureanalysis

import (
	"encoding/json"
	"testing"
	"time"

	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// mockSource is an in-memory EnrichmentSource for unit tests (no DB).
type mockSource struct {
	defects    []models.Defect
	defectsErr error
	reqs       []*models.Requirement
	reqsErr    error
	history    []*models.RunResult
	historyErr error

	resultDefects    map[string][]models.Defect
	resultDefectsErr error
	resultDefectIDs  []string
	examples         []TriageExample
	examplesErr      error
	exampleCalls     int
	gotExampleFilter TriageExampleFilter
	categories       []string
	categoriesErr    error

	outcomes          string
	outcomesErr       error
	outcomesCalls     int
	gotOutcomesBefore time.Time
	gotOutcomesExcl   string
	gotOutcomesLimit  int

	// captured call state
	defectsCalls int
	reqsCalls    int
	historyCalls int
	gotTCID      string
	gotSince     time.Time
	gotBefore    time.Time
	gotLimit     int
	gotExclude   string
}

func (m *mockSource) ListDefectsByTestCase(tcID string) ([]models.Defect, error) {
	m.defectsCalls++
	m.gotTCID = tcID
	return m.defects, m.defectsErr
}

func (m *mockSource) ListRequirementsByTestCase(tcID string) ([]*models.Requirement, error) {
	m.reqsCalls++
	return m.reqs, m.reqsErr
}

func (m *mockSource) ListRecentFailuresByTestCase(tcID string, since, before time.Time, limit int, excludeRunID string) ([]*models.RunResult, error) {
	m.historyCalls++
	m.gotSince = since
	m.gotBefore = before
	m.gotLimit = limit
	m.gotExclude = excludeRunID
	return m.history, m.historyErr
}

func strptr(s string) *string { return &s }

// Compile-time assurance the mock satisfies the interface (as *store.Store must).
var _ EnrichmentSource = (*mockSource)(nil)

func TestBuildContextMapsAllSlots(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	steps, err := json.Marshal([]map[string]any{
		{"action": "<p>Go to /login</p>", "expected_result": "Login &amp; form shows", "order_index": 0},
		{"action": "Enter creds", "expected_result": "<b>Redirect</b>", "order_index": 1},
	})
	require.NoError(t, err)

	result := &models.RunResult{
		ID:          "rr-current",
		TestRunID:   "run-current",
		TestCaseID:  strptr("tc1"),
		Environment: "staging",
		Browser:     "chrome",
		OS:          "linux",
		AppVersion:  "1.2.3",
		Steps:       steps,
	}

	src := &mockSource{
		defects: []models.Defect{
			{ID: "defabcdefgh1234", ExternalKey: "JIRA-1", Status: "open", Title: "Login 500"},
			{ID: "abcdefghijklmnop", ExternalKey: "", Status: "open", Title: "Long ID defect"}, // -> shortID = abcdefgh
			{ID: "short", ExternalKey: "", Status: "closed", Title: "Short ID defect"},         // len < 8 -> "short", no panic
		},
		reqs: []*models.Requirement{
			{Identifier: "REQ-1", Title: "Must be able to login"},
		},
		history: []*models.RunResult{
			{ID: "h1", StartTime: now.Add(-1 * time.Hour), Status: models.StatusFail, ErrorMessage: "boom1", DefectType: "automation_bug"},
			{ID: "h2", StartTime: now.Add(-2 * time.Hour), Status: models.StatusError, ErrorMessage: "boom2", DefectType: "automation_bug"},
			{ID: "h3", StartTime: now.Add(-3 * time.Hour), Status: models.StatusFail, ErrorMessage: "boom3", DefectType: "product_bug"},
		},
	}

	ctx := BuildContext(src, result, now, 0)

	// Env fields copied straight off the RunResult.
	require.Same(t, result, ctx.Result)
	require.Equal(t, "staging", ctx.Env)
	require.Equal(t, "chrome", ctx.Browser)
	require.Equal(t, "linux", ctx.OS)
	require.Equal(t, "1.2.3", ctx.AppVersion)

	// Steps: HTML stripped, entities unescaped, 0-based order_index rendered 1-based.
	require.Len(t, ctx.Steps, 2)
	require.Equal(t, PromptStep{Order: 1, Action: "Go to /login", Expected: "Login & form shows"}, ctx.Steps[0])
	require.Equal(t, PromptStep{Order: 2, Action: "Enter creds", Expected: "Redirect"}, ctx.Steps[1])

	// Defects: ExternalKey preferred, else length-safe shortID; Title -> Summary.
	require.Len(t, ctx.LinkedDefects, 3)
	require.Equal(t, LinkedDefect{Key: "JIRA-1", Status: "open", Summary: "Login 500"}, ctx.LinkedDefects[0])
	require.Equal(t, LinkedDefect{Key: "abcdefgh", Status: "open", Summary: "Long ID defect"}, ctx.LinkedDefects[1])
	require.Equal(t, LinkedDefect{Key: "short", Status: "closed", Summary: "Short ID defect"}, ctx.LinkedDefects[2])

	// Requirements: Identifier -> Key, Title -> Title.
	require.Equal(t, []LinkedRequirement{{Key: "REQ-1", Title: "Must be able to login"}}, ctx.LinkedRequirements)

	// History rows carry DefectType; a row with no result-scoped defect has no DefectKey.
	require.Len(t, ctx.SimilarFailures, 3)
	require.Equal(t, "FAIL", ctx.SimilarFailures[0].Status)
	require.Equal(t, "boom1", ctx.SimilarFailures[0].ErrorMessage)
	require.Equal(t, "automation_bug", ctx.SimilarFailures[0].DefectType)
	require.Equal(t, "", ctx.SimilarFailures[0].DefectKey)
	require.True(t, ctx.SimilarFailures[0].RunStartedAt.Equal(now.Add(-1*time.Hour)))
	require.Equal(t, "ERROR", ctx.SimilarFailures[1].Status)

	// Rollup: highest count first, ties alphabetical.
	require.Equal(t, "automation_bug "+timesGlyph+"2, product_bug "+timesGlyph+"1", ctx.SimilarFailuresRollup)

	// Query args: current run excluded, 30-day window, capped at SimilarFailuresMax.
	require.Equal(t, "tc1", src.gotTCID)
	require.Equal(t, "run-current", src.gotExclude)
	require.Equal(t, SimilarFailuresMax, src.gotLimit)
	require.True(t, src.gotSince.Equal(now.AddDate(0, 0, -enrichHistoryDays)))
	require.True(t, src.gotBefore.Equal(now), "a result with no time of its own ends its history at now")
}

func TestBuildContextSourceErrorsAreBestEffort(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	steps, err := json.Marshal([]map[string]any{{"action": "step", "expected_result": "ok"}})
	require.NoError(t, err)

	result := &models.RunResult{
		ID: "rr1", TestRunID: "run1", TestCaseID: strptr("tc1"),
		Environment: "prod", Browser: "ff", OS: "win", AppVersion: "9",
		Steps: steps,
	}
	src := &mockSource{
		defectsErr: errContext("defects down"),
		reqsErr:    errContext("reqs down"),
		historyErr: errContext("history down"),
	}

	// Must not panic and must not lose the free (query-less) slots.
	ctx := BuildContext(src, result, now, 0)

	require.Nil(t, ctx.LinkedDefects)
	require.Nil(t, ctx.LinkedRequirements)
	require.Nil(t, ctx.SimilarFailures)
	require.Equal(t, "", ctx.SimilarFailuresRollup)

	require.Equal(t, "prod", ctx.Env)
	require.Equal(t, "ff", ctx.Browser)
	require.Equal(t, "win", ctx.OS)
	require.Equal(t, "9", ctx.AppVersion)
	require.Len(t, ctx.Steps, 1)
	require.Equal(t, PromptStep{Order: 1, Action: "step", Expected: "ok"}, ctx.Steps[0])
}

func TestBuildContextNilTestCaseIDSkipsQueries(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	steps, err := json.Marshal([]map[string]any{{"action": "only step", "expected_result": "still enriched"}})
	require.NoError(t, err)

	src := &mockSource{
		defects: []models.Defect{{ID: "d1", Title: "should not appear"}},
		history: []*models.RunResult{{ID: "h1", DefectType: "product_bug"}},
	}

	// nil TestCaseID (deleted test case): env + steps only, no cross-entity queries.
	result := &models.RunResult{ID: "rr1", TestRunID: "run1", TestCaseID: nil, Environment: "e2e", Steps: steps}
	ctx := BuildContext(src, result, now, 0)

	require.Equal(t, "e2e", ctx.Env)
	require.Len(t, ctx.Steps, 1)
	require.Nil(t, ctx.LinkedDefects)
	require.Nil(t, ctx.LinkedRequirements)
	require.Nil(t, ctx.SimilarFailures)
	require.Equal(t, "", ctx.SimilarFailuresRollup)
	require.Zero(t, src.defectsCalls)
	require.Zero(t, src.reqsCalls)
	require.Zero(t, src.historyCalls)

	// Empty-string TestCaseID takes the same skip path.
	result.TestCaseID = strptr("")
	ctx = BuildContext(src, result, now, 0)
	require.Nil(t, ctx.LinkedDefects)
	require.Zero(t, src.defectsCalls)
	require.Zero(t, src.historyCalls)
}

func TestBuildContextMalformedStepsJSON(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	src := &mockSource{}

	// Malformed JSON -> nil steps, never a panic.
	bad := &models.RunResult{ID: "rr1", TestCaseID: nil, Steps: json.RawMessage(`[{"action": bad`)}
	ctx := BuildContext(src, bad, now, 0)
	require.Nil(t, ctx.Steps)

	// Absent / empty steps also yield nil.
	require.Nil(t, BuildContext(src, &models.RunResult{ID: "rr2"}, now, 0).Steps)
	require.Nil(t, BuildContext(src, &models.RunResult{ID: "rr3", Steps: json.RawMessage(`[]`)}, now, 0).Steps)
}

func TestBuildContextStepOrderSynthesizedAndRollupEmptyWithoutLabels(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	// order_index absent (0) on both steps -> synthesized 1, 2 from position.
	steps, err := json.Marshal([]map[string]any{
		{"action": "first", "expected_result": "a"},
		{"action": "second", "expected_result": "b"},
	})
	require.NoError(t, err)

	src := &mockSource{
		history: []*models.RunResult{
			{ID: "h1", Status: models.StatusFail, ErrorMessage: "x"}, // no DefectType
			{ID: "h2", Status: models.StatusError, ErrorMessage: "y"},
		},
	}
	result := &models.RunResult{ID: "rr1", TestRunID: "run1", TestCaseID: strptr("tc1"), Steps: steps}

	ctx := BuildContext(src, result, now, 0)
	require.Equal(t, 1, ctx.Steps[0].Order)
	require.Equal(t, 2, ctx.Steps[1].Order)

	// No labels anywhere -> empty rollup, but the history rows still map.
	require.Len(t, ctx.SimilarFailures, 2)
	require.Equal(t, "", ctx.SimilarFailuresRollup)
}

func TestBuildContextStepOrderZeroBasedRenderedOneBased(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	// order_index is 0-based repo-wide, so a PRESENT 0 must not be mistaken for
	// absent: real steps [0,1,2] must render Orders [1,2,3], not [1,1,2].
	steps, err := json.Marshal([]map[string]any{
		{"action": "first", "expected_result": "a", "order_index": 0},
		{"action": "second", "expected_result": "b", "order_index": 1},
		{"action": "third", "expected_result": "c", "order_index": 2},
	})
	require.NoError(t, err)

	result := &models.RunResult{ID: "rr1", TestRunID: "run1", TestCaseID: strptr("tc1"), Steps: steps}
	ctx := BuildContext(&mockSource{}, result, now, 0)

	require.Len(t, ctx.Steps, 3)
	require.Equal(t, 1, ctx.Steps[0].Order)
	require.Equal(t, 2, ctx.Steps[1].Order)
	require.Equal(t, 3, ctx.Steps[2].Order)
}

func TestRollupDefectTypesTieBreakAlphabetical(t *testing.T) {
	// Equal counts must break ties alphabetically for a deterministic rollup.
	hist := []*models.RunResult{
		{DefectType: "zebra_bug"},
		{DefectType: "apple_bug"},
	}
	require.Equal(t, "apple_bug "+timesGlyph+"1, zebra_bug "+timesGlyph+"1", rollupDefectTypes(hist))
}

// errContext is a tiny error helper so tests don't need the errors import churn.
type errContext string

func (e errContext) Error() string { return string(e) }

func TestBuildContextHistoryEndsWhenTheResultRan(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	ran := time.Date(2026, 9, 21, 3, 0, 0, 0, time.UTC)

	src := &mockSource{}
	BuildContext(src, &models.RunResult{TestRunID: "old-run", TestCaseID: strptr("tc1"), StartTime: ran, CreatedAt: ran.Add(time.Minute)}, now, 0)
	require.True(t, src.gotBefore.Equal(ran), "analyzing an older run must not see what came after it: %v", src.gotBefore)
	require.True(t, src.gotSince.Equal(ran.AddDate(0, 0, -enrichHistoryDays)), "the 30-day lookback counts back from the result")

	// Recorded without timing (a manual result): when its row was written.
	written := time.Date(2026, 9, 20, 9, 30, 0, 0, time.UTC)
	BuildContext(src, &models.RunResult{TestRunID: "manual-run", TestCaseID: strptr("tc1"), CreatedAt: written}, now, 0)
	require.True(t, src.gotBefore.Equal(written), "%v", src.gotBefore)
	require.True(t, src.gotSince.Equal(written.AddDate(0, 0, -enrichHistoryDays)))
}

func TestBuildContextDefectKeyPerHistoryRow(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	src := &mockSource{
		history: []*models.RunResult{
			{ID: "h1", StartTime: now.Add(-1 * time.Hour), Status: models.StatusFail, DefectType: "product_bug"},
			{ID: "h2", StartTime: now.Add(-2 * time.Hour), Status: models.StatusFail, DefectType: "product_bug"},
			{ID: "h3", StartTime: now.Add(-3 * time.Hour), Status: models.StatusFail},
		},
		resultDefects: map[string][]models.Defect{
			"h1": {{ID: "d-newest-0001", ExternalKey: "JIRA-9"}, {ID: "d-older-0002", ExternalKey: "JIRA-1"}},
			"h2": {{ID: "abcdefghijkl"}},
		},
	}
	ctx := BuildContext(src, &models.RunResult{ID: "rr", TestRunID: "run", TestCaseID: strptr("tc1")}, now, 0)
	require.Len(t, ctx.SimilarFailures, 3)
	require.Equal(t, "JIRA-9", ctx.SimilarFailures[0].DefectKey, "the first result-scoped defect (most recently linked)")
	require.Equal(t, "abcdefgh", ctx.SimilarFailures[1].DefectKey, "no external key: the short id")
	require.Equal(t, "", ctx.SimilarFailures[2].DefectKey, "no defect on that result")
	require.Equal(t, []string{"h1", "h2", "h3"}, src.resultDefectIDs)

	src.resultDefectsErr = errContext("links down")
	ctx = BuildContext(src, &models.RunResult{ID: "rr", TestRunID: "run", TestCaseID: strptr("tc1")}, now, 0)
	require.Len(t, ctx.SimilarFailures, 3, "a failed key lookup keeps the history row")
	require.Equal(t, "", ctx.SimilarFailures[0].DefectKey)
}

func TestBuildContextExamples(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	ran := time.Date(2026, 9, 21, 10, 0, 0, 0, time.FixedZone("EEST", 3*3600))
	result := &models.RunResult{ID: "rr", TestRunID: "run-1", TestCaseID: strptr("tc1"), FailureType: "timeout", StartTime: ran}

	src := &mockSource{examples: []TriageExample{{ResultID: "x1", ErrorMessage: "boom"}}}
	ctx := BuildContext(src, result, now, 4)
	require.Equal(t, 1, src.exampleCalls)
	f := src.gotExampleFilter
	require.Equal(t, "tc1", f.TestCaseID)
	require.Equal(t, "timeout", f.FailureType)
	require.Equal(t, "run-1", f.ExcludeRunID)
	require.Equal(t, 4, f.Limit)
	require.True(t, f.Before.Equal(ran), "examples end when the analyzed failure happened: %v", f.Before)
	require.Equal(t, time.UTC, f.Before.Location(), "the anchor is passed as UTC")
	require.True(t, f.Since.Equal(ran.AddDate(0, 0, -ExampleWindowDays)))
	require.Equal(t, time.UTC, f.Since.Location())
	require.Equal(t, src.examples, ctx.Examples)

	off := &mockSource{}
	require.Nil(t, BuildContext(off, result, now, 0).Examples)
	require.Zero(t, off.exampleCalls, "0 = off: no query")

	BuildContext(off, result, now, 50)
	require.Equal(t, models.MaxFewShotExamples, off.gotExampleFilter.Limit, "the limit is clamped to the maximum")

	// A deleted test case still gets examples from other tests, with no test-case preference.
	orphan := &mockSource{examples: []TriageExample{{ResultID: "x2"}}}
	ctx = BuildContext(orphan, &models.RunResult{ID: "rr", TestRunID: "run-1", FailureType: "timeout", StartTime: ran}, now, 4)
	require.Equal(t, "", orphan.gotExampleFilter.TestCaseID)
	require.Len(t, ctx.Examples, 1)
	require.Zero(t, orphan.defectsCalls, "the test-case queries are still skipped")

	failing := &mockSource{examplesErr: errContext("db down")}
	require.Nil(t, BuildContext(failing, result, now, 4).Examples, "a failed lookup leaves the slot empty")
}

func TestBuildContextCategories(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	result := &models.RunResult{ID: "rr", TestRunID: "run", TestCaseID: strptr("tc1")}
	src := &mockSource{categories: []string{"Checkout", "Smoke"}}
	require.Equal(t, "Checkout; Smoke", BuildContext(src, result, now, 0).Categories)

	src.categoriesErr = errContext("db down")
	require.Equal(t, "", BuildContext(src, result, now, 0).Categories)
}

func (m *mockSource) ListDefectsByResult(resultID string) ([]models.Defect, error) {
	m.resultDefectIDs = append(m.resultDefectIDs, resultID)
	return m.resultDefects[resultID], m.resultDefectsErr
}

func (m *mockSource) ListTriageExamples(f TriageExampleFilter) ([]TriageExample, error) {
	m.exampleCalls++
	m.gotExampleFilter = f
	return m.examples, m.examplesErr
}

func (m *mockSource) ListCategoryNamesByTestCase(string) ([]string, error) {
	return m.categories, m.categoriesErr
}

func (m *mockSource) ListRecentOutcomesByTestCase(tcID string, before time.Time, excludeRunID string, limit int) (string, error) {
	m.outcomesCalls++
	m.gotOutcomesBefore, m.gotOutcomesExcl, m.gotOutcomesLimit = before, excludeRunID, limit
	return m.outcomes, m.outcomesErr
}

func TestBuildContextRecentOutcomes(t *testing.T) {
	eest := time.FixedZone("EEST", 3*3600)
	ran := time.Date(2026, 9, 24, 10, 0, 0, 0, eest)
	result := &models.RunResult{ID: "rr", TestRunID: "run-now", TestCaseID: strptr("tc1"), StartTime: ran}

	src := &mockSource{outcomes: "PFPFE"}
	ctx := BuildContext(src, result, time.Now(), 0)
	require.Equal(t, "PFPFE", ctx.RecentOutcomes)
	require.Equal(t, 1, src.outcomesCalls)
	require.True(t, src.gotOutcomesBefore.Equal(ran), "the same anchor as the history window")
	require.Equal(t, time.UTC, src.gotOutcomesBefore.Location(), "passed as UTC")
	require.Equal(t, "run-now", src.gotOutcomesExcl, "the analyzed run is excluded")
	require.Equal(t, RecentOutcomesLimit, src.gotOutcomesLimit)
	require.Equal(t, 10, RecentOutcomesLimit)
	require.False(t, ctx.HistoryAvailable(), "recent outcomes are not the history block history_available records")

	failing := &mockSource{outcomesErr: errContext("db down")}
	require.Empty(t, BuildContext(failing, result, time.Now(), 0).RecentOutcomes, "best effort: an error leaves it empty")

	orphan := &mockSource{outcomes: "PPP"}
	noCase := *result
	noCase.TestCaseID = nil
	require.Empty(t, BuildContext(orphan, &noCase, time.Now(), 0).RecentOutcomes)
	require.Zero(t, orphan.outcomesCalls, "no test case, no query")
}
