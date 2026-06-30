package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func TestLinkAndReverification(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','Login',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO test_runs (id,name) VALUES ('run1','R1')`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO run_results (id,test_run_id,test_case_id,status) VALUES ('rr1','run1','tc1','FAIL')`).Error)

	d := &models.Defect{Title: "bug"}
	require.NoError(t, s.CreateDefect(d))

	link, err := s.LinkDefectToResult(d.ID, "rr1", "tc1")
	require.NoError(t, err)
	assert.Equal(t, "tc1", *link.TestCaseID)

	_, err = s.LinkDefectToResult(d.ID, "rr1", "tc1")
	assert.ErrorIs(t, err, models.ErrDuplicateDefectLink)

	defs, err := s.ListDefectsByResult("rr1")
	require.NoError(t, err)
	assert.Len(t, defs, 1)
	assert.False(t, reverFlag(t, s, "tc1"))

	closed := "closed"
	_, err = s.UpdateDefect(d.ID, models.UpdateDefectRequest{Status: &closed})
	require.NoError(t, err)
	assert.True(t, reverFlag(t, s, "tc1"))

	open := "open"
	_, err = s.UpdateDefect(d.ID, models.UpdateDefectRequest{Status: &open})
	require.NoError(t, err)
	assert.False(t, reverFlag(t, s, "tc1"))

	// ListDefects should show LinkedTestCount == 1 for the linked defect (Task 2 CRUD gap).
	all, err := s.ListDefects("", "", "")
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, 1, all[0].LinkedTestCount)
}

func TestLinkDefectToTestCase(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','Login',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)

	d := &models.Defect{Title: "case-bug"}
	require.NoError(t, s.CreateDefect(d))

	// Link a defect to a test case (case-scoped, run_result_id must be NULL)
	link, err := s.LinkDefectToTestCase(d.ID, "tc1")
	require.NoError(t, err)
	assert.Equal(t, "tc1", *link.TestCaseID)
	assert.Nil(t, link.RunResultID)

	// Appears in ListDefectsByTestCase
	defs, err := s.ListDefectsByTestCase("tc1")
	require.NoError(t, err)
	require.Len(t, defs, 1)
	assert.Equal(t, d.ID, defs[0].ID)

	// Duplicate returns ErrDuplicateDefectLink
	_, err = s.LinkDefectToTestCase(d.ID, "tc1")
	assert.ErrorIs(t, err, models.ErrDuplicateDefectLink)
}

func TestUnlinkDefectFromResult(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','Login',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO test_runs (id,name) VALUES ('run1','R1')`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO run_results (id,test_run_id,test_case_id,status) VALUES ('rr1','run1','tc1','FAIL')`).Error)

	d := &models.Defect{Title: "bug"}
	require.NoError(t, s.CreateDefect(d))

	// Close the defect so reverification fires
	closed := "closed"
	_, err := s.UpdateDefect(d.ID, models.UpdateDefectRequest{Status: &closed})
	require.NoError(t, err)

	_, err = s.LinkDefectToResult(d.ID, "rr1", "tc1")
	require.NoError(t, err)
	// After linking a closed defect, flag should be true
	assert.True(t, reverFlag(t, s, "tc1"))

	// Unlink: link should be gone and reverification recomputed to false
	require.NoError(t, s.UnlinkDefectFromResult(d.ID, "rr1"))
	defs, err := s.ListDefectsByResult("rr1")
	require.NoError(t, err)
	assert.Empty(t, defs)
	assert.False(t, reverFlag(t, s, "tc1"))

	// Unlinking a non-existent link returns error
	err = s.UnlinkDefectFromResult(d.ID, "rr1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "defect link not found")
}

func TestUnlinkDefectFromTestCase(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','Login',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)

	d := &models.Defect{Title: "case-bug"}
	require.NoError(t, s.CreateDefect(d))

	// Close the defect and link it case-scoped
	closed := "closed"
	_, err := s.UpdateDefect(d.ID, models.UpdateDefectRequest{Status: &closed})
	require.NoError(t, err)

	_, err = s.LinkDefectToTestCase(d.ID, "tc1")
	require.NoError(t, err)
	assert.True(t, reverFlag(t, s, "tc1"))

	// Unlink: link gone and reverification recomputed
	require.NoError(t, s.UnlinkDefectFromTestCase(d.ID, "tc1"))
	defs, err := s.ListDefectsByTestCase("tc1")
	require.NoError(t, err)
	assert.Empty(t, defs)
	assert.False(t, reverFlag(t, s, "tc1"))

	// Unlinking a non-existent link returns error
	err = s.UnlinkDefectFromTestCase(d.ID, "tc1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "defect link not found")
}

// TestTestCaseDeletionNullOut verifies the tests.go deletion behaviour:
//   - result-scoped link (run_result_id + test_case_id): link survives but test_case_id becomes NULL
//   - case-scoped link (run_result_id NULL): deleted entirely
func TestTestCaseDeletionNullOut(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','Login',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO test_runs (id,name) VALUES ('run1','R1')`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO run_results (id,test_run_id,test_case_id,status) VALUES ('rr1','run1','tc1','FAIL')`).Error)

	d := &models.Defect{Title: "bug"}
	require.NoError(t, s.CreateDefect(d))

	// Result-scoped link (run_result_id + test_case_id both set)
	resultLink, err := s.LinkDefectToResult(d.ID, "rr1", "tc1")
	require.NoError(t, err)

	// Case-scoped link (run_result_id NULL)
	_, err = s.LinkDefectToTestCase(d.ID, "tc1")
	require.NoError(t, err)

	// Delete the test case
	require.NoError(t, s.DeleteTestCase("tc1"))

	// Result-scoped link still exists but test_case_id is now NULL
	var link models.DefectLink
	err = s.db.First(&link, "id = ?", resultLink.ID).Error
	require.NoError(t, err, "result-scoped link should still exist")
	assert.Nil(t, link.TestCaseID, "test_case_id should be NULL after test case deletion")
	assert.NotNil(t, link.RunResultID, "run_result_id should still be set")

	// Case-scoped link is deleted
	var count int64
	s.db.Model(&models.DefectLink{}).Where("defect_id = ? AND run_result_id IS NULL", d.ID).Count(&count)
	assert.Equal(t, int64(0), count, "case-scoped link should be deleted when test case is deleted")
}

// TestRunResultDeletionReverification verifies that deleting a run result:
//   - removes the result-scoped defect link
//   - recomputes reverification_flagged to false (no remaining links)
func TestRunResultDeletionReverification(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','Login',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO test_runs (id,name) VALUES ('run1','R1')`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO run_results (id,test_run_id,test_case_id,status) VALUES ('rr1','run1','tc1','FAIL')`).Error)

	d := &models.Defect{Title: "bug"}
	require.NoError(t, s.CreateDefect(d))

	// Close the defect so linking it will set reverification_flagged = true
	closed := "closed"
	_, err := s.UpdateDefect(d.ID, models.UpdateDefectRequest{Status: &closed})
	require.NoError(t, err)

	_, err = s.LinkDefectToResult(d.ID, "rr1", "tc1")
	require.NoError(t, err)
	// Confirm the flag is set
	assert.True(t, reverFlag(t, s, "tc1"))

	// Delete the run result
	require.NoError(t, s.DeleteRunResult("run1", "rr1"))

	// Link should be gone
	defs, err := s.ListDefectsByResult("rr1")
	require.NoError(t, err)
	assert.Empty(t, defs)

	// reverification_flagged should be recomputed to false (0 linked defects remain)
	assert.False(t, reverFlag(t, s, "tc1"))
}

func reverFlag(t *testing.T, s *Store, tcID string) bool {
	t.Helper()
	var flagged bool
	require.NoError(t, s.db.Raw(`SELECT reverification_flagged FROM test_cases WHERE id = ?`, tcID).Scan(&flagged).Error)
	return flagged
}
