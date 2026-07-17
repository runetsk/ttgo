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

	// captured call state
	defectsCalls int
	reqsCalls    int
	historyCalls int
	gotTCID      string
	gotSince     time.Time
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

func (m *mockSource) ListRecentFailuresByTestCase(tcID string, since time.Time, limit int, excludeRunID string) ([]*models.RunResult, error) {
	m.historyCalls++
	m.gotSince = since
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

	ctx := BuildContext(src, result, now)

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

	// History rows carry DefectType; DefectKey is empty in v1.
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
	ctx := BuildContext(src, result, now)

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
	ctx := BuildContext(src, result, now)

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
	ctx = BuildContext(src, result, now)
	require.Nil(t, ctx.LinkedDefects)
	require.Zero(t, src.defectsCalls)
	require.Zero(t, src.historyCalls)
}

func TestBuildContextMalformedStepsJSON(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	src := &mockSource{}

	// Malformed JSON -> nil steps, never a panic.
	bad := &models.RunResult{ID: "rr1", TestCaseID: nil, Steps: json.RawMessage(`[{"action": bad`)}
	ctx := BuildContext(src, bad, now)
	require.Nil(t, ctx.Steps)

	// Absent / empty steps also yield nil.
	require.Nil(t, BuildContext(src, &models.RunResult{ID: "rr2"}, now).Steps)
	require.Nil(t, BuildContext(src, &models.RunResult{ID: "rr3", Steps: json.RawMessage(`[]`)}, now).Steps)
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

	ctx := BuildContext(src, result, now)
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
	ctx := BuildContext(&mockSource{}, result, now)

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
