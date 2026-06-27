package store

import (
	"ttgo/pkg/tracker/models"

	"gorm.io/gorm"
)

// SeedDemoTx creates all demo entities within a single atomic transaction.
// It unconditionally purges any pre-existing demo entities by their deterministic
// IDs before inserting fresh data, making the operation idempotent regardless of
// whether the demo_seeds tracking table is in sync with the actual data.
// removeFirst is preserved only for the ReplacedExisting flag in the response.
func (s *Store) SeedDemoTx(removeFirst bool) (SeedResult, error) {
	var result SeedResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := purgeKnownDemoEntities(tx); err != nil {
			return err
		}
		r, err := SeedDemo(tx)
		if err != nil {
			return err
		}
		result = r
		result.ReplacedExisting = removeFirst
		return nil
	})
	return result, err
}

// purgeKnownDemoEntities hard-deletes every record that demoDataset() would create,
// identified by their deterministic UUIDs. It is safe to call even when the
// demo_seeds table is empty or partially populated.
func purgeKnownDemoEntities(tx *gorm.DB) error {
	ds := demoDataset()

	rrIDs := make([]string, 0, len(ds.RunResults))
	for _, rr := range ds.RunResults {
		rrIDs = append(rrIDs, rr.ID)
	}
	analysisIDs := make([]string, 0, len(ds.RunResultAnalyses))
	for _, a := range ds.RunResultAnalyses {
		analysisIDs = append(analysisIDs, a.ID)
	}
	jobIDs := make([]string, 0, len(ds.RunAnalysisJobs))
	for _, j := range ds.RunAnalysisJobs {
		jobIDs = append(jobIDs, j.ID)
	}
	defectIDs := make([]string, 0, len(ds.DefectLinks))
	for _, d := range ds.DefectLinks {
		defectIDs = append(defectIDs, d.ID)
	}
	providerIDs := make([]string, 0, len(ds.LLMProviders))
	for _, p := range ds.LLMProviders {
		providerIDs = append(providerIDs, p.ID)
	}
	runIDs := make([]string, 0, len(ds.TestRuns))
	for _, run := range ds.TestRuns {
		runIDs = append(runIDs, run.ID)
	}
	rfIDs := make([]string, 0, len(ds.RunFolders))
	for _, rf := range ds.RunFolders {
		rfIDs = append(rfIDs, rf.ID)
	}
	tcIDs := make([]string, 0, len(ds.TestCases))
	for _, tc := range ds.TestCases {
		tcIDs = append(tcIDs, tc.ID)
	}
	categoryIDs := make([]string, 0, len(ds.Categories))
	for _, category := range ds.Categories {
		categoryIDs = append(categoryIDs, category.ID)
	}
	folderIDs := make([]string, 0, len(ds.Folders))
	for _, f := range ds.Folders {
		folderIDs = append(folderIDs, f.ID)
	}
	reqIDs := make([]string, 0, len(ds.Requirements))
	for _, req := range ds.Requirements {
		reqIDs = append(reqIDs, req.ID)
	}
	stepIDs := make([]string, 0, len(ds.Steps))
	for _, step := range ds.Steps {
		stepIDs = append(stepIDs, step.ID)
	}
	linkIDs := make([]string, 0, len(ds.RequirementLinks))
	for _, link := range ds.RequirementLinks {
		linkIDs = append(linkIDs, link.ID)
	}

	// Comments attached to demo runs/results — cleared up via demo_seeds during
	// RemoveSeed, but on a re-seed we may not have a tracking row if the demo was
	// partially written. Purge by target_id to stay idempotent.
	if len(rrIDs) > 0 {
		if err := tx.Where("target_type = ? AND target_id IN ?", "result", rrIDs).
			Delete(&models.Comment{}).Error; err != nil {
			return err
		}
	}
	if len(runIDs) > 0 {
		if err := tx.Where("target_type = ? AND target_id IN ?", "run", runIDs).
			Delete(&models.Comment{}).Error; err != nil {
			return err
		}
	}
	if len(analysisIDs) > 0 {
		if err := tx.Where("id IN ?", analysisIDs).Delete(&models.RunResultAnalysis{}).Error; err != nil {
			return err
		}
	}
	if len(rrIDs) > 0 {
		if err := tx.Where("run_result_id IN ?", rrIDs).Delete(&models.RunResultAnalysis{}).Error; err != nil {
			return err
		}
	}
	if len(jobIDs) > 0 {
		if err := tx.Where("id IN ?", jobIDs).Delete(&models.RunAnalysisJob{}).Error; err != nil {
			return err
		}
	}
	if len(defectIDs) > 0 {
		if err := tx.Where("id IN ?", defectIDs).Delete(&models.DefectLink{}).Error; err != nil {
			return err
		}
	}
	if len(rrIDs) > 0 {
		if err := tx.Where("run_result_id IN ?", rrIDs).Delete(&models.DefectLink{}).Error; err != nil {
			return err
		}
	}
	if len(providerIDs) > 0 {
		if err := tx.Where("id IN ?", providerIDs).Delete(&models.LLMProviderConfig{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("id IN ?", rrIDs).Delete(&models.RunResult{}).Error; err != nil {
		return err
	}
	if err := tx.Where("id IN ?", runIDs).Delete(&models.TestRun{}).Error; err != nil {
		return err
	}
	if err := tx.Where("suite_id IN ?", categoryIDs).Delete(&models.CategoryTestCase{}).Error; err != nil {
		return err
	}
	if err := tx.Where("test_case_id IN ?", tcIDs).Delete(&models.CategoryTestCase{}).Error; err != nil {
		return err
	}
	if len(linkIDs) > 0 {
		if err := tx.Where("id IN ?", linkIDs).Delete(&models.RequirementTestCaseLink{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("test_case_id IN ?", tcIDs).Delete(&models.RequirementTestCaseLink{}).Error; err != nil {
		return err
	}
	if err := tx.Where("requirement_id IN ?", reqIDs).Delete(&models.RequirementTestCaseLink{}).Error; err != nil {
		return err
	}
	if len(stepIDs) > 0 {
		if err := tx.Where("id IN ?", stepIDs).Delete(&models.TestStep{}).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("test_case_id IN ?", tcIDs).Delete(&models.TestStep{}).Error; err != nil {
		return err
	}
	if err := tx.Where("id IN ?", tcIDs).Delete(&models.TestCase{}).Error; err != nil {
		return err
	}
	if err := tx.Where("id IN ?", categoryIDs).Delete(&models.Category{}).Error; err != nil {
		return err
	}
	if err := tx.Model(&models.TestRun{}).
		Where("run_folder_id IN ?", rfIDs).
		Update("run_folder_id", nil).Error; err != nil {
		return err
	}
	if err := tx.Where("id IN ?", rfIDs).Delete(&models.RunFolder{}).Error; err != nil {
		return err
	}
	if err := tx.Where("id IN ?", reqIDs).Delete(&models.Requirement{}).Error; err != nil {
		return err
	}
	if err := tx.Model(&models.Folder{}).
		Where("parent_id IN ?", folderIDs).
		Update("parent_id", nil).Error; err != nil {
		return err
	}
	if err := tx.Where("id IN ?", folderIDs).Delete(&models.Folder{}).Error; err != nil {
		return err
	}
	if err := tx.Where("1 = 1").Delete(&models.DemoSeed{}).Error; err != nil {
		return err
	}

	return nil
}

// RemoveSeedTx removes all demo-seeded entities within a single atomic transaction.
func (s *Store) RemoveSeedTx() (SeedDeleteResult, error) {
	var result SeedDeleteResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var txErr error
		result, txErr = RemoveSeed(tx)
		return txErr
	})
	return result, err
}

// GetSeedStatus queries the demo_seeds table and returns summary information.
func (s *Store) GetSeedStatus() (SeedStatus, error) {
	var count int64
	if err := s.db.Model(&models.DemoSeed{}).Count(&count).Error; err != nil {
		return SeedStatus{}, err
	}

	if count == 0 {
		return SeedStatus{HasDemoData: false}, nil
	}

	var earliest models.DemoSeed
	if err := s.db.Order("seeded_at ASC").First(&earliest).Error; err != nil {
		return SeedStatus{}, err
	}

	counts, err := s.seedCountsByType()
	if err != nil {
		return SeedStatus{}, err
	}

	return SeedStatus{
		HasDemoData: true,
		SeededAt:    &earliest.SeededAt,
		Counts:      &counts,
	}, nil
}

// seedCountsByType returns per-entity-type counts from the demo_seeds table.
func (s *Store) seedCountsByType() (SeedCounts, error) {
	type row struct {
		EntityType string
		Cnt        int
	}

	var rows []row
	if err := s.db.Model(&models.DemoSeed{}).
		Select("entity_type, COUNT(*) as cnt").
		Group("entity_type").
		Scan(&rows).Error; err != nil {
		return SeedCounts{}, err
	}

	var counts SeedCounts
	for _, r := range rows {
		switch r.EntityType {
		case "folder":
			counts.Folders = r.Cnt
		case "category":
			counts.Categories = r.Cnt
		case "run_folder":
			counts.RunFolders = r.Cnt
		case "test_case":
			counts.TestCases = r.Cnt
		case "test_run":
			counts.TestRuns = r.Cnt
		case "run_result":
			counts.RunResults = r.Cnt
		case "requirement":
			counts.Requirements = r.Cnt
		case "defect_link":
			counts.DefectLinks = r.Cnt
		case "llm_provider":
			counts.LLMProviders = r.Cnt
		case "run_result_analysis":
			counts.RunResultAnalyses = r.Cnt
		case "comment":
			counts.Comments = r.Cnt
		}
	}

	return counts, nil
}
