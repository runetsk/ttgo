package store

import (
	"fmt"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// aiDemoSeed pins the in-app AI demo dataset to one PRNG seed so every entity
// ID is deterministic (perfID): reloading purges the previous copy by ID and a
// scratch DB seeded by `perfseed -profile ai` (which uses seed 1 too) is
// replaced cleanly instead of colliding. Changing this — or the
// DefaultAISeedConfig shape — orphans rows seeded by older builds; "Remove
// Demo Data" (demo_seeds-driven) remains the universal cleanup for that case.
const aiDemoSeed = 1

// AIDemoRootFolderID returns the deterministic ID of the "AI Demo" root
// folder — its presence is the cheap "is the AI demo loaded?" probe.
func AIDemoRootFolderID() string { return perfID(aiDemoSeed, "ai-folder-root", 0) }

// AIDemoLatestRunID returns the deterministic ID of the newest seeded run —
// the one to point "Analyze failures" at.
func AIDemoLatestRunID() string { return perfID(aiDemoSeed, "ai-run", 0) }

// AISeedDemoResult is returned by SeedAIDemoTx (and POST /api/seed/ai).
type AISeedDemoResult struct {
	ReplacedExisting bool                `json:"replaced_existing"`
	Created          SeedCounts          `json:"created"`
	FailingRows      int                 `json:"failing_rows"`
	LabeledRows      int                 `json:"labeled_rows"`
	LatestRunID      string              `json:"latest_run_id"`
	GroundTruth      []AISeedGroundTruth `json:"ground_truth"`
}

// HasAIDemoData reports whether the AI demo dataset is currently loaded.
func (s *Store) HasAIDemoData() (bool, error) {
	var count int64
	err := s.db.Model(&models.Folder{}).Where("id = ?", AIDemoRootFolderID()).Count(&count).Error
	return count > 0, err
}

// SeedAIDemoTx loads the AI failure-analysis demo dataset into the live
// database: purge any previous copy by deterministic ID, insert fresh rows in
// batches, and mark every entity in demo_seeds so the existing "Remove Demo
// Data" flow cleans the AI dataset up alongside the classic one.
func (s *Store) SeedAIDemoTx() (AISeedDemoResult, error) {
	cfg := DefaultAISeedConfig()
	cfg.Seed = aiDemoSeed
	return s.seedAIDemoTx(cfg)
}

// seedAIDemoTx is the config-injectable body of SeedAIDemoTx (tests shrink the
// dataset; production always uses the fixed default above).
func (s *Store) seedAIDemoTx(cfg AISeedConfig) (AISeedDemoResult, error) {
	ds, built, err := buildAIFailureDataset(cfg)
	if err != nil {
		return AISeedDemoResult{}, err
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := purgeAIDemoEntities(tx, ds); err != nil {
			return err
		}
		if err := insertAIDataset(tx, ds); err != nil {
			return err
		}
		return markAIDemoEntities(tx, ds)
	})
	if err != nil {
		return AISeedDemoResult{}, err
	}

	return AISeedDemoResult{
		Created: SeedCounts{
			Folders:     built.Folders,
			Categories:  built.Categories,
			TestCases:   built.TestCases,
			TestRuns:    built.TestRuns,
			RunResults:  built.RunResults,
			Defects:     len(ds.defects),
			DefectLinks: len(ds.links),
		},
		FailingRows: built.FailingRows,
		LabeledRows: built.LabeledRows,
		LatestRunID: built.LatestRunID,
		GroundTruth: built.GroundTruth,
	}, nil
}

// idChunks yields ids in slices small enough to stay clear of SQLite's bound
// parameter limit even with room for other placeholders in the query.
func idChunks(ids []string) [][]string {
	const size = 5000
	var out [][]string
	for len(ids) > size {
		out = append(out, ids[:size])
		ids = ids[size:]
	}
	if len(ids) > 0 {
		out = append(out, ids)
	}
	return out
}

func chunkedDelete(tx *gorm.DB, model interface{}, column string, ids []string) error {
	for _, chunk := range idChunks(ids) {
		if err := tx.Where(column+" IN ?", chunk).Delete(model).Error; err != nil {
			return err
		}
	}
	return nil
}

// purgeAIDemoEntities hard-deletes every row a previous AI demo load (or a
// same-seed perfseed run) created, plus anything the app attached to those
// rows since (analyses, jobs, comments, result-scoped defect links) — so a
// reload lands on clean ground. Safe when nothing was ever seeded.
func purgeAIDemoEntities(tx *gorm.DB, ds aiDataset) error {
	rrIDs := make([]string, 0, len(ds.results))
	for _, rr := range ds.results {
		rrIDs = append(rrIDs, rr.ID)
	}
	runIDs := make([]string, 0, len(ds.runs))
	for _, run := range ds.runs {
		runIDs = append(runIDs, run.ID)
	}
	tcIDs := make([]string, 0, len(ds.cases))
	for _, tc := range ds.cases {
		tcIDs = append(tcIDs, tc.ID)
	}
	catIDs := make([]string, 0, len(ds.categories))
	for _, c := range ds.categories {
		catIDs = append(catIDs, c.ID)
	}
	folderIDs := make([]string, 0, len(ds.folders))
	for _, f := range ds.folders {
		folderIDs = append(folderIDs, f.ID)
	}
	defectIDs := make([]string, 0, len(ds.defects))
	for _, d := range ds.defects {
		defectIDs = append(defectIDs, d.ID)
	}
	linkIDs := make([]string, 0, len(ds.links))
	for _, l := range ds.links {
		linkIDs = append(linkIDs, l.ID)
	}

	// App-attached rows hanging off seeded runs/results.
	for _, chunk := range idChunks(rrIDs) {
		if err := tx.Where("target_type = ? AND target_id IN ?", "result", chunk).Delete(&models.Comment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("run_result_id IN ?", chunk).Delete(&models.RunResultAnalysis{}).Error; err != nil {
			return err
		}
		if err := tx.Where("run_result_id IN ?", chunk).Delete(&models.DefectLink{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("target_type = ? AND target_id IN ?", "run", runIDs).Delete(&models.Comment{}).Error; err != nil {
		return err
	}
	if err := tx.Where("test_run_id IN ?", runIDs).Delete(&models.RunAnalysisJob{}).Error; err != nil {
		return err
	}

	// The seeded entities themselves, children before parents.
	if err := chunkedDelete(tx, &models.DefectLink{}, "id", linkIDs); err != nil {
		return err
	}
	// The user may have linked seeded defects to their own results/cases.
	if err := tx.Where("defect_id IN ?", defectIDs).Delete(&models.DefectLink{}).Error; err != nil {
		return err
	}
	if err := chunkedDelete(tx, &models.Defect{}, "id", defectIDs); err != nil {
		return err
	}
	if err := chunkedDelete(tx, &models.RunResult{}, "id", rrIDs); err != nil {
		return err
	}
	if err := chunkedDelete(tx, &models.TestRun{}, "id", runIDs); err != nil {
		return err
	}
	if err := tx.Where("suite_id IN ?", catIDs).Delete(&models.CategoryTestCase{}).Error; err != nil {
		return err
	}
	if err := chunkedDelete(tx, &models.CategoryTestCase{}, "test_case_id", tcIDs); err != nil {
		return err
	}
	// Rows created OUTSIDE the dataset can hold foreign keys into it — a user
	// (or CI ingest) running seeded test cases records results referencing
	// them, runs can point at seeded categories, steps/requirement links can
	// be attached to seeded cases, and folders can gain user subfolders.
	// Detach or drop those references the same way the app's own delete flows
	// do, or the purge trips FK enforcement.
	for _, chunk := range idChunks(tcIDs) {
		if err := tx.Model(&models.RunResult{}).Where("test_case_id IN ?", chunk).
			Update("test_case_id", nil).Error; err != nil {
			return err
		}
		if err := tx.Where("test_case_id IN ?", chunk).Delete(&models.TestStep{}).Error; err != nil {
			return err
		}
		if err := tx.Where("test_case_id IN ?", chunk).Delete(&models.RequirementTestCaseLink{}).Error; err != nil {
			return err
		}
	}
	if err := chunkedDelete(tx, &models.TestCase{}, "id", tcIDs); err != nil {
		return err
	}
	if err := tx.Model(&models.TestRun{}).Where("category_id IN ?", catIDs).
		Update("category_id", nil).Error; err != nil {
		return err
	}
	if err := tx.Where("id IN ?", catIDs).Delete(&models.Category{}).Error; err != nil {
		return err
	}
	if err := tx.Model(&models.Folder{}).Where("parent_id IN ?", folderIDs).
		Update("parent_id", nil).Error; err != nil {
		return err
	}
	if err := tx.Where("id IN ?", folderIDs).Delete(&models.Folder{}).Error; err != nil {
		return err
	}

	// Stale tracking rows for everything above.
	all := make([]string, 0, len(rrIDs)+len(runIDs)+len(tcIDs)+len(catIDs)+len(folderIDs)+len(defectIDs)+len(linkIDs))
	all = append(all, rrIDs...)
	all = append(all, runIDs...)
	all = append(all, tcIDs...)
	all = append(all, catIDs...)
	all = append(all, folderIDs...)
	all = append(all, defectIDs...)
	all = append(all, linkIDs...)
	return chunkedDelete(tx, &models.DemoSeed{}, "entity_id", all)
}

// markAIDemoEntities records every seeded entity in demo_seeds, batched — the
// classic per-row seedMark would mean ~16k individual INSERTs.
func markAIDemoEntities(tx *gorm.DB, ds aiDataset) error {
	now := time.Now()
	marks := make([]models.DemoSeed, 0,
		len(ds.folders)+len(ds.categories)+len(ds.cases)+len(ds.runs)+len(ds.results)+len(ds.defects)+len(ds.links))
	add := func(entityType, entityID string) {
		marks = append(marks, models.DemoSeed{
			ID: uuid.New().String(), EntityType: entityType, EntityID: entityID, SeededAt: now,
		})
	}
	for _, f := range ds.folders {
		add("folder", f.ID)
	}
	for _, c := range ds.categories {
		add("category", c.ID)
	}
	for _, tc := range ds.cases {
		add("test_case", tc.ID)
	}
	for _, run := range ds.runs {
		add("test_run", run.ID)
	}
	for _, rr := range ds.results {
		add("run_result", rr.ID)
	}
	for _, d := range ds.defects {
		add("defect", d.ID)
	}
	for _, l := range ds.links {
		add("defect_link", l.ID)
	}
	if err := tx.CreateInBatches(&marks, 500).Error; err != nil {
		return fmt.Errorf("insert demo-seed marks: %w", err)
	}
	return nil
}
