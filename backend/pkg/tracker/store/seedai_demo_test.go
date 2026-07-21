package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

// aiDemoTestCfg mirrors aiTestCfg but MUST keep Seed == aiDemoSeed: the
// status/purge helpers derive their deterministic IDs from that constant.
func aiDemoTestCfg() AISeedConfig {
	return AISeedConfig{Seed: aiDemoSeed, Days: 12, ResultsPerRun: 200, TestCases: 250}
}

func aiDemoCount(t *testing.T, s *Store, model interface{}) int64 {
	t.Helper()
	var n int64
	require.NoError(t, s.db.Model(model).Count(&n).Error)
	return n
}

func TestSeedAIDemoTxSeedsMarksAndStatus(t *testing.T) {
	s := newTestStore(t)

	has, err := s.HasAIDemoData()
	require.NoError(t, err)
	assert.False(t, has)

	res, err := s.seedAIDemoTx(aiDemoTestCfg())
	require.NoError(t, err)
	assert.False(t, res.ReplacedExisting) // handler stamps it; store default false
	assert.Equal(t, 12, res.Created.TestRuns)
	assert.Equal(t, 12*200, res.Created.RunResults)
	assert.Positive(t, res.FailingRows)
	assert.NotEmpty(t, res.GroundTruth)
	assert.Equal(t, AIDemoLatestRunID(), res.LatestRunID)

	has, err = s.HasAIDemoData()
	require.NoError(t, err)
	assert.True(t, has)

	// Every seeded entity is tracked in demo_seeds.
	wantMarks := int64(res.Created.Folders + res.Created.Categories + res.Created.TestCases +
		res.Created.TestRuns + res.Created.RunResults + res.Created.Defects + res.Created.DefectLinks)
	assert.Equal(t, wantMarks, aiDemoCount(t, s, &models.DemoSeed{}))

	status, err := s.GetSeedStatus()
	require.NoError(t, err)
	assert.True(t, status.HasAIDemoData)
	assert.Equal(t, AIDemoLatestRunID(), status.AILatestRunID)
	assert.True(t, status.HasDemoData, "AI rows are demo_seeds-tracked, so demo data is present")
}

// Reloading must replace, not duplicate — including rows the app attached to
// the previous copy (an analysis on a seeded result).
func TestSeedAIDemoTxReloadReplaces(t *testing.T) {
	s := newTestStore(t)
	cfg := aiDemoTestCfg()

	first, err := s.seedAIDemoTx(cfg)
	require.NoError(t, err)
	baseResults := aiDemoCount(t, s, &models.RunResult{})
	baseMarks := aiDemoCount(t, s, &models.DemoSeed{})

	// Simulate an app-generated analysis hanging off a seeded result.
	var rr models.RunResult
	require.NoError(t, s.db.Where("test_run_id = ? AND status = 'FAIL'", first.LatestRunID).First(&rr).Error)
	require.NoError(t, s.db.Create(&models.RunResultAnalysis{
		ID: "test-analysis-1", RunResultID: rr.ID, Version: 1, Verdict: models.VerdictProductBug,
	}).Error)

	second, err := s.seedAIDemoTx(cfg)
	require.NoError(t, err)
	assert.Equal(t, first.Created, second.Created)
	assert.Equal(t, baseResults, aiDemoCount(t, s, &models.RunResult{}), "reload must not duplicate results")
	assert.Equal(t, baseMarks, aiDemoCount(t, s, &models.DemoSeed{}), "reload must not duplicate marks")
	assert.Zero(t, aiDemoCount(t, s, &models.RunResultAnalysis{}), "attached analyses purged with their results")
}

// Reload must survive rows created OUTSIDE the dataset that hold foreign keys
// into it — the exact shape that made the first live load 500 with "FOREIGN
// KEY constraint failed": a user run whose results reference seeded test
// cases, a run categorized under a seeded category, a user subfolder under
// the AI Demo root, and a seeded defect linked to a user result.
func TestSeedAIDemoTxReloadDetachesForeignReferences(t *testing.T) {
	s := newTestStore(t)
	cfg := aiDemoTestCfg()

	first, err := s.seedAIDemoTx(cfg)
	require.NoError(t, err)

	var seededCase models.TestCase
	require.NoError(t, s.db.Where("folder_id IN (SELECT id FROM folders WHERE parent_id = ?)", AIDemoRootFolderID()).First(&seededCase).Error)
	var seededCat models.Category
	require.NoError(t, s.db.Where("name = ?", "nightly").First(&seededCat).Error)

	userRun := &models.TestRun{ID: "user-run-1", Name: "My own run", CategoryID: &seededCat.ID}
	require.NoError(t, s.db.Create(userRun).Error)
	userResult := &models.RunResult{
		ID: "user-result-1", TestRunID: userRun.ID, TestCaseID: &seededCase.ID,
		TestNameSnapshot: seededCase.Name, AttemptNumber: 1, Status: models.StatusFail,
		ErrorMessage: "user failure", FailureType: "assertion",
	}
	require.NoError(t, s.db.Create(userResult).Error)
	root := AIDemoRootFolderID()
	require.NoError(t, s.db.Create(&models.Folder{ID: "user-folder-1", Name: "My folder", ParentID: &root}).Error)
	var seededDefect models.Defect
	require.NoError(t, s.db.First(&seededDefect).Error)
	rrID := userResult.ID
	require.NoError(t, s.db.Create(&models.DefectLink{
		ID: "user-link-1", DefectID: seededDefect.ID, TestCaseID: &seededCase.ID, RunResultID: &rrID,
	}).Error)

	second, err := s.seedAIDemoTx(cfg)
	require.NoError(t, err, "reload must detach foreign references, not trip FK enforcement")
	assert.Equal(t, first.Created, second.Created)

	// The user's rows survive, detached from the replaced dataset.
	var gotRun models.TestRun
	require.NoError(t, s.db.First(&gotRun, "id = ?", "user-run-1").Error)
	assert.Nil(t, gotRun.CategoryID)
	var gotResult models.RunResult
	require.NoError(t, s.db.First(&gotResult, "id = ?", "user-result-1").Error)
	assert.Nil(t, gotResult.TestCaseID)
	var gotFolder models.Folder
	require.NoError(t, s.db.First(&gotFolder, "id = ?", "user-folder-1").Error)
	assert.Nil(t, gotFolder.ParentID)
	var linkCount int64
	require.NoError(t, s.db.Model(&models.DefectLink{}).Where("id = ?", "user-link-1").Count(&linkCount).Error)
	assert.Zero(t, linkCount, "links to seeded defects go with the defect")
}

// The existing Remove Demo Data flow must clean the AI dataset too (its rows
// share the demo_seeds tracking table).
func TestRemoveSeedClearsAIDemo(t *testing.T) {
	s := newTestStore(t)

	_, err := s.seedAIDemoTx(aiDemoTestCfg())
	require.NoError(t, err)

	_, err = s.RemoveSeedTx()
	require.NoError(t, err)

	has, err := s.HasAIDemoData()
	require.NoError(t, err)
	assert.False(t, has)
	assert.Zero(t, aiDemoCount(t, s, &models.RunResult{}))
	assert.Zero(t, aiDemoCount(t, s, &models.Folder{}))
	assert.Zero(t, aiDemoCount(t, s, &models.DemoSeed{}))
}

// The classic demo dataset and the AI dataset must coexist: loading or
// replacing one leaves the other intact.
func TestSeedAIDemoCoexistsWithClassicDemo(t *testing.T) {
	s := newTestStore(t)

	_, err := s.SeedDemoTx(false)
	require.NoError(t, err)
	classicResults := aiDemoCount(t, s, &models.RunResult{})

	_, err = s.seedAIDemoTx(aiDemoTestCfg())
	require.NoError(t, err)
	withAI := aiDemoCount(t, s, &models.RunResult{})
	assert.Equal(t, classicResults+12*200, withAI)

	// Replacing the classic demo must not touch the AI rows.
	_, err = s.SeedDemoTx(true)
	require.NoError(t, err)
	assert.Equal(t, withAI, aiDemoCount(t, s, &models.RunResult{}))
	has, err := s.HasAIDemoData()
	require.NoError(t, err)
	assert.True(t, has)

	// And replacing the AI demo must not touch the classic rows.
	_, err = s.seedAIDemoTx(aiDemoTestCfg())
	require.NoError(t, err)
	assert.Equal(t, withAI, aiDemoCount(t, s, &models.RunResult{}))
}
