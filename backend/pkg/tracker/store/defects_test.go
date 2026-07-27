package store

import (
	"testing"

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
		require.Len(t, byID, 4)
		assert.Equal(t, "Active Ann", byID[assigned.ID].AssigneeName)
		assert.Equal(t, "Inactive Ivan", byID[deactivated.ID].AssigneeName)
		assert.Equal(t, "nameless@example.com", byID[fallback.ID].AssigneeName)
		assert.Empty(t, byID[unassigned.ID].AssigneeName)
	})

	t.Run("a dangling assignee id yields an empty name rather than an error", func(t *testing.T) {
		orphan := &models.Defect{Title: "eee ghost owner", AssigneeID: strPtr("does-not-exist")}
		require.NoError(t, s.CreateDefect(orphan))
		got, err := s.GetDefect(orphan.ID)
		require.NoError(t, err)
		require.NotNil(t, got.AssigneeID)
		assert.Empty(t, got.AssigneeName)
	})
}
