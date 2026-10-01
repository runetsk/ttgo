package store

import (
	"context"
	"fmt"
	"testing"

	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// sqliteLimitVariableNumber is SQLITE_LIMIT_VARIABLE_NUMBER from sqlite3.h: the sqlite3_limit()
// id for how many ?-parameters a single statement may bind.
const sqliteLimitVariableNumber = 9

// testBindLimit is the per-statement variable cap the tests below run under. Every fixture seeds
// more rows than this, so a statement that binds one variable per row fails exactly the way
// production did once a delete reached the compiled-in 32766.
const testBindLimit = 16

// lowerBindLimit caps how many SQL variables one statement may bind on the test store's
// connection, so a few dozen rows reproduce what tens of thousands do against the compiled-in
// limit, and scales idChunkSize down with it the way production's 5000 sits under 32766. Call it
// after seeding (a multi-row INSERT binds a variable per column per row). It pins the pool to
// that one connection and proves the cap took effect, so a test built on it cannot pass
// vacuously.
func lowerBindLimit(t *testing.T, s *Store) {
	t.Helper()
	prevChunk := idChunkSize
	idChunkSize = testBindLimit / 2
	t.Cleanup(func() { idChunkSize = prevChunk })

	sqlDB, err := s.db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	withDriverConn(t, s, func(l sqliteLimits) { l.SetLimit(sqliteLimitVariableNumber, testBindLimit) })

	var n int
	quiet := s.db.Session(&gorm.Session{Logger: logger.Discard})
	err = quiet.Raw("SELECT COUNT(*) WHERE 1 IN ?", make([]int, testBindLimit+1)).Scan(&n).Error
	require.ErrorContains(t, err, "too many SQL variables", "the lowered limit must govern the store's connection")
}

// sqliteLimits is the part of mattn/go-sqlite3's *SQLiteConn the tests use, asserted on the raw
// driver connection so the test needn't import the driver.
type sqliteLimits interface {
	GetLimit(id int) int
	SetLimit(id, newVal int) int
}

// withDriverConn runs fn on the store's underlying SQLite connection.
func withDriverConn(t *testing.T, s *Store, fn func(sqliteLimits)) {
	t.Helper()
	sqlDB, err := s.db.DB()
	require.NoError(t, err)
	conn, err := sqlDB.Conn(context.Background())
	require.NoError(t, err)
	require.NoError(t, conn.Raw(func(dc any) error {
		l, ok := dc.(sqliteLimits)
		if !ok {
			return fmt.Errorf("driver connection %T cannot read or set limits", dc)
		}
		fn(l)
		return nil
	}))
	require.NoError(t, conn.Close())
}

// liveBindLimit is the variable cap the store's connection enforces (the compiled-in 32766 unless
// a test lowered it).
func liveBindLimit(t *testing.T, s *Store) int {
	t.Helper()
	var limit int
	withDriverConn(t, s, func(l sqliteLimits) { limit = l.GetLimit(sqliteLimitVariableNumber) })
	require.Positive(t, limit)
	return limit
}

// columnCount is how many columns table has: the most variables one row of a multi-row INSERT
// into it can bind.
func columnCount(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	require.NoError(t, s.db.Raw(`SELECT COUNT(*) FROM pragma_table_info(?)`, table).Scan(&n).Error)
	require.Positive(t, n, "no table %q", table)
	return n
}

// rowsOverflowingOneInsert is a row count of table that one multi-row INSERT cannot bind under
// the live limit. It is twice the limit over the column count because GORM leaves default-valued
// columns it was not given out of the INSERT, so a row binds somewhat fewer variables than the
// table has columns. Derived rather than hard-coded, so the tests keep reproducing the overflow as
// the table gains or loses columns.
func rowsOverflowingOneInsert(t *testing.T, s *Store, table string) int {
	t.Helper()
	return 2 * liveBindLimit(t, s) / columnCount(t, s, table)
}

// runFixture is a set of runs whose results carry everything a run delete must clean up:
// a result comment, an AI analysis and a defect link per result, plus a run comment and an
// analysis job per run.
type runFixture struct {
	runIDs    []string
	resultIDs []string
	testCases []string
}

// seedRunsWithResults creates runs×perResults results, each on its own test case and each
// linked to the closed defect, so every test case starts reverification-flagged.
func seedRunsWithResults(t *testing.T, s *Store, prefix string, runs, perRun int, defectID string) runFixture {
	t.Helper()
	var f runFixture
	for r := 0; r < runs; r++ {
		runID := fmt.Sprintf("%s-run%d", prefix, r)
		require.NoError(t, s.db.Exec(`INSERT INTO test_runs (id,name) VALUES (?,?)`, runID, runID).Error)
		require.NoError(t, s.db.Exec(`INSERT INTO comments (id,target_type,target_id,content,created_at,updated_at)
			VALUES (?, 'run', ?, 'run note', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, runID+"-c", runID).Error)
		require.NoError(t, s.db.Create(&models.RunAnalysisJob{
			ID: runID + "-job", TestRunID: runID, Trigger: "manual", Status: "completed",
		}).Error)
		f.runIDs = append(f.runIDs, runID)
		for i := 0; i < perRun; i++ {
			tcID := fmt.Sprintf("%s-tc%d-%d", prefix, r, i)
			rrID := fmt.Sprintf("%s-rr%d-%d", prefix, r, i)
			require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at)
				VALUES (?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, tcID, tcID).Error)
			require.NoError(t, s.db.Exec(`INSERT INTO run_results (id,test_run_id,test_case_id,status)
				VALUES (?,?,?,'FAIL')`, rrID, runID, tcID).Error)
			require.NoError(t, s.db.Exec(`INSERT INTO comments (id,target_type,target_id,content,created_at,updated_at)
				VALUES (?, 'result', ?, 'result note', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, rrID+"-c", rrID).Error)
			require.NoError(t, s.db.Create(&models.RunResultAnalysis{
				ID: rrID + "-a", RunResultID: rrID, Version: 1,
				Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh,
			}).Error)
			_, err := s.LinkDefectToResult(defectID, rrID, tcID)
			require.NoError(t, err)
			f.resultIDs = append(f.resultIDs, rrID)
			f.testCases = append(f.testCases, tcID)
		}
	}
	return f
}

func closedDefect(t *testing.T, s *Store) string {
	t.Helper()
	d := &models.Defect{Title: "bug"}
	require.NoError(t, s.CreateDefect(d))
	closed := "closed"
	_, err := s.UpdateDefect(d.ID, models.UpdateDefectRequest{Status: &closed})
	require.NoError(t, err)
	return d.ID
}

// countWhere counts the rows of model matching query.
func countWhere(t *testing.T, s *Store, model interface{}, query string, args ...interface{}) int64 {
	t.Helper()
	var n int64
	require.NoError(t, s.db.Model(model).Where(query, args...).Count(&n).Error)
	return n
}

// assertRunsGone checks that every row the delete owns is gone and every test case the deleted
// links pointed at had its reverification flag recomputed (no links left → cleared).
func assertRunsGone(t *testing.T, s *Store, f runFixture) {
	t.Helper()
	for _, runID := range f.runIDs {
		assert.Zero(t, countWhere(t, s, &models.TestRun{}, "id = ?", runID), "run %s", runID)
		assert.Zero(t, countWhere(t, s, &models.RunResult{}, "test_run_id = ?", runID), "results of %s", runID)
		assert.Zero(t, countWhere(t, s, &models.Comment{}, "target_type = 'run' AND target_id = ?", runID), "run comment of %s", runID)
		assert.Zero(t, countWhere(t, s, &models.RunAnalysisJob{}, "test_run_id = ?", runID), "analysis job of %s", runID)
	}
	for _, rrID := range f.resultIDs {
		assert.Zero(t, countWhere(t, s, &models.Comment{}, "target_type = 'result' AND target_id = ?", rrID), "comment of %s", rrID)
		assert.Zero(t, countWhere(t, s, &models.DefectLink{}, "run_result_id = ?", rrID), "defect link of %s", rrID)
		assert.Zero(t, countWhere(t, s, &models.RunResultAnalysis{}, "run_result_id = ?", rrID), "analysis of %s", rrID)
	}
	for _, tcID := range f.testCases {
		assert.False(t, reverFlag(t, s, tcID), "test case %s lost its last link and must be un-flagged", tcID)
	}
}

// assertRunsKept checks that a run outside the delete is untouched, links and flags included.
func assertRunsKept(t *testing.T, s *Store, f runFixture) {
	t.Helper()
	for _, runID := range f.runIDs {
		assert.EqualValues(t, 1, countWhere(t, s, &models.TestRun{}, "id = ?", runID), "run %s", runID)
		assert.EqualValues(t, 1, countWhere(t, s, &models.RunAnalysisJob{}, "test_run_id = ?", runID), "analysis job of %s", runID)
	}
	for _, rrID := range f.resultIDs {
		assert.EqualValues(t, 1, countWhere(t, s, &models.RunResult{}, "id = ?", rrID), "result %s", rrID)
		assert.EqualValues(t, 1, countWhere(t, s, &models.Comment{}, "target_type = 'result' AND target_id = ?", rrID), "comment of %s", rrID)
		assert.EqualValues(t, 1, countWhere(t, s, &models.DefectLink{}, "run_result_id = ?", rrID), "defect link of %s", rrID)
		assert.EqualValues(t, 1, countWhere(t, s, &models.RunResultAnalysis{}, "run_result_id = ?", rrID), "analysis of %s", rrID)
	}
	for _, tcID := range f.testCases {
		assert.True(t, reverFlag(t, s, tcID), "test case %s still has its link and stays flagged", tcID)
	}
}

// TestDeleteTestRunsWithMoreResultsThanBindLimit reproduces the bulk-delete 500: the results of
// the selected runs were plucked and bound one variable each into IN clauses, so 100 runs of
// ~1,500 results failed with "too many SQL variables".
func TestDeleteTestRunsWithMoreResultsThanBindLimit(t *testing.T) {
	s := newTestStore(t)
	defectID := closedDefect(t, s)
	doomed := seedRunsWithResults(t, s, "del", 3, testBindLimit, defectID)
	kept := seedRunsWithResults(t, s, "keep", 1, 2, defectID)
	lowerBindLimit(t, s)

	require.NoError(t, s.DeleteTestRuns(doomed.runIDs))

	assertRunsGone(t, s, doomed)
	assertRunsKept(t, s, kept)
}

// TestDeleteTestRunWithMoreResultsThanBindLimit is the single-run delete: one run can hold more
// results than SQLite binds in a statement just as a bulk selection can.
func TestDeleteTestRunWithMoreResultsThanBindLimit(t *testing.T) {
	s := newTestStore(t)
	defectID := closedDefect(t, s)
	doomed := seedRunsWithResults(t, s, "del", 1, 2*testBindLimit, defectID)
	kept := seedRunsWithResults(t, s, "keep", 1, 2, defectID)
	lowerBindLimit(t, s)

	require.NoError(t, s.DeleteTestRun(doomed.runIDs[0]))

	assertRunsGone(t, s, doomed)
	assertRunsKept(t, s, kept)
}

// TestDeleteFolderWithMoreTestCasesThanBindLimit covers the folder cascade: every test case in
// the folder subtree was plucked and bound into the test-case delete's IN clauses.
func TestDeleteFolderWithMoreTestCasesThanBindLimit(t *testing.T) {
	s := newTestStore(t)
	defectID := closedDefect(t, s)
	doomed, err := s.CreateFolder("Doomed", nil)
	require.NoError(t, err)
	sub, err := s.CreateFolder("Sub", &doomed.ID)
	require.NoError(t, err)
	kept, err := s.CreateFolder("Kept", nil)
	require.NoError(t, err)
	require.NoError(t, s.db.Exec(`INSERT INTO test_runs (id,name) VALUES ('run1','R1')`).Error)

	seedCase := func(id, folderID string) {
		require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,folder_id,created_at,updated_at)
			VALUES (?,?,?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`, id, id, folderID).Error)
		require.NoError(t, s.db.Exec(`INSERT INTO run_results (id,test_run_id,test_case_id,status)
			VALUES (?, 'run1', ?, 'FAIL')`, id+"-rr", id).Error)
		_, err := s.LinkDefectToResult(defectID, id+"-rr", id)
		require.NoError(t, err)
		_, err = s.LinkDefectToTestCase(defectID, id)
		require.NoError(t, err)
	}
	var doomedCases []string
	for i := 0; i < 2*testBindLimit; i++ {
		folderID := doomed.ID
		if i%2 == 1 {
			folderID = sub.ID
		}
		id := fmt.Sprintf("tc%d", i)
		seedCase(id, folderID)
		doomedCases = append(doomedCases, id)
	}
	seedCase("keep", kept.ID)
	lowerBindLimit(t, s)

	require.NoError(t, s.DeleteFolder(doomed.ID))

	for _, id := range doomedCases {
		assert.Zero(t, countWhere(t, s, &models.TestCase{}, "id = ?", id), "test case %s", id)
		assert.Zero(t, countWhere(t, s, &models.DefectLink{}, "test_case_id = ?", id), "links still naming %s", id)
		assert.EqualValues(t, 1, countWhere(t, s, &models.RunResult{}, "id = ? AND test_case_id IS NULL", id+"-rr"),
			"run history of %s is kept, detached from the deleted case", id)
		assert.EqualValues(t, 1, countWhere(t, s, &models.DefectLink{}, "run_result_id = ?", id+"-rr"),
			"the result-scoped link of %s is kept", id)
	}
	assert.Zero(t, countWhere(t, s, &models.Folder{}, "id IN ?", []string{doomed.ID, sub.ID}))
	assert.EqualValues(t, 1, countWhere(t, s, &models.TestCase{}, "id = 'keep'"))
	assert.EqualValues(t, 2, countWhere(t, s, &models.DefectLink{}, "test_case_id = 'keep'"))
}

// TestDeleteRequirementsWithMoreDescendantsThanBindLimit covers the requirement cascade: the
// descendant closure is discovered level by level and then bound whole into the deletes, so a
// wide requirement tree (an imported epic, say) overflowed in the BFS or in the final delete.
func TestDeleteRequirementsWithMoreDescendantsThanBindLimit(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at)
		VALUES ('tc','T',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	mk := func(id string, parentID *string) {
		require.NoError(t, s.db.Create(&models.Requirement{ID: id, Identifier: id, Title: id, ParentID: parentID}).Error)
		require.NoError(t, s.db.Create(&models.RequirementTestCaseLink{ID: id + "-l", RequirementID: id, TestCaseID: "tc"}).Error)
	}
	root := "root"
	mk(root, nil)
	doomed := []string{root}
	for i := 0; i < 2*testBindLimit; i++ {
		child := fmt.Sprintf("child%d", i)
		mk(child, &root)
		doomed = append(doomed, child)
		if i%2 == 0 {
			grandchild := child + "-gc"
			mk(grandchild, &child)
			doomed = append(doomed, grandchild)
		}
	}
	mk("kept", nil)
	lowerBindLimit(t, s)

	require.NoError(t, s.DeleteRequirement(root))

	for _, id := range doomed {
		assert.Zero(t, countWhere(t, s, &models.Requirement{}, "id = ?", id), "requirement %s", id)
		assert.Zero(t, countWhere(t, s, &models.RequirementTestCaseLink{}, "requirement_id = ?", id), "links of %s", id)
	}
	assert.EqualValues(t, 1, countWhere(t, s, &models.Requirement{}, "id = 'kept'"))
	assert.EqualValues(t, 1, countWhere(t, s, &models.RequirementTestCaseLink{}, "requirement_id = 'kept'"))
}

// seqCTE numbers rows 1..? for the INSERT ... SELECT fixtures below, so a fixture of a thousand
// rows is one statement binding a handful of variables rather than a thousand round-trips.
const seqCTE = `WITH RECURSIVE seq(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM seq WHERE i < ?) `

// seedCases creates test cases <prefix>-tc1..n.
func seedCases(t *testing.T, s *Store, prefix string, n int) {
	t.Helper()
	require.NoError(t, s.db.Exec(seqCTE+`INSERT INTO test_cases (id,name,created_at,updated_at)
		SELECT ? || '-tc' || i, ? || ' case ' || i, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP FROM seq`,
		n, prefix, prefix).Error)
}

// seedRunOfCases creates run runID holding one FAIL result on each of the test cases <prefix>-tc1..n.
func seedRunOfCases(t *testing.T, s *Store, runID, prefix string, folderID *string, n int) {
	t.Helper()
	require.NoError(t, s.db.Exec(`INSERT INTO test_runs (id,name,run_folder_id) VALUES (?,?,?)`, runID, runID, folderID).Error)
	require.NoError(t, s.db.Exec(seqCTE+`INSERT INTO run_results (id,test_run_id,test_case_id,test_name_snapshot,status)
		SELECT ? || '-rr' || i, ?, ? || '-tc' || i, ? || ' case ' || i, 'FAIL' FROM seq`,
		n, runID, runID, prefix, prefix).Error)
}

// TestCopyTestRunWithMoreResultsThanOneInsertBinds: a copy inserted all of the run's results in
// one multi-row INSERT at a variable per column per row, so a run of ~940 results or more (the AI
// demo's hold ~1,500) failed to copy with "too many SQL variables".
func TestCopyTestRunWithMoreResultsThanOneInsertBinds(t *testing.T) {
	s := newTestStore(t)
	n := rowsOverflowingOneInsert(t, s, "run_results")
	seedCases(t, s, "src", n)
	seedRunOfCases(t, s, "src", "src", nil, n)

	cp, err := s.CopyTestRun("src", "", nil)

	require.NoError(t, err)
	assert.EqualValues(t, n, countWhere(t, s, &models.RunResult{}, "test_run_id = ? AND status = ?", cp.ID, models.StatusPending))
}

// TestCopyRunFolderWithMoreResultsThanOneInsertBinds is the folder copy, which inserts each run's
// results the same way.
func TestCopyRunFolderWithMoreResultsThanOneInsertBinds(t *testing.T) {
	s := newTestStore(t)
	n := rowsOverflowingOneInsert(t, s, "run_results")
	folder := &models.RunFolder{Name: "Nightly"}
	require.NoError(t, s.CreateRunFolder(folder))
	seedCases(t, s, "src", n)
	seedRunOfCases(t, s, "src", "src", &folder.ID, n)

	cp, err := s.CopyRunFolder(folder.ID, "", nil)

	require.NoError(t, err)
	assert.EqualValues(t, n, countWhere(t, s, &models.RunResult{},
		"test_run_id IN (SELECT id FROM test_runs WHERE run_folder_id = ?)", cp.ID))
}

// TestCreateRunFromCategoryWithMoreCasesThanOneInsertBinds: a run created from a category snapshots
// one result per test case in the category, all in one INSERT, so a category of ~940 test cases or
// more could not be run. (A run from explicit test-case ids is capped at 500 by the API.)
func TestCreateRunFromCategoryWithMoreCasesThanOneInsertBinds(t *testing.T) {
	s := newTestStore(t)
	n := rowsOverflowingOneInsert(t, s, "run_results")
	cat, err := s.CreateCategory("Regression", "")
	require.NoError(t, err)
	seedCases(t, s, "cat", n)
	require.NoError(t, s.db.Exec(seqCTE+`INSERT INTO suite_test_cases (suite_id,test_case_id)
		SELECT ?, 'cat-tc' || i FROM seq`, n, cat.ID).Error)

	run := &models.TestRun{CategoryID: &cat.ID}
	require.NoError(t, s.CreateTestRunWithCases(run, nil))

	assert.EqualValues(t, n, countWhere(t, s, &models.RunResult{}, "test_run_id = ?", run.ID))
}

// TestCreateBatchFitsWidestTable guards createBatchSize, which the store's GORM config uses to
// split every slice insert: a full batch of the widest table's rows must bind no more variables
// than SQLite allows, or batching only moves the overflow.
func TestCreateBatchFitsWidestTable(t *testing.T) {
	s := newTestStore(t)
	var tables []string
	require.NoError(t, s.db.Raw(`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).
		Scan(&tables).Error)
	require.NotEmpty(t, tables)
	widest, widestTable := 0, ""
	for _, table := range tables {
		if n := columnCount(t, s, table); n > widest {
			widest, widestTable = n, table
		}
	}

	assert.Equal(t, createBatchSize, s.db.CreateBatchSize, "the store's GORM config must batch slice inserts")
	assert.LessOrEqual(t, createBatchSize*widest, liveBindLimit(t, s),
		"a batch of %d %s rows (%d columns) binds more variables than SQLite allows", createBatchSize, widestTable, widest)
}
