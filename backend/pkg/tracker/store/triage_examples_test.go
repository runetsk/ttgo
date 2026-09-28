package store

import (
	"fmt"
	"testing"
	"time"

	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// *Store is the enrichment source BuildContext reads; the signatures must match exactly.
var _ failureanalysis.EnrichmentSource = (*Store)(nil)

// Timestamps keep the offset they were written with, so every bound and the order must follow
// the instant, not the text. Each "text would…" note names the row a TEXT comparison gets wrong.
func TestListTriageExamples_PredicatesRankingAndInstants(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Checkout", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc))
	other := &models.TestCase{Name: "Search", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(other))
	cur := &models.TestRun{Name: "analyzed"}
	require.NoError(t, s.CreateTestRun(cur))
	prior := &models.TestRun{Name: "prior"}
	require.NoError(t, s.CreateTestRun(prior))

	eest := time.FixedZone("EEST", 3*3600)
	edt := time.FixedZone("EDT", -4*3600)
	anchor := time.Date(2026, 9, 24, 10, 0, 0, 0, eest) // 07:00Z
	floor := anchor.UTC().AddDate(0, 0, -90)
	direct, clone := tsBool(false), tsBool(true)
	at := func(tm time.Time) *time.Time { return &tm }

	type row struct {
		name, run, ft, suggested, human string
		tcID                            *string
		status                          models.ExecutionStatus
		clone                           *bool
		decided                         *time.Time
	}
	seed := func(r row) string {
		t.Helper()
		st := r.status
		if st == "" {
			st = models.StatusFail
		}
		rr := &models.RunResult{TestRunID: r.run, TestCaseID: r.tcID, TestNameSnapshot: r.name, Status: st,
			FailureType: r.ft, ErrorMessage: r.name, DefectType: r.human,
			SuggestedDefectType: r.suggested, SuggestedIsClone: r.clone, DecidedAt: r.decided}
		require.NoError(t, s.AddRunResult(rr))
		return rr.ID
	}

	// Returned, in rank order.
	r1 := seed(row{name: "same case, same type, agreed", run: prior.ID, tcID: &tc.ID, ft: "timeout", suggested: "product_bug", human: "product_bug", clone: direct, decided: at(time.Date(2026, 9, 24, 6, 0, 0, 0, time.UTC))})
	r2 := seed(row{name: "same case, other type, corrected", run: prior.ID, tcID: &tc.ID, ft: "assertion", suggested: "product_bug", human: "automation_bug", clone: direct, decided: at(time.Date(2026, 9, 24, 5, 0, 0, 0, time.UTC))})
	r3 := seed(row{name: "other case, same type, corrected", run: prior.ID, tcID: &other.ID, ft: "timeout", suggested: "product_bug", human: "system_issue", clone: direct, decided: at(time.Date(2026, 9, 24, 6, 30, 0, 0, time.UTC))})
	r4 := seed(row{name: "other case, newest, agreed", run: prior.ID, tcID: &other.ID, ft: "network", suggested: "automation_bug", human: "automation_bug", clone: direct, decided: at(time.Date(2026, 9, 24, 6, 50, 0, 0, time.UTC))})
	// 06:40Z stored as 09:40+03:00. Text would drop it (09:40 > 07:00), and text DESC would sort it before r4.
	r5 := seed(row{name: "other case, local offset, corrected", run: prior.ID, tcID: &other.ID, ft: "network", suggested: "product_bug", human: "automation_bug", clone: direct, decided: at(time.Date(2026, 9, 24, 9, 40, 0, 0, eest))})
	r6 := seed(row{name: "89 days ago, agreed", run: prior.ID, tcID: &other.ID, ft: "network", suggested: "system_issue", human: "system_issue", clone: direct, decided: at(anchor.UTC().AddDate(0, 0, -89))})

	// Excluded.
	seed(row{name: "clone", run: prior.ID, tcID: &tc.ID, ft: "timeout", suggested: "product_bug", human: "product_bug", clone: clone, decided: at(time.Date(2026, 9, 24, 6, 55, 0, 0, time.UTC))})
	seed(row{name: "unknown provenance", run: prior.ID, tcID: &tc.ID, ft: "timeout", suggested: "product_bug", human: "product_bug", clone: nil, decided: at(time.Date(2026, 9, 24, 6, 55, 0, 0, time.UTC))})
	seed(row{name: "the analyzed run", run: cur.ID, tcID: &other.ID, ft: "timeout", suggested: "product_bug", human: "product_bug", clone: direct, decided: at(time.Date(2026, 9, 24, 6, 55, 0, 0, time.UTC))})
	// 07:30Z stored as 03:30-04:00. Text would keep it (03:30 < 07:00), but it was decided after the anchor.
	seed(row{name: "after the anchor, negative offset", run: prior.ID, tcID: &tc.ID, ft: "timeout", suggested: "product_bug", human: "product_bug", clone: direct, decided: at(time.Date(2026, 9, 24, 3, 30, 0, 0, edt))})
	seed(row{name: "at the anchor", run: prior.ID, tcID: &tc.ID, ft: "timeout", suggested: "product_bug", human: "product_bug", clone: direct, decided: at(anchor)})
	// 30 minutes before the floor, stored as +03:00. Text would keep it.
	seed(row{name: "before the 90-day floor, local offset", run: prior.ID, tcID: &tc.ID, ft: "timeout", suggested: "product_bug", human: "product_bug", clone: direct, decided: at(floor.Add(-30 * time.Minute).In(eest))})
	seed(row{name: "untriaged", run: prior.ID, tcID: &tc.ID, ft: "timeout", suggested: "product_bug", human: "to_investigate", clone: direct, decided: at(time.Date(2026, 9, 24, 6, 55, 0, 0, time.UTC))})
	seed(row{name: "no suggestion", run: prior.ID, tcID: &tc.ID, ft: "timeout", suggested: "", human: "product_bug", clone: direct, decided: at(time.Date(2026, 9, 24, 6, 55, 0, 0, time.UTC))})
	seed(row{name: "passed later", run: prior.ID, tcID: &tc.ID, ft: "timeout", suggested: "product_bug", human: "product_bug", status: models.StatusPass, clone: direct, decided: at(time.Date(2026, 9, 24, 6, 55, 0, 0, time.UTC))})
	seed(row{name: "never decided", run: prior.ID, tcID: &tc.ID, ft: "timeout", suggested: "product_bug", human: "product_bug", clone: direct})

	got, err := s.ListTriageExamples(TriageExampleFilter{TestCaseID: tc.ID, FailureType: "timeout",
		ExcludeRunID: cur.ID, Before: anchor.UTC(), Since: floor, Limit: models.MaxFewShotExamples})
	require.NoError(t, err)
	ids := make([]string, 0, len(got))
	for _, e := range got {
		ids = append(ids, e.ResultID)
	}
	require.Equal(t, []string{r1, r2, r3, r4, r5, r6}, ids,
		"same test case, then same failure type, then newest instant; clones, unknown provenance, the analyzed run, later, older and non-calibration rows excluded")

	require.Equal(t, failureanalysis.TriageExample{ResultID: r2, ErrorMessage: "same case, other type, corrected",
		FailureType: "assertion", SuggestedDefectType: "product_bug", HumanDefectType: "automation_bug", Corrected: true}, got[1])
	require.False(t, got[0].Corrected)
}

func TestListTriageExamples_BalancesCorrectionsAndAgreements(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	anchor := time.Now().UTC()
	add := func(n int, corrected bool) string {
		t.Helper()
		human := "product_bug"
		if corrected {
			human = "automation_bug"
		}
		decided := anchor.Add(-time.Duration(n) * time.Minute) // n = 1 is the newest
		rr := &models.RunResult{TestRunID: runID, TestNameSnapshot: fmt.Sprintf("r%d", n), AttemptNumber: n,
			Status: models.StatusFail, ErrorMessage: fmt.Sprintf("r%d", n), DefectType: human,
			SuggestedDefectType: "product_bug", SuggestedIsClone: tsBool(false), DecidedAt: &decided}
		require.NoError(t, s.AddRunResult(rr))
		return rr.ID
	}
	c1, c2, c3 := add(1, true), add(2, true), add(3, true)
	a4, a5, a6 := add(4, false), add(5, false), add(6, false)
	list := func(limit int) []string {
		t.Helper()
		got, err := s.ListTriageExamples(TriageExampleFilter{ExcludeRunID: "another-run",
			Before: anchor, Since: anchor.AddDate(0, 0, -90), Limit: limit})
		require.NoError(t, err)
		ids := []string{}
		for _, e := range got {
			ids = append(ids, e.ResultID)
		}
		return ids
	}
	require.Equal(t, []string{c1, a4}, list(2), "at most ceil(2/2) = 1 correction while agreements exist")
	require.Equal(t, []string{c1, c2, a4, a5}, list(4))
	require.Equal(t, []string{c1, c2, c3, a4, a5}, list(5), "ceil(5/2) = 3 corrections")
	require.Equal(t, []string{c1, c2, c3, a4, a5, a6}, list(8), "fewer candidates than the limit: all of them")
	require.Empty(t, list(0))
}

func TestListTriageExamples_FillsFromOneKindWhenTheOtherRunsOut(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	anchor := time.Now().UTC()
	var ids []string
	for n := 1; n <= 3; n++ {
		decided := anchor.Add(-time.Duration(n) * time.Minute)
		rr := &models.RunResult{TestRunID: runID, TestNameSnapshot: fmt.Sprintf("c%d", n), AttemptNumber: n,
			Status: models.StatusError, ErrorMessage: "boom", DefectType: "system_issue",
			SuggestedDefectType: "product_bug", SuggestedIsClone: tsBool(false), DecidedAt: &decided}
		require.NoError(t, s.AddRunResult(rr))
		ids = append(ids, rr.ID)
	}
	got, err := s.ListTriageExamples(TriageExampleFilter{ExcludeRunID: "x", Before: anchor, Since: anchor.AddDate(0, 0, -90), Limit: 3})
	require.NoError(t, err)
	require.Len(t, got, 3, "no agreements to balance with: corrections fill the limit")
	for i, e := range got {
		require.Equal(t, ids[i], e.ResultID)
		require.True(t, e.Corrected)
	}
}

func TestListCategoryNamesByTestCase(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Checkout", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc))
	smoke, err := s.CreateCategory("Smoke", "")
	require.NoError(t, err)
	checkout, err := s.CreateCategory("Checkout", "")
	require.NoError(t, err)
	_, err = s.CreateCategory("Unrelated", "")
	require.NoError(t, err)
	require.NoError(t, s.AssignCategoryToTest(smoke.ID, tc.ID))
	require.NoError(t, s.AssignCategoryToTest(checkout.ID, tc.ID))

	names, err := s.ListCategoryNamesByTestCase(tc.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"Checkout", "Smoke"}, names, "the test's categories, by name")

	none, err := s.ListCategoryNamesByTestCase("missing")
	require.NoError(t, err)
	require.Empty(t, none)
}
