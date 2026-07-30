package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func TestCountDefectLinksByRunResults(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','x',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO test_runs (id,name) VALUES ('run1','R1')`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO run_results (id,test_run_id,test_case_id,status) VALUES ('rr1','run1','tc1','FAIL')`).Error)

	o := &models.Defect{Title: "open", Status: "open"}
	c := &models.Defect{Title: "closed", Status: "closed"}
	require.NoError(t, s.CreateDefect(o))
	require.NoError(t, s.CreateDefect(c))
	_, _ = s.LinkDefectToResult(o.ID, "rr1", "tc1")
	_, _ = s.LinkDefectToResult(c.ID, "rr1", "tc1")

	open, closed, err := s.CountDefectLinksByRunResults([]string{"rr1"})
	require.NoError(t, err)
	assert.Equal(t, 1, open["rr1"])
	assert.Equal(t, 1, closed["rr1"])
}

// defectCounterFixture builds one folder / test case / run / run result and links three defects to
// it — one "open", one "fixed", one "closed". Shared setup for the four open/closed defect counters.
func defectCounterFixture(t *testing.T, s *Store) (folderID, tcID, runID, resultID string) {
	t.Helper()
	folder, err := s.CreateFolder("Counters", nil)
	require.NoError(t, err)
	tc := &models.TestCase{Name: "Counter Test", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc))

	runID, resultID = "run-counters", "rr-counters"
	require.NoError(t, s.db.Exec(`INSERT INTO test_runs (id,name) VALUES (?,?)`, runID, "Counters Run").Error)
	require.NoError(t, s.db.Exec(`INSERT INTO run_results (id,test_run_id,test_case_id,status) VALUES (?,?,?,'FAIL')`,
		resultID, runID, tc.ID).Error)

	for _, status := range []string{"open", "fixed", "closed"} {
		d := &models.Defect{Title: status + " defect", Status: status}
		require.NoError(t, s.CreateDefect(d))
		_, err := s.LinkDefectToResult(d.ID, resultID, tc.ID)
		require.NoError(t, err)
	}
	return folder.ID, tc.ID, runID, resultID
}

// TestFixedDefectCountsAsOpenInAllCounters pins the four open/closed defect counters against the
// new "fixed" status. Every one of them buckets `closed` vs. everything else rather than comparing
// against "open", so a fixed defect lands in the open bucket and none needed a code change.
func TestFixedDefectCountsAsOpenInAllCounters(t *testing.T) {
	t.Run("CountDefectLinksByRunResults", func(t *testing.T) {
		s := newTestStore(t)
		_, _, _, resultID := defectCounterFixture(t, s)

		open, closed, err := s.CountDefectLinksByRunResults([]string{resultID})
		require.NoError(t, err)
		assert.Equal(t, 2, open[resultID], "open and fixed both count as open")
		assert.Equal(t, 1, closed[resultID])
	})

	// CountDefectLinksByRuns lives in comments.go, not defects.go, and had no test anywhere
	// before this one.
	t.Run("CountDefectLinksByRuns", func(t *testing.T) {
		s := newTestStore(t)
		_, _, runID, _ := defectCounterFixture(t, s)

		open, closed, err := s.CountDefectLinksByRuns([]string{runID})
		require.NoError(t, err)
		assert.Equal(t, 2, open[runID], "open and fixed both count as open")
		assert.Equal(t, 1, closed[runID])
	})

	t.Run("per-test-case counters in ListTestCases", func(t *testing.T) {
		s := newTestStore(t)
		folderID, tcID, _, _ := defectCounterFixture(t, s)

		tests, err := s.ListTestCases(TestCaseFilter{FolderIDs: []string{folderID}})
		require.NoError(t, err)
		require.Len(t, tests, 1)
		require.Equal(t, tcID, tests[0].ID)
		assert.Equal(t, 2, tests[0].OpenDefectCount, "open and fixed both count as open")
		assert.Equal(t, 1, tests[0].ClosedDefectCount)
	})

	t.Run("per-folder counters in GetFolderTree", func(t *testing.T) {
		s := newTestStore(t)
		folderID, tcID, _, _ := defectCounterFixture(t, s)

		tree, err := s.GetFolderTree()
		require.NoError(t, err)
		var found *models.TestCase
		for _, f := range tree {
			if f.ID != folderID {
				continue
			}
			for _, tc := range f.TestCases {
				if tc.ID == tcID {
					found = tc
				}
			}
		}
		require.NotNil(t, found, "the seeded test case must appear in its folder")
		assert.Equal(t, 2, found.OpenDefectCount, "open and fixed both count as open")
		assert.Equal(t, 1, found.ClosedDefectCount)
	})
}

func TestListAffectedTestCases(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','Login works',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc2','Checkout works',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO test_runs (id,name) VALUES ('run1','R1')`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO run_results (id,test_run_id,test_case_id,status) VALUES ('rr1','run1','tc1','FAIL')`).Error)

	d := &models.Defect{Title: "bug"}
	require.NoError(t, s.CreateDefect(d))
	_, err := s.LinkDefectToResult(d.ID, "rr1", "tc1") // result-scoped -> tc1
	require.NoError(t, err)
	_, err = s.LinkDefectToTestCase(d.ID, "tc2") // test-case-scoped -> tc2
	require.NoError(t, err)

	got, err := s.ListAffectedTestCases(d.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "tc2", got[0].ID) // ordered by name: "Checkout works" < "Login works"
	assert.Equal(t, "Checkout works", got[0].Name)
	assert.Equal(t, "tc1", got[1].ID)

	// link-less defect -> empty
	e := &models.Defect{Title: "no links"}
	require.NoError(t, s.CreateDefect(e))
	none, err := s.ListAffectedTestCases(e.ID)
	require.NoError(t, err)
	assert.Empty(t, none)

	// orphaned link (test-case row gone) is excluded by the JOIN
	require.NoError(t, s.db.Exec(`DELETE FROM test_cases WHERE id='tc2'`).Error)
	after, err := s.ListAffectedTestCases(d.ID)
	require.NoError(t, err)
	require.Len(t, after, 1)
	assert.Equal(t, "tc1", after[0].ID)
}

// insertTestCaseRow / insertRunRow / insertResultRow build affected-test-case fixtures directly in
// SQL so start_time and attempt_number can be set explicitly — the "last failing run" ordering must
// be pinned by the recorded execution time, never by insert order.
func insertTestCaseRow(t *testing.T, s *Store, id, name string) {
	t.Helper()
	require.NoError(t, s.db.Exec(
		`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES (?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
		id, name).Error)
}

func insertRunRow(t *testing.T, s *Store, id, name string) {
	t.Helper()
	require.NoError(t, s.db.Exec(`INSERT INTO test_runs (id,name) VALUES (?,?)`, id, name).Error)
}

func insertResultRow(t *testing.T, s *Store, id, runID, tcID, status, startTime string, attempt int) {
	t.Helper()
	require.NoError(t, s.db.Exec(
		`INSERT INTO run_results (id,test_run_id,test_case_id,status,start_time,attempt_number) VALUES (?,?,?,?,?,?)`,
		id, runID, tcID, status, startTime, attempt).Error)
}

// TestListAffectedTestCasesLastFailingRun covers the run enrichment on AffectedTestCase. The run
// reported is the one the TEST CASE last failed in, which is a property of its execution history
// and not of the defect link — case-scoped links carry no run result at all.
func TestListAffectedTestCasesLastFailingRun(t *testing.T) {
	t.Run("populated from the test case's failing result", func(t *testing.T) {
		s := newTestStore(t)
		insertTestCaseRow(t, s, "tc1", "Login works")
		insertRunRow(t, s, "run1", "Nightly regression")
		insertResultRow(t, s, "rr1", "run1", "tc1", "FAIL", "2026-01-01 10:00:00", 1)

		d := &models.Defect{Title: "bug"}
		require.NoError(t, s.CreateDefect(d))
		_, err := s.LinkDefectToResult(d.ID, "rr1", "tc1")
		require.NoError(t, err)

		got, err := s.ListAffectedTestCases(d.ID)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "run1", got[0].LastRunID)
		assert.Equal(t, "Nightly regression", got[0].LastRunName)
		assert.Equal(t, "FAIL", got[0].LastResultStatus)
	})

	t.Run("most recent failure wins across runs", func(t *testing.T) {
		s := newTestStore(t)
		insertTestCaseRow(t, s, "tc1", "Login works")
		insertRunRow(t, s, "run-old", "January run")
		insertRunRow(t, s, "run-new", "March run")
		insertRunRow(t, s, "run-pass", "April run")
		insertResultRow(t, s, "rr-old", "run-old", "tc1", "FAIL", "2026-01-01 10:00:00", 1)
		insertResultRow(t, s, "rr-new", "run-new", "tc1", "ERROR", "2026-03-01 10:00:00", 1)
		// a later PASS is not a failure and must not become the "last failing run"
		insertResultRow(t, s, "rr-pass", "run-pass", "tc1", "PASS", "2026-04-01 10:00:00", 1)

		d := &models.Defect{Title: "bug"}
		require.NoError(t, s.CreateDefect(d))
		// linked from the OLDEST result on purpose: the answer must not be the linked result
		_, err := s.LinkDefectToResult(d.ID, "rr-old", "tc1")
		require.NoError(t, err)

		got, err := s.ListAffectedTestCases(d.ID)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "run-new", got[0].LastRunID)
		assert.Equal(t, "March run", got[0].LastRunName)
		assert.Equal(t, "ERROR", got[0].LastResultStatus, "ERROR is a failure status too")
	})

	t.Run("the later attempt wins inside one run", func(t *testing.T) {
		s := newTestStore(t)
		insertTestCaseRow(t, s, "tc1", "Login works")
		insertRunRow(t, s, "run1", "Retried run")
		insertResultRow(t, s, "rr-a1", "run1", "tc1", "FAIL", "2026-01-01 10:00:00", 1)
		insertResultRow(t, s, "rr-a2", "run1", "tc1", "ERROR", "2026-01-01 10:00:00", 2)

		d := &models.Defect{Title: "bug"}
		require.NoError(t, s.CreateDefect(d))
		_, err := s.LinkDefectToResult(d.ID, "rr-a1", "tc1")
		require.NoError(t, err)

		got, err := s.ListAffectedTestCases(d.ID)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "ERROR", got[0].LastResultStatus, "attempt 2 outranks attempt 1 at the same start_time")
	})

	t.Run("a case-scoped link with no failing result still appears, with empty run fields", func(t *testing.T) {
		s := newTestStore(t)
		insertTestCaseRow(t, s, "tc1", "Never run")

		d := &models.Defect{Title: "filed against the case"}
		require.NoError(t, s.CreateDefect(d))
		_, err := s.LinkDefectToTestCase(d.ID, "tc1")
		require.NoError(t, err)

		got, err := s.ListAffectedTestCases(d.ID)
		require.NoError(t, err)
		require.Len(t, got, 1, "a test case that never failed must not be dropped")
		assert.Equal(t, "tc1", got[0].ID)
		assert.Empty(t, got[0].LastRunID)
		assert.Empty(t, got[0].LastRunName)
		assert.Empty(t, got[0].LastResultStatus)
	})

	t.Run("a test case linked through several results appears once", func(t *testing.T) {
		s := newTestStore(t)
		insertTestCaseRow(t, s, "tc1", "Login works")
		insertRunRow(t, s, "run1", "January run")
		insertRunRow(t, s, "run2", "March run")
		insertResultRow(t, s, "rr1", "run1", "tc1", "FAIL", "2026-01-01 10:00:00", 1)
		insertResultRow(t, s, "rr2", "run2", "tc1", "FAIL", "2026-03-01 10:00:00", 1)

		d := &models.Defect{Title: "recurring bug"}
		require.NoError(t, s.CreateDefect(d))
		_, err := s.LinkDefectToResult(d.ID, "rr1", "tc1")
		require.NoError(t, err)
		_, err = s.LinkDefectToResult(d.ID, "rr2", "tc1")
		require.NoError(t, err)
		// plus a third, case-scoped link to the very same test case
		_, err = s.LinkDefectToTestCase(d.ID, "tc1")
		require.NoError(t, err)

		got, err := s.ListAffectedTestCases(d.ID)
		require.NoError(t, err)
		require.Len(t, got, 1, "three links to one test case must still list it once")
		assert.Equal(t, "run2", got[0].LastRunID)
	})
}

func TestDefectCRUD(t *testing.T) {
	s := newTestStore(t)

	d := &models.Defect{Title: "Checkout 500", Severity: "major"}
	require.NoError(t, s.CreateDefect(d))
	assert.NotEmpty(t, d.ID)
	assert.Equal(t, "open", d.Status)

	got, err := s.GetDefect(d.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Checkout 500", got.Title)

	closed := "closed"
	upd, err := s.UpdateDefect(d.ID, models.UpdateDefectRequest{Status: &closed})
	require.NoError(t, err)
	assert.Equal(t, "closed", upd.Status)

	list, err := s.ListDefects("closed", "", "checkout")
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, s.DeleteDefect(d.ID))
	gone, err := s.GetDefect(d.ID)
	require.NoError(t, err)
	assert.Nil(t, gone)
}

// TestReverificationCountsFixedAsResolved covers the widened recomputeReverification predicate:
// the flag means "every linked defect is fixed-or-closed", so "fixed" resolves a test case just
// like "closed" does.
func TestReverificationCountsFixedAsResolved(t *testing.T) {
	t.Run("marking the only linked defect fixed raises the flag", func(t *testing.T) {
		s := newTestStore(t)
		require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','Login',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)

		d := &models.Defect{Title: "bug"}
		require.NoError(t, s.CreateDefect(d))
		_, err := s.LinkDefectToTestCase(d.ID, "tc1")
		require.NoError(t, err)
		require.False(t, reverFlag(t, s, "tc1"))

		_, err = s.UpdateDefect(d.ID, models.UpdateDefectRequest{Status: strPtr("fixed")})
		require.NoError(t, err)
		assert.True(t, reverFlag(t, s, "tc1"), "a fixed defect leaves the test ready to retest")
	})

	t.Run("one fixed plus one open defect stays unflagged", func(t *testing.T) {
		s := newTestStore(t)
		require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','Login',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)

		fixed := &models.Defect{Title: "fixed one"}
		stillOpen := &models.Defect{Title: "open one"}
		require.NoError(t, s.CreateDefect(fixed))
		require.NoError(t, s.CreateDefect(stillOpen))
		_, err := s.LinkDefectToTestCase(fixed.ID, "tc1")
		require.NoError(t, err)
		_, err = s.LinkDefectToTestCase(stillOpen.ID, "tc1")
		require.NoError(t, err)

		_, err = s.UpdateDefect(fixed.ID, models.UpdateDefectRequest{Status: strPtr("fixed")})
		require.NoError(t, err)
		assert.False(t, reverFlag(t, s, "tc1"), "an unresolved defect keeps the test out of the retest queue")

		// closing the remaining open defect resolves the last one -> flag raised
		_, err = s.UpdateDefect(stillOpen.ID, models.UpdateDefectRequest{Status: strPtr("closed")})
		require.NoError(t, err)
		assert.True(t, reverFlag(t, s, "tc1"))
	})

	t.Run("reopening a fixed defect clears the flag again", func(t *testing.T) {
		s := newTestStore(t)
		require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','Login',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)

		d := &models.Defect{Title: "bug"}
		require.NoError(t, s.CreateDefect(d))
		_, err := s.LinkDefectToTestCase(d.ID, "tc1")
		require.NoError(t, err)

		_, err = s.UpdateDefect(d.ID, models.UpdateDefectRequest{Status: strPtr("fixed")})
		require.NoError(t, err)
		require.True(t, reverFlag(t, s, "tc1"))

		_, err = s.UpdateDefect(d.ID, models.UpdateDefectRequest{Status: strPtr("open")})
		require.NoError(t, err)
		assert.False(t, reverFlag(t, s, "tc1"))
	})

	t.Run("closed still resolves a test case", func(t *testing.T) {
		s := newTestStore(t)
		require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','Login',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)

		d := &models.Defect{Title: "bug"}
		require.NoError(t, s.CreateDefect(d))
		_, err := s.LinkDefectToTestCase(d.ID, "tc1")
		require.NoError(t, err)

		_, err = s.UpdateDefect(d.ID, models.UpdateDefectRequest{Status: strPtr("closed")})
		require.NoError(t, err)
		assert.True(t, reverFlag(t, s, "tc1"))
	})
}

// TestSeedDemoReverificationFlags pins the demo dataset's reverification outcome across the
// widened predicate: no seeded defect is "fixed", so only the closed-defect test case is flagged.
func TestSeedDemoReverificationFlags(t *testing.T) {
	s := newTestStore(t)
	_, err := s.SeedDemoTx(false)
	require.NoError(t, err)

	assert.True(t, reverFlag(t, s, demoID("tc:category-filter")),
		"the only linked defect is closed -> ready to retest")
	assert.False(t, reverFlag(t, s, demoID("tc:session-expires")),
		"linked defect is still open")
	assert.False(t, reverFlag(t, s, demoID("tc:checkout-valid-payment")),
		"linked defect is still open")
}

// newDefectUser inserts a user and forces the active/deleted flags CreateUser does not expose.
func newDefectUser(t *testing.T, s *Store, email, displayName string, active bool) *models.User {
	t.Helper()
	u, err := s.CreateUser(email, displayName, "hash", "member")
	require.NoError(t, err)
	if !active {
		_, err = s.UpdateUser(u.ID, map[string]interface{}{"active": false})
		require.NoError(t, err)
	}
	return u
}

func TestDefectAssigneePersistence(t *testing.T) {
	s := newTestStore(t)
	owner := newDefectUser(t, s, "owner@example.com", "Owner One", true)
	other := newDefectUser(t, s, "other@example.com", "Other Two", true)

	t.Run("create persists an assignee", func(t *testing.T) {
		d := &models.Defect{Title: "assigned on create", AssigneeID: &owner.ID}
		require.NoError(t, s.CreateDefect(d))
		got, err := s.GetDefect(d.ID)
		require.NoError(t, err)
		require.NotNil(t, got.AssigneeID)
		assert.Equal(t, owner.ID, *got.AssigneeID)
	})

	t.Run("create normalises an empty assignee to NULL", func(t *testing.T) {
		d := &models.Defect{Title: "unassigned on create", AssigneeID: strPtr("")}
		require.NoError(t, s.CreateDefect(d))
		assert.Nil(t, d.AssigneeID)
		got, err := s.GetDefect(d.ID)
		require.NoError(t, err)
		assert.Nil(t, got.AssigneeID)
	})

	t.Run("update assigns, reassigns and leaves absent fields unchanged", func(t *testing.T) {
		d := &models.Defect{Title: "reassign me"}
		require.NoError(t, s.CreateDefect(d))

		upd, err := s.UpdateDefect(d.ID, models.UpdateDefectRequest{AssigneeID: &owner.ID})
		require.NoError(t, err)
		require.NotNil(t, upd.AssigneeID)
		assert.Equal(t, owner.ID, *upd.AssigneeID)

		upd, err = s.UpdateDefect(d.ID, models.UpdateDefectRequest{AssigneeID: &other.ID})
		require.NoError(t, err)
		require.NotNil(t, upd.AssigneeID)
		assert.Equal(t, other.ID, *upd.AssigneeID)

		// an absent assignee_id must not disturb the current owner
		upd, err = s.UpdateDefect(d.ID, models.UpdateDefectRequest{Title: strPtr("renamed")})
		require.NoError(t, err)
		assert.Equal(t, "renamed", upd.Title)
		require.NotNil(t, upd.AssigneeID)
		assert.Equal(t, other.ID, *upd.AssigneeID)
	})

	t.Run("update with an empty string clears the assignee", func(t *testing.T) {
		d := &models.Defect{Title: "unassign me", AssigneeID: &owner.ID}
		require.NoError(t, s.CreateDefect(d))

		upd, err := s.UpdateDefect(d.ID, models.UpdateDefectRequest{AssigneeID: strPtr("")})
		require.NoError(t, err)
		assert.Nil(t, upd.AssigneeID)
		assert.Empty(t, upd.AssigneeName)

		got, err := s.GetDefect(d.ID)
		require.NoError(t, err)
		assert.Nil(t, got.AssigneeID)
	})
}

// byDefectID indexes a returned defect slice, whose order (created_at DESC) is not meaningful when
// every row is created in the same instant.
func byDefectID(defects []models.Defect) map[string]models.Defect {
	out := make(map[string]models.Defect, len(defects))
	for _, d := range defects {
		out[d.ID] = d
	}
	return out
}

func TestBulkUpdateDefects(t *testing.T) {
	t.Run("applies several fields at once and touches only the selection", func(t *testing.T) {
		s := newTestStore(t)
		owner := newDefectUser(t, s, "owner@example.com", "Owner One", true)

		a := &models.Defect{Title: "aaa", Severity: "minor"}
		b := &models.Defect{Title: "bbb", Severity: "minor"}
		untouched := &models.Defect{Title: "ccc", Severity: "minor"}
		for _, d := range []*models.Defect{a, b, untouched} {
			require.NoError(t, s.CreateDefect(d))
		}

		got, err := s.BulkUpdateDefects([]string{a.ID, b.ID}, models.BulkUpdateDefectsRequest{
			Status:     strPtr("fixed"),
			Severity:   strPtr("critical"),
			AssigneeID: &owner.ID,
		})
		require.NoError(t, err)
		require.Len(t, got, 2)
		for _, d := range got {
			assert.Equal(t, "fixed", d.Status)
			assert.Equal(t, "critical", d.Severity)
			require.NotNil(t, d.AssigneeID)
			assert.Equal(t, owner.ID, *d.AssigneeID)
			assert.Equal(t, "Owner One", d.AssigneeName, "returned rows carry the resolved name")
		}

		still, err := s.GetDefect(untouched.ID)
		require.NoError(t, err)
		assert.Equal(t, "open", still.Status)
		assert.Equal(t, "minor", still.Severity)
		assert.Nil(t, still.AssigneeID)
	})

	t.Run("unknown ids are skipped without an error", func(t *testing.T) {
		s := newTestStore(t)
		d := &models.Defect{Title: "real one"}
		require.NoError(t, s.CreateDefect(d))

		got, err := s.BulkUpdateDefects([]string{d.ID, "no-such-defect"},
			models.BulkUpdateDefectsRequest{Status: strPtr("closed")})
		require.NoError(t, err)
		require.Len(t, got, 1, "only the defects that exist come back")
		assert.Equal(t, d.ID, got[0].ID)
		assert.Equal(t, "closed", got[0].Status)
	})

	t.Run("an empty id list is a no-op", func(t *testing.T) {
		s := newTestStore(t)
		d := &models.Defect{Title: "leave me alone"}
		require.NoError(t, s.CreateDefect(d))

		got, err := s.BulkUpdateDefects(nil, models.BulkUpdateDefectsRequest{Status: strPtr("closed")})
		require.NoError(t, err)
		assert.Empty(t, got)

		after, err := s.GetDefect(d.ID)
		require.NoError(t, err)
		assert.Equal(t, "open", after.Status, "an empty selection must not become a global update")
	})

	t.Run("an empty assignee clears the owner across the selection", func(t *testing.T) {
		s := newTestStore(t)
		owner := newDefectUser(t, s, "owner@example.com", "Owner One", true)

		a := &models.Defect{Title: "aaa", AssigneeID: &owner.ID}
		b := &models.Defect{Title: "bbb", AssigneeID: &owner.ID}
		require.NoError(t, s.CreateDefect(a))
		require.NoError(t, s.CreateDefect(b))

		got, err := s.BulkUpdateDefects([]string{a.ID, b.ID},
			models.BulkUpdateDefectsRequest{AssigneeID: strPtr("")})
		require.NoError(t, err)
		require.Len(t, got, 2)
		for _, d := range got {
			assert.Nil(t, d.AssigneeID)
			assert.Empty(t, d.AssigneeName)
		}
	})

	t.Run("an absent field leaves the stored value unchanged", func(t *testing.T) {
		s := newTestStore(t)
		owner := newDefectUser(t, s, "owner@example.com", "Owner One", true)
		d := &models.Defect{Title: "keep my owner", Severity: "major", AssigneeID: &owner.ID}
		require.NoError(t, s.CreateDefect(d))

		got, err := s.BulkUpdateDefects([]string{d.ID},
			models.BulkUpdateDefectsRequest{Status: strPtr("closed")})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "closed", got[0].Status)
		assert.Equal(t, "major", got[0].Severity)
		require.NotNil(t, got[0].AssigneeID, "a nil assignee_id means unchanged, not unassign")
		assert.Equal(t, owner.ID, *got[0].AssigneeID)
	})

	t.Run("a request setting no field does not bump updated_at", func(t *testing.T) {
		s := newTestStore(t)
		stale := &models.Defect{Title: "nothing to do"}
		fresh := &models.Defect{Title: "really changed"}
		require.NoError(t, s.CreateDefect(stale))
		require.NoError(t, s.CreateDefect(fresh))
		// back-date both so "was it bumped?" does not depend on clock resolution
		old := time.Now().Add(-30 * 24 * time.Hour)
		require.NoError(t, s.db.Model(&models.Defect{}).Where("id IN ?", []string{stale.ID, fresh.ID}).
			Update("updated_at", old).Error)

		got, err := s.BulkUpdateDefects([]string{stale.ID}, models.BulkUpdateDefectsRequest{})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.WithinDuration(t, old, got[0].UpdatedAt, time.Second,
			"updated_at is the staleness signal — a request with nothing to write must not reset it")

		// a request that does write a field bumps it, so the guard above is not vacuous
		got, err = s.BulkUpdateDefects([]string{fresh.ID},
			models.BulkUpdateDefectsRequest{Severity: strPtr("critical")})
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.WithinDuration(t, time.Now(), got[0].UpdatedAt, time.Minute)
	})

	t.Run("returned rows carry a correct linked_test_count", func(t *testing.T) {
		s := newTestStore(t)
		require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','Login',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
		require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc2','Checkout',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)

		two := &models.Defect{Title: "hits two tests"}
		one := &models.Defect{Title: "hits one test"}
		none := &models.Defect{Title: "hits nothing"}
		for _, d := range []*models.Defect{two, one, none} {
			require.NoError(t, s.CreateDefect(d))
		}
		for _, tc := range []string{"tc1", "tc2"} {
			_, err := s.LinkDefectToTestCase(two.ID, tc)
			require.NoError(t, err)
		}
		_, err := s.LinkDefectToTestCase(one.ID, "tc1")
		require.NoError(t, err)

		got, err := s.BulkUpdateDefects([]string{two.ID, one.ID, none.ID},
			models.BulkUpdateDefectsRequest{Severity: strPtr("major")})
		require.NoError(t, err)
		require.Len(t, got, 3)
		rows := byDefectID(got)
		assert.Equal(t, 2, rows[two.ID].LinkedTestCount)
		assert.Equal(t, 1, rows[one.ID].LinkedTestCount)
		assert.Equal(t, 0, rows[none.ID].LinkedTestCount)
	})
}

// TestBulkUpdateDefectsReverificationParity pins the requirement that closing N defects in one bulk
// call leaves exactly the reverification state that closing them one at a time through UpdateDefect
// would. "Mark verified & close" is the main path to closed, so a divergence here would be silent.
func TestBulkUpdateDefectsReverificationParity(t *testing.T) {
	// seedReverificationFixture builds two test cases that each have one linked defect plus a third
	// test case whose second defect stays open, and returns the defect ids in a stable order.
	seedReverificationFixture := func(t *testing.T, s *Store) []string {
		t.Helper()
		for _, tc := range []string{"tc1", "tc2", "tc3"} {
			require.NoError(t, s.db.Exec(
				`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES (?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`,
				tc, tc).Error)
		}
		var ids []string
		for i, tc := range []string{"tc1", "tc2", "tc3"} {
			d := &models.Defect{Title: "bug " + tc}
			require.NoError(t, s.CreateDefect(d))
			_, err := s.LinkDefectToTestCase(d.ID, tc)
			require.NoError(t, err)
			ids = append(ids, d.ID)
			if i == 2 {
				// tc3 keeps a second, untouched defect so it must stay unflagged
				extra := &models.Defect{Title: "still open on " + tc}
				require.NoError(t, s.CreateDefect(extra))
				_, err := s.LinkDefectToTestCase(extra.ID, tc)
				require.NoError(t, err)
			}
		}
		return ids
	}

	bulk := newTestStore(t)
	bulkIDs := seedReverificationFixture(t, bulk)
	_, err := bulk.BulkUpdateDefects(bulkIDs, models.BulkUpdateDefectsRequest{Status: strPtr("closed")})
	require.NoError(t, err)

	single := newTestStore(t)
	singleIDs := seedReverificationFixture(t, single)
	for _, id := range singleIDs {
		_, err := single.UpdateDefect(id, models.UpdateDefectRequest{Status: strPtr("closed")})
		require.NoError(t, err)
	}

	for _, tc := range []string{"tc1", "tc2", "tc3"} {
		assert.Equal(t, reverFlag(t, single, tc), reverFlag(t, bulk, tc),
			"bulk close must flag %s exactly as N single closes do", tc)
	}
	assert.True(t, reverFlag(t, bulk, "tc1"))
	assert.True(t, reverFlag(t, bulk, "tc2"))
	assert.False(t, reverFlag(t, bulk, "tc3"), "tc3 still has an open defect")

	// marking fixed rather than closed is equally a status change, so it recomputes too
	t.Run("a bulk fixed also raises the flag", func(t *testing.T) {
		s := newTestStore(t)
		ids := seedReverificationFixture(t, s)
		_, err := s.BulkUpdateDefects(ids, models.BulkUpdateDefectsRequest{Status: strPtr("fixed")})
		require.NoError(t, err)
		assert.True(t, reverFlag(t, s, "tc1"))
		assert.False(t, reverFlag(t, s, "tc3"))
	})

	// a bulk call that changes no status must not recompute anything
	t.Run("a severity-only bulk leaves the flags alone", func(t *testing.T) {
		s := newTestStore(t)
		ids := seedReverificationFixture(t, s)
		_, err := s.BulkUpdateDefects(ids, models.BulkUpdateDefectsRequest{Severity: strPtr("critical")})
		require.NoError(t, err)
		assert.False(t, reverFlag(t, s, "tc1"))
	})
}

// TestAffectedTestCaseLookupFailureAborts pins that a failed affectedTestCaseIDs lookup is never
// swallowed. The list it returns is the INPUT to recomputeReverification, so a swallowed error is
// indistinguishable from "no test cases are affected": the recompute no-ops on the empty slice and
// the caller commits a status change, returns success, and leaves every affected test case's
// reverification_flagged silently stale. Dropping defect_links is what makes that lookup — and only
// that lookup — fail on demand.
func TestAffectedTestCaseLookupFailureAborts(t *testing.T) {
	// seedLinkedDefect builds one test case with one defect linked to it, then breaks the table
	// the link lives in. Seeding happens first, so the fixture is real before the query can fail.
	seedLinkedDefect := func(t *testing.T, s *Store) *models.Defect {
		t.Helper()
		require.NoError(t, s.db.Exec(
			`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','Login',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
		d := &models.Defect{Title: "linked bug"}
		require.NoError(t, s.CreateDefect(d))
		_, err := s.LinkDefectToTestCase(d.ID, "tc1")
		require.NoError(t, err)
		require.NoError(t, s.db.Exec(`DROP TABLE defect_links`).Error)
		return d
	}

	// The one that was reported: a bulk close committed and reported success while the retest
	// flags it is supposed to raise were never recomputed.
	t.Run("BulkUpdateDefects rolls the status change back", func(t *testing.T) {
		s := newTestStore(t)
		d := seedLinkedDefect(t, s)

		_, err := s.BulkUpdateDefects([]string{d.ID}, models.BulkUpdateDefectsRequest{Status: strPtr("closed")})
		require.Error(t, err, "a failed lookup must not commit as a success")

		after, err := s.GetDefect(d.ID)
		require.NoError(t, err)
		assert.Equal(t, "open", after.Status, "the whole transaction rolls back, status included")
	})

	// Not transactional by design (see the comment at the call site): the UPDATE has already
	// committed by then, so what this pins is that the failure is REPORTED rather than hidden.
	t.Run("UpdateDefect reports the failure", func(t *testing.T) {
		s := newTestStore(t)
		d := seedLinkedDefect(t, s)

		_, err := s.UpdateDefect(d.ID, models.UpdateDefectRequest{Status: strPtr("closed")})
		require.Error(t, err)
	})

	// The links are the only record of which test cases need recomputing, so deleting them
	// without that list in hand is unrecoverable — the delete aborts instead. This one held
	// before the fix too (the link DELETE that follows fails against the same dropped table,
	// which rolled the transaction back anyway); what it pins is the ORDERING that makes the
	// abort reachable at all — the lookup has to run, and be checked, before the links go.
	t.Run("DeleteDefect aborts and keeps the defect", func(t *testing.T) {
		s := newTestStore(t)
		d := seedLinkedDefect(t, s)

		require.Error(t, s.DeleteDefect(d.ID))

		after, err := s.GetDefect(d.ID)
		require.NoError(t, err)
		require.NotNil(t, after, "an aborted delete leaves the defect in place")
	})

	// That the abort belongs to the status path alone — a bulk that changes no status never asks
	// the question and recomputes nothing — is pinned by TestBulkUpdateDefectsReverificationParity's
	// "a severity-only bulk leaves the flags alone", against an intact database. It cannot be
	// asserted here: the enrichment every bulk response carries reads defect_links too, so with
	// the table dropped there is no such thing as a bulk call that succeeds.
}

func TestDefectAssigneeNamePopulation(t *testing.T) {
	s := newTestStore(t)
	active := newDefectUser(t, s, "active@example.com", "Active Ann", true)
	inactive := newDefectUser(t, s, "gone@example.com", "Inactive Ivan", false)
	nameless := newDefectUser(t, s, "nameless@example.com", "", true)

	assigned := &models.Defect{Title: "aaa active owner", AssigneeID: &active.ID}
	deactivated := &models.Defect{Title: "bbb inactive owner", AssigneeID: &inactive.ID}
	fallback := &models.Defect{Title: "ccc nameless owner", AssigneeID: &nameless.ID}
	unassigned := &models.Defect{Title: "ddd no owner"}
	for _, d := range []*models.Defect{assigned, deactivated, fallback, unassigned} {
		require.NoError(t, s.CreateDefect(d))
	}

	t.Run("GetDefect resolves the display name", func(t *testing.T) {
		got, err := s.GetDefect(assigned.ID)
		require.NoError(t, err)
		assert.Equal(t, "Active Ann", got.AssigneeName)
	})

	t.Run("a deactivated user still resolves", func(t *testing.T) {
		got, err := s.GetDefect(deactivated.ID)
		require.NoError(t, err)
		assert.Equal(t, "Inactive Ivan", got.AssigneeName)
	})

	t.Run("a blank display name falls back to email", func(t *testing.T) {
		got, err := s.GetDefect(fallback.ID)
		require.NoError(t, err)
		assert.Equal(t, "nameless@example.com", got.AssigneeName)
	})

	t.Run("an unassigned defect has an empty name", func(t *testing.T) {
		got, err := s.GetDefect(unassigned.ID)
		require.NoError(t, err)
		assert.Nil(t, got.AssigneeID)
		assert.Empty(t, got.AssigneeName)
	})

	t.Run("ListDefects populates every row in one batch", func(t *testing.T) {
		list, err := s.ListDefects("", "", "")
		require.NoError(t, err)
		byID := make(map[string]models.Defect, len(list))
		for _, d := range list {
			byID[d.ID] = d
		}
		// The four seeded above, and no assertion on the total: a sibling subtest
		// creating a fifth defect in this shared store must not fail this one.
		require.Len(t, byID, 4)
		assert.Equal(t, "Active Ann", byID[assigned.ID].AssigneeName)
		assert.Equal(t, "Inactive Ivan", byID[deactivated.ID].AssigneeName)
		assert.Equal(t, "nameless@example.com", byID[fallback.ID].AssigneeName)
		assert.Empty(t, byID[unassigned.ID].AssigneeName)
	})

	// Its own store: this one WRITES, and the subtest above hard-asserts the row count
	// of the store it shares with its siblings.
	t.Run("a dangling assignee id yields an empty name rather than an error", func(t *testing.T) {
		own := newTestStore(t)
		orphan := &models.Defect{Title: "eee ghost owner", AssigneeID: strPtr("does-not-exist")}
		require.NoError(t, own.CreateDefect(orphan))
		got, err := own.GetDefect(orphan.ID)
		require.NoError(t, err)
		require.NotNil(t, got.AssigneeID)
		assert.Empty(t, got.AssigneeName)
	})

	// The gap that let POST /defects ship an empty assignee_name: only the read paths
	// resolved it, so a defect created with an owner came back with assignee_id set and
	// assignee_name blank — which the register renders as "Unknown user", the label that
	// is supposed to mean the account is gone.
	t.Run("CreateDefect resolves the name on the row it hands back", func(t *testing.T) {
		own := newTestStore(t)
		owner := newDefectUser(t, own, "fresh@example.com", "Fresh Owner", true)
		d := &models.Defect{Title: "created with an owner", AssigneeID: &owner.ID}
		require.NoError(t, own.CreateDefect(d))
		assert.Equal(t, "Fresh Owner", d.AssigneeName)
	})
}
