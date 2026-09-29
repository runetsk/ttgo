package store

import (
	"testing"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// seedOutcome creates one result of caseID with the given status and start time, in its own run.
func seedOutcome(t *testing.T, s *Store, caseID string, status models.ExecutionStatus, start time.Time) *models.RunResult {
	t.Helper()
	run := &models.TestRun{Name: string(status)}
	require.NoError(t, s.CreateTestRun(run))
	rr := &models.RunResult{TestRunID: run.ID, TestCaseID: &caseID, TestNameSnapshot: "t", AttemptNumber: 1,
		Status: status, StartTime: start}
	require.NoError(t, s.AddRunResult(rr))
	return rr
}

func TestListRecentOutcomesByTestCase(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc))
	other := &models.TestCase{Name: "Logout", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(other))

	anchor := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	// Twelve earlier results, oldest first: P F P E S S P F F P E P (PENDING is "S": neither a pass nor a failure).
	seq := []models.ExecutionStatus{models.StatusPass, models.StatusFail, models.StatusPass, models.StatusError,
		models.StatusSkip, models.StatusPending, models.StatusPass, models.StatusFail, models.StatusFail,
		models.StatusPass, models.StatusError, models.StatusPass}
	for i, st := range seq {
		seedOutcome(t, s, tc.ID, st, anchor.Add(-time.Duration(len(seq)-i)*time.Hour))
	}
	seedOutcome(t, s, tc.ID, models.StatusFail, anchor)                                 // the analyzed instant: not earlier
	seedOutcome(t, s, tc.ID, models.StatusFail, anchor.Add(time.Hour))                  // later
	seedOutcome(t, s, other.ID, models.StatusFail, anchor.Add(-30*time.Minute))         // another test
	current := seedOutcome(t, s, tc.ID, models.StatusFail, anchor.Add(-30*time.Minute)) // the analyzed run

	got, err := s.ListRecentOutcomesByTestCase(tc.ID, anchor, current.TestRunID, 10)
	require.NoError(t, err)
	require.Equal(t, "PESSPFFPEP", got, "the ten newest earlier results, oldest first, the analyzed run excluded")

	got, err = s.ListRecentOutcomesByTestCase(tc.ID, anchor, current.TestRunID, 3)
	require.NoError(t, err)
	require.Equal(t, "PEP", got, "the limit keeps the newest")

	got, err = s.ListRecentOutcomesByTestCase(tc.ID, anchor, "", 20)
	require.NoError(t, err)
	require.Equal(t, "PFPESSPFFPEPF", got, "without an excluded run every earlier result counts")

	for _, c := range []struct {
		tcID  string
		limit int
	}{{"", 10}, {tc.ID, 0}, {"no-such-case", 10}} {
		got, err = s.ListRecentOutcomesByTestCase(c.tcID, anchor, "", c.limit)
		require.NoError(t, err)
		require.Empty(t, got, "%+v", c)
	}
}

// Timestamps keep the offset they were written with; the window and the order follow the
// instant. A result recorded without timing counts at the time its row was written.
func TestListRecentOutcomesByTestCase_InstantsAndRowTime(t *testing.T) {
	s := newTestStore(t)
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc))
	eest := time.FixedZone("EEST", 3*3600)
	anchor := time.Date(2026, 9, 24, 10, 0, 0, 0, eest) // 07:00Z

	seedOutcome(t, s, tc.ID, models.StatusPass, time.Date(2026, 9, 24, 8, 30, 0, 0, time.UTC))  // 08:30Z: after, though its text sorts first
	seedOutcome(t, s, tc.ID, models.StatusFail, time.Date(2026, 9, 24, 7, 50, 0, 0, eest))      // 04:50Z
	seedOutcome(t, s, tc.ID, models.StatusError, time.Date(2026, 9, 24, 5, 10, 0, 0, time.UTC)) // 05:10Z
	untimed := seedOutcome(t, s, tc.ID, models.StatusPass, time.Time{})                         // no start time
	require.NoError(t, s.db.Exec("UPDATE run_results SET created_at = ? WHERE id = ?",
		time.Date(2026, 9, 24, 6, 0, 0, 0, time.UTC), untimed.ID).Error) // 06:00Z
	seedOutcome(t, s, tc.ID, models.StatusFail, time.Time{}) // no start time, written now: after the anchor

	got, err := s.ListRecentOutcomesByTestCase(tc.ID, anchor, "", 10)
	require.NoError(t, err)
	require.Equal(t, "FEP", got)
}
