package store

import (
	"fmt"
	"testing"

	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The read paths below bound a variable per row they had loaded: a run's results, a folder's or
// category's test cases, every test case in the install, every defect. GORM's Preload does the
// same with every distinct key it loads for. Each test seeds 2×testBindLimit rows, lowers the
// limit, and reads them back.

// library is a folder of test cases, each in the category, with one step, one requirement link
// and an open defect linked both to the test case and to its result in the run.
type library struct {
	folderID, categoryID, runID string
	n                           int
}

func seedLibrary(t *testing.T, s *Store) library {
	t.Helper()
	n := 2 * testBindLimit
	folder, err := s.CreateFolder("Library", nil)
	require.NoError(t, err)
	cat, err := s.CreateCategory("Smoke", "")
	require.NoError(t, err)
	d := &models.Defect{Title: "bug"}
	require.NoError(t, s.CreateDefect(d))
	require.NoError(t, s.db.Create(&models.Requirement{ID: "req", Identifier: "REQ-1", Title: "R"}).Error)

	exec := func(sql string, args ...interface{}) {
		t.Helper()
		require.NoError(t, s.db.Exec(seqCTE+sql, append([]interface{}{n}, args...)...).Error)
	}
	exec(`INSERT INTO test_cases (id,name,folder_id,created_at,updated_at)
		SELECT 'lib-tc' || i, 'case ' || i, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP FROM seq`, folder.ID)
	exec(`INSERT INTO suite_test_cases (suite_id,test_case_id) SELECT ?, 'lib-tc' || i FROM seq`, cat.ID)
	exec(`INSERT INTO test_steps (id,test_case_id,action,expected_result,order_index)
		SELECT 'lib-step' || i, 'lib-tc' || i, 'do', 'done', 0 FROM seq`)
	exec(`INSERT INTO requirement_test_case_links (id,requirement_id,test_case_id,created_at)
		SELECT 'lib-rl' || i, 'req', 'lib-tc' || i, CURRENT_TIMESTAMP FROM seq`)
	exec(`INSERT INTO defect_links (id,defect_id,test_case_id,created_at)
		SELECT 'lib-tcl' || i, ?, 'lib-tc' || i, CURRENT_TIMESTAMP FROM seq`, d.ID)
	seedRunOfCases(t, s, "lib-run", "lib", nil, n)
	exec(`INSERT INTO defect_links (id,defect_id,test_case_id,run_result_id,created_at)
		SELECT 'lib-rrl' || i, ?, 'lib-tc' || i, 'lib-run-rr' || i, CURRENT_TIMESTAMP FROM seq`, d.ID)
	return library{folderID: folder.ID, categoryID: cat.ID, runID: "lib-run", n: n}
}

// assertResultsHydrated checks the row shape GetTestRun promises for each result: its test case
// with that case's categories, and the result's defect-link counts.
func assertResultsHydrated(t *testing.T, lib library, results []*models.RunResult) {
	t.Helper()
	require.Len(t, results, lib.n)
	for _, rr := range results {
		require.NotNil(t, rr.TestCaseID, "result %s", rr.ID)
		if assert.NotNil(t, rr.TestCase, "test case of %s", rr.ID) {
			assert.Equal(t, *rr.TestCaseID, rr.TestCase.ID)
			if assert.Len(t, rr.TestCase.Categories, 1, "categories of %s", rr.TestCase.ID) {
				assert.Equal(t, lib.categoryID, rr.TestCase.Categories[0].ID)
			}
		}
		assert.Equal(t, 1, rr.OpenDefectLinkCount, "open defect links of %s", rr.ID)
	}
}

func TestGetTestRunWithMoreResultsThanBindLimit(t *testing.T) {
	s := newTestStore(t)
	lib := seedLibrary(t, s)
	lowerBindLimit(t, s)

	run, err := s.GetTestRun(lib.runID)

	require.NoError(t, err)
	require.NotNil(t, run)
	assertResultsHydrated(t, lib, run.RunResults)
	assert.Equal(t, lib.n, run.TotalAttempts)
}

// TestGetRunResultsByIDsWithMoreIDsThanBindLimit: auto-apply republishes a whole failure group's
// members through this, and a group can be any size.
func TestGetRunResultsByIDsWithMoreIDsThanBindLimit(t *testing.T) {
	s := newTestStore(t)
	lib := seedLibrary(t, s)
	var ids []string
	require.NoError(t, s.db.Model(&models.RunResult{}).Where("test_run_id = ?", lib.runID).Pluck("id", &ids).Error)
	lowerBindLimit(t, s)

	rows, err := s.GetRunResultsByIDs(lib.runID, ids)

	require.NoError(t, err)
	assertResultsHydrated(t, lib, rows)
}

func TestListTestCasesWithMoreCasesThanBindLimit(t *testing.T) {
	// plain is the listing's filter as one unpreloaded query: the order the endpoint has always
	// returned rows in, which loading them in chunks must not reshuffle.
	for _, tc := range []struct {
		name   string
		filter func(lib library) TestCaseFilter
		plain  func(db *gorm.DB, lib library) *gorm.DB
	}{
		{"folder list view",
			func(lib library) TestCaseFilter {
				return TestCaseFilter{FolderIDs: []string{lib.folderID}, ListView: true}
			},
			func(db *gorm.DB, lib library) *gorm.DB { return db.Where("folder_id IN ?", []string{lib.folderID}) }},
		{"folder full view",
			func(lib library) TestCaseFilter { return TestCaseFilter{FolderIDs: []string{lib.folderID}} },
			func(db *gorm.DB, lib library) *gorm.DB { return db.Where("folder_id IN ?", []string{lib.folderID}) }},
		{"category",
			func(lib library) TestCaseFilter { return TestCaseFilter{CategoryID: &lib.categoryID} },
			func(db *gorm.DB, lib library) *gorm.DB {
				return db.Joins("JOIN suite_test_cases ON suite_test_cases.test_case_id = test_cases.id").
					Where("suite_test_cases.suite_id = ?", lib.categoryID)
			}},
		{"no filter",
			func(library) TestCaseFilter { return TestCaseFilter{ListView: true} },
			func(db *gorm.DB, _ library) *gorm.DB { return db }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestStore(t)
			lib := seedLibrary(t, s)
			filter := tc.filter(lib)
			var want []*models.TestCase
			require.NoError(t, tc.plain(s.db.Model(&models.TestCase{}), lib).Find(&want).Error)
			require.Len(t, want, lib.n)
			lowerBindLimit(t, s)

			got, err := s.ListTestCases(filter)

			require.NoError(t, err)
			require.Len(t, got, lib.n)
			for i, tcase := range got {
				assert.Equal(t, want[i].ID, tcase.ID, "row %d keeps the listing's order", i)
				assert.Len(t, tcase.Categories, 1, "categories of %s", tcase.ID)
				assert.Len(t, tcase.LinkedRequirements, 1, "requirements of %s", tcase.ID)
				assert.Equal(t, 1, tcase.OpenDefectCount, "open defects of %s", tcase.ID)
				if filter.ListView {
					assert.Equal(t, 1, tcase.StepsCount, "step count of %s", tcase.ID)
				} else {
					assert.Len(t, tcase.Steps, 1, "steps of %s", tcase.ID)
				}
			}
		})
	}
}

// TestGetFolderTreeCountsDefectsPastBindLimit: the tree counted defects for every test case in
// the install in one statement and discarded its error, so past the limit every badge vanished.
func TestGetFolderTreeCountsDefectsPastBindLimit(t *testing.T) {
	s := newTestStore(t)
	lib := seedLibrary(t, s)
	lowerBindLimit(t, s)

	tree, err := s.GetFolderTree()

	require.NoError(t, err)
	var folder *models.Folder
	for _, f := range tree {
		if f.ID == lib.folderID {
			folder = f
		}
	}
	require.NotNil(t, folder)
	require.Len(t, folder.TestCases, lib.n)
	for _, tcase := range folder.TestCases {
		assert.Equal(t, 1, tcase.OpenDefectCount, "open defects of %s", tcase.ID)
	}
}

func TestListDefectsWithMoreDefectsThanBindLimit(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at)
		VALUES ('tc','T',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	n := 2 * testBindLimit
	for i := 0; i < n; i++ {
		d := &models.Defect{Title: fmt.Sprintf("bug %d", i)}
		require.NoError(t, s.CreateDefect(d))
		_, err := s.LinkDefectToTestCase(d.ID, "tc")
		require.NoError(t, err)
	}
	lowerBindLimit(t, s)

	defects, err := s.ListDefects("", "", "")

	require.NoError(t, err)
	require.Len(t, defects, n)
	for _, d := range defects {
		assert.Equal(t, 1, d.LinkedTestCount, "linked tests of %s", d.Title)
	}
}

func TestGetRequirementChildCountsWithMoreIDsThanBindLimit(t *testing.T) {
	s := newTestStore(t)
	var parents []string
	for i := 0; i < 2*testBindLimit; i++ {
		parent := fmt.Sprintf("p%d", i)
		require.NoError(t, s.db.Create(&models.Requirement{ID: parent, Identifier: parent, Title: parent}).Error)
		require.NoError(t, s.db.Create(&models.Requirement{ID: parent + "-c", Identifier: parent + "-c", Title: "c", ParentID: &parent}).Error)
		parents = append(parents, parent)
	}
	lowerBindLimit(t, s)

	counts, err := s.GetRequirementChildCounts(parents)

	require.NoError(t, err)
	require.Len(t, counts, len(parents))
	for _, p := range parents {
		assert.Equal(t, 1, counts[p], "children of %s", p)
	}
}

// TestApplyNarrationWithMoreClonesThanBindLimit: the narration is written to the representative
// and every clone, and the rows are read back by id for the live update.
func TestApplyNarrationWithMoreClonesThanBindLimit(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	mk := func(a *models.RunResultAnalysis) *models.RunResultAnalysis {
		a.RunResultID = addFailingResult(t, s, runID).ID
		a.Verdict, a.Confidence = models.VerdictFlakyTest, models.ConfidenceHigh
		out, err := s.CreateAnalysis(a)
		require.NoError(t, err)
		return out
	}
	rep := mk(&models.RunResultAnalysis{NarrativeStatus: models.NarrativeStatusPending})
	n := 2 * testBindLimit
	for i := 0; i < n; i++ {
		mk(&models.RunResultAnalysis{NarrativeStatus: models.NarrativeStatusPending, SourceAnalysisID: &rep.ID,
			DedupMethod: models.DedupMethodSignature, Rationale: "[Grouped from representative analysis] "})
	}
	lowerBindLimit(t, s)

	changed, err := s.ApplyNarration(rep.ID, okDelta())

	require.NoError(t, err)
	require.Len(t, changed, n+1)
	assert.Equal(t, rep.ID, changed[0].ID, "the representative comes first")
	for _, a := range changed {
		assert.Equal(t, "S", a.Summary, "summary of %s", a.ID)
	}
}

// TestRemoveSeedWithMoreEntitiesThanBindLimit: removing demo data deletes every marked entity by
// type, and the AI demo marks every one of its (tens of thousands of) results.
func TestRemoveSeedWithMoreEntitiesThanBindLimit(t *testing.T) {
	s := newTestStore(t)
	lib := seedLibrary(t, s)
	mark := func(entityType, sql string, args ...interface{}) {
		t.Helper()
		require.NoError(t, s.db.Exec(`INSERT INTO demo_seeds (id,entity_type,entity_id,seeded_at)
			SELECT ? || '-' || id, ?, id, CURRENT_TIMESTAMP FROM (`+sql+`)`,
			append([]interface{}{entityType, entityType}, args...)...).Error)
	}
	mark("test_case", `SELECT id FROM test_cases WHERE folder_id = ?`, lib.folderID)
	mark("test_run", `SELECT id FROM test_runs WHERE id = ?`, lib.runID)
	mark("run_result", `SELECT id FROM run_results WHERE test_run_id = ?`, lib.runID)
	mark("defect_link", `SELECT id FROM defect_links`)
	mark("folder", `SELECT id FROM folders WHERE id = ?`, lib.folderID)
	mark("category", `SELECT id FROM suites WHERE id = ?`, lib.categoryID)
	lowerBindLimit(t, s)

	var res SeedDeleteResult
	require.NoError(t, s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		res, err = RemoveSeed(tx)
		return err
	}))

	assert.Equal(t, lib.n, res.Deleted.TestCases)
	assert.Equal(t, lib.n, res.Deleted.RunResults)
	for _, table := range []string{"test_cases", "run_results", "test_runs", "defect_links", "suite_test_cases",
		"test_steps", "requirement_test_case_links", "demo_seeds"} {
		var n int64
		require.NoError(t, s.db.Table(table).Count(&n).Error)
		assert.Zero(t, n, "%s left behind", table)
	}
}
