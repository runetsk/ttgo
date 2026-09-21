package store

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

// aiDemoTestCfg mirrors aiTestCfg but MUST keep Seed == aiDemoSeed: the
// status/purge helpers derive their deterministic IDs from that constant.
func aiDemoTestCfg() AISeedConfig {
	return AISeedConfig{Seed: aiDemoSeed, Days: 12, ResultsPerRun: 300, TestCases: 350}
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
	assert.Equal(t, 12*300, res.Created.RunResults)
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
	assert.Equal(t, classicResults+12*300, withAI)

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

func TestAIDemoGroundTruth_ListsEveryPlantedTemplate(t *testing.T) {
	gt, err := AIDemoGroundTruth()
	if err != nil {
		t.Fatal(err)
	}
	if len(gt) != len(aiTemplates) {
		t.Fatalf("got %d entries, want one per template (%d)", len(gt), len(aiTemplates))
	}
	byKey := map[string]AISeedGroundTruth{}
	for _, g := range gt {
		byKey[g.TemplateKey] = g
		if g.TemplateKey == "" || g.SampleMessage == "" || g.ExpectedDefect == "" {
			t.Fatalf("incomplete entry: %+v", g)
		}
		if g.ExpectedVerdict == "" && g.TemplateKey != "stale-checkout-selector" {
			t.Fatalf("only the stale-locator template is unscored on verdict: %+v", g)
		}
	}
	// A runner killed by an out-of-memory condition is what the product calls
	// infrastructure ("CI, the runner, or the network"), not an unknown.
	if oom := byKey["runner-heap-oom"]; oom.ExpectedVerdict != "infrastructure" || oom.ExpectedDefect != "system_issue" {
		t.Fatalf("runner-heap-oom: %+v", oom)
	}
	// A stale locator is an automation bug, and the verdict vocabulary has no
	// option for it: the key grades that template on its defect type only.
	if stale := byKey["stale-checkout-selector"]; stale.ExpectedVerdict != "" || stale.ExpectedDefect != "automation_bug" {
		t.Fatalf("stale-checkout-selector: %+v", stale)
	}
}

func TestAIDataset_LogTailWarnsAboutTheEdgeOnlyForSystemTemplates(t *testing.T) {
	ds, _, err := buildAIFailureDataset(AISeedConfig{Seed: 1, Days: 12, ResultsPerRun: 300, TestCases: 350})
	if err != nil {
		t.Fatal(err)
	}
	hinted, plain := 0, 0
	for _, r := range ds.results {
		if r.ErrorMessage == "" {
			continue
		}
		edge := strings.Contains(r.LogText, "cdn-edge")
		switch r.FailureType {
		case "timeout", "network":
			if edge {
				hinted++
			}
		default:
			if edge {
				t.Fatalf("a %s failure must not carry the transient-502 hint that points at the environment: %q", r.FailureType, r.LogText)
			}
			plain++
		}
	}
	if hinted == 0 || plain == 0 {
		t.Fatalf("expected both kinds of rows in the sample: hinted=%d plain=%d", hinted, plain)
	}
}

// latestRunFailures counts failing rows of the newest run in a built dataset.
func latestRunFailures(ds aiDataset, latestRunID string) int {
	n := 0
	for _, r := range ds.results {
		if r.TestRunID == latestRunID && (r.Status == models.StatusFail || r.Status == models.StatusError) {
			n++
		}
	}
	return n
}

func TestBuildAIFailureDataset_FailureScaleMultipliesPlantedFailures(t *testing.T) {
	base := AISeedConfig{Seed: 1, Days: 7, ResultsPerRun: 600, TestCases: 800}
	ds1, res1, err := buildAIFailureDataset(base)
	require.NoError(t, err)
	scaled := base.Scaled(3)
	require.Equal(t, 3, scaled.FailureScale)
	require.Equal(t, 1800, scaled.ResultsPerRun)
	require.Equal(t, 2400, scaled.TestCases)
	ds3, res3, err := buildAIFailureDataset(scaled)
	require.NoError(t, err)

	f1, f3 := latestRunFailures(ds1, res1.LatestRunID), latestRunFailures(ds3, res3.LatestRunID)
	assert.GreaterOrEqual(t, f3, f1*5/2, "scale 3 must roughly triple the newest run's failures (%d -> %d)", f1, f3)

	seen := map[string]bool{}
	for _, r := range ds3.results {
		key := r.TestRunID + "|" + r.TestNameSnapshot
		require.False(t, seen[key], "results must stay unique per (run, case): %s", key)
		seen[key] = true
	}
	// Every template with dedicated cases shows up in the newest run at scale 3.
	for _, g := range res3.GroundTruth {
		if g.Scenario == aiScenPersistent || g.Scenario == aiScenLatestIncident || g.Scenario == aiScenSingleton {
			assert.Positive(t, g.LatestRunRows, "template %s missing from the newest run", g.TemplateKey)
		}
	}
}

func TestAIDemoGroundTruth_CoversTheNewTemplates(t *testing.T) {
	gt, err := AIDemoGroundTruth()
	require.NoError(t, err)
	byKey := map[string]AISeedGroundTruth{}
	for _, g := range gt {
		byKey[g.TemplateKey] = g
	}
	want := map[string][2]string{
		"auth-token-expired":           {"environment", "system_issue"},
		"api-rate-limited-429":         {"environment", "system_issue"},
		"order-summary-null-typeerror": {"product_bug", "product_bug"},
		"visual-diff-missing-button":   {"product_bug", "product_bug"},
		"timezone-date-assertion":      {"product_bug", "product_bug"},
		"duplicate-fixture-email":      {"test_data", "automation_bug"},
		"feature-flag-missing":         {"environment", "system_issue"},
		"runner-disk-full":             {"infrastructure", "system_issue"},
		"search-count-off-by-one":      {"product_bug", "product_bug"},
		"session-redirect-loop":        {"product_bug", "product_bug"},
		"upload-progress-race":         {"flaky_test", "automation_bug"},
		"payment-gateway-503":          {"infrastructure", "system_issue"},
	}
	for key, exp := range want {
		g, ok := byKey[key]
		require.True(t, ok, "template %s missing from the answer key", key)
		assert.Equal(t, exp[0], g.ExpectedVerdict, key)
		assert.Equal(t, exp[1], g.ExpectedDefect, key)
		assert.NotEmpty(t, g.SampleMessage, key)
	}
	assert.Len(t, gt, 26, "14 original + 12 new templates")
}
