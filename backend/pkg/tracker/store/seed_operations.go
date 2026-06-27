package store

import (
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// seedMark inserts a DemoSeed record for the given entity.
func seedMark(tx *gorm.DB, entityType, entityID string) error {
	return tx.Create(&models.DemoSeed{
		ID:         uuid.New().String(),
		EntityType: entityType,
		EntityID:   entityID,
		SeededAt:   time.Now(),
	}).Error
}

// SeedDemo creates all demo entities within the supplied transaction.
// The caller is responsible for wrapping this in db.Transaction().
func SeedDemo(tx *gorm.DB) (SeedResult, error) {
	ds := demoDataset()
	var counts SeedCounts

	for _, f := range ds.Folders {
		if err := tx.Create(&f).Error; err != nil {
			return SeedResult{}, err
		}
		if err := seedMark(tx, "folder", f.ID); err != nil {
			return SeedResult{}, err
		}
		counts.Folders++
	}

	for _, category := range ds.Categories {
		if err := tx.Create(&category).Error; err != nil {
			return SeedResult{}, err
		}
		if err := seedMark(tx, "category", category.ID); err != nil {
			return SeedResult{}, err
		}
		counts.Categories++
	}

	for _, tc := range ds.TestCases {
		if err := tx.Create(&tc).Error; err != nil {
			return SeedResult{}, err
		}
		if err := seedMark(tx, "test_case", tc.ID); err != nil {
			return SeedResult{}, err
		}
		counts.TestCases++
	}

	for _, step := range ds.Steps {
		if err := tx.Create(&step).Error; err != nil {
			return SeedResult{}, err
		}
	}

	for _, sa := range ds.CategoryAssignments {
		if err := tx.Create(&sa).Error; err != nil {
			return SeedResult{}, err
		}
	}

	for _, rf := range ds.RunFolders {
		if err := tx.Create(&rf).Error; err != nil {
			return SeedResult{}, err
		}
		if err := seedMark(tx, "run_folder", rf.ID); err != nil {
			return SeedResult{}, err
		}
		counts.RunFolders++
	}

	for _, run := range ds.TestRuns {
		if err := tx.Create(&run).Error; err != nil {
			return SeedResult{}, err
		}
		if err := seedMark(tx, "test_run", run.ID); err != nil {
			return SeedResult{}, err
		}
		counts.TestRuns++
	}

	for _, rr := range ds.RunResults {
		if err := tx.Create(&rr).Error; err != nil {
			return SeedResult{}, err
		}
		if err := seedMark(tx, "run_result", rr.ID); err != nil {
			return SeedResult{}, err
		}
		counts.RunResults++
	}

	for _, req := range ds.Requirements {
		if err := tx.Create(&req).Error; err != nil {
			return SeedResult{}, err
		}
		if err := seedMark(tx, "requirement", req.ID); err != nil {
			return SeedResult{}, err
		}
		counts.Requirements++
	}

	for _, link := range ds.RequirementLinks {
		if err := tx.Create(&link).Error; err != nil {
			return SeedResult{}, err
		}
	}

	for _, p := range ds.LLMProviders {
		if err := tx.Create(&p).Error; err != nil {
			return SeedResult{}, err
		}
		if err := seedMark(tx, "llm_provider", p.ID); err != nil {
			return SeedResult{}, err
		}
		counts.LLMProviders++
	}

	for _, d := range ds.DefectLinks {
		if err := tx.Create(&d).Error; err != nil {
			return SeedResult{}, err
		}
		if err := seedMark(tx, "defect_link", d.ID); err != nil {
			return SeedResult{}, err
		}
		counts.DefectLinks++
	}

	for _, tcID := range ds.ReverificationFlaggedTCIDs {
		if err := tx.Model(&models.TestCase{}).
			Where("id = ?", tcID).
			Update("reverification_flagged", true).Error; err != nil {
			return SeedResult{}, err
		}
	}

	for _, job := range ds.RunAnalysisJobs {
		if err := tx.Create(&job).Error; err != nil {
			return SeedResult{}, err
		}
		if err := seedMark(tx, "run_analysis_job", job.ID); err != nil {
			return SeedResult{}, err
		}
	}

	for _, a := range ds.RunResultAnalyses {
		if err := tx.Create(&a).Error; err != nil {
			return SeedResult{}, err
		}
		if err := seedMark(tx, "run_result_analysis", a.ID); err != nil {
			return SeedResult{}, err
		}
		counts.RunResultAnalyses++
	}

	// Comments need a live User ID (FK) — look one up at seed time.
	// Prefer an active admin; fall back to any user; if none exist, skip comments.
	var author models.User
	if err := tx.Where("role = ? AND active = ?", "admin", true).
		Order("created_at ASC").First(&author).Error; err != nil {
		if err := tx.Order("created_at ASC").First(&author).Error; err != nil {
			author = models.User{}
		}
	}
	if author.ID != "" {
		now := time.Now()
		for _, c := range buildDemoComments(now, author.ID) {
			if err := tx.Create(&c).Error; err != nil {
				return SeedResult{}, err
			}
			if err := seedMark(tx, "comment", c.ID); err != nil {
				return SeedResult{}, err
			}
			counts.Comments++
		}
	}

	return SeedResult{Created: counts}, nil
}

// RemoveSeed deletes all demo-seeded entities within the supplied transaction.
// Entities are deleted in reverse dependency order to avoid FK constraint issues.
// Missing entities (manually deleted) are silently skipped.
// The caller is responsible for wrapping this in db.Transaction().
func RemoveSeed(tx *gorm.DB) (SeedDeleteResult, error) {
	var seeds []models.DemoSeed
	if err := tx.Find(&seeds).Error; err != nil {
		return SeedDeleteResult{}, err
	}

	byType := make(map[string][]string)
	for _, s := range seeds {
		byType[s.EntityType] = append(byType[s.EntityType], s.EntityID)
	}

	var counts SeedCounts

	if ids := byType["comment"]; len(ids) > 0 {
		if err := tx.Where("id IN ?", ids).Delete(&models.Comment{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
		counts.Comments = len(ids)
	}

	if ids := byType["run_result_analysis"]; len(ids) > 0 {
		if err := tx.Where("id IN ?", ids).Delete(&models.RunResultAnalysis{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
		counts.RunResultAnalyses = len(ids)
	}

	if ids := byType["run_analysis_job"]; len(ids) > 0 {
		if err := tx.Where("id IN ?", ids).Delete(&models.RunAnalysisJob{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
	}

	if ids := byType["defect_link"]; len(ids) > 0 {
		if err := tx.Where("id IN ?", ids).Delete(&models.DefectLink{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
		counts.DefectLinks = len(ids)
	}

	if ids := byType["llm_provider"]; len(ids) > 0 {
		if err := tx.Where("id IN ?", ids).Delete(&models.LLMProviderConfig{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
		counts.LLMProviders = len(ids)
	}

	if ids := byType["run_result"]; len(ids) > 0 {
		if err := tx.Where("id IN ?", ids).Delete(&models.RunResult{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
		counts.RunResults = len(ids)
	}

	if ids := byType["test_run"]; len(ids) > 0 {
		if err := tx.Where("id IN ?", ids).Delete(&models.TestRun{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
		counts.TestRuns = len(ids)
	}

	if categoryIDs := byType["category"]; len(categoryIDs) > 0 {
		if err := tx.Where("suite_id IN ?", categoryIDs).Delete(&models.CategoryTestCase{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
	}
	if tcIDs := byType["test_case"]; len(tcIDs) > 0 {
		if err := tx.Where("test_case_id IN ?", tcIDs).Delete(&models.CategoryTestCase{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
	}
	if ids := byType["test_case"]; len(ids) > 0 {
		if err := tx.Where("test_case_id IN ?", ids).Delete(&models.RequirementTestCaseLink{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
	}
	if ids := byType["test_case"]; len(ids) > 0 {
		if err := tx.Where("test_case_id IN ?", ids).Delete(&models.TestStep{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
	}
	if ids := byType["test_case"]; len(ids) > 0 {
		if err := tx.Where("id IN ?", ids).Delete(&models.TestCase{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
		counts.TestCases = len(ids)
	}
	if ids := byType["category"]; len(ids) > 0 {
		if err := tx.Where("id IN ?", ids).Delete(&models.Category{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
		counts.Categories = len(ids)
	}
	if rfIDs := byType["run_folder"]; len(rfIDs) > 0 {
		if err := tx.Model(&models.TestRun{}).
			Where("run_folder_id IN ?", rfIDs).
			Update("run_folder_id", nil).Error; err != nil {
			return SeedDeleteResult{}, err
		}
		if err := tx.Where("id IN ?", rfIDs).Delete(&models.RunFolder{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
		counts.RunFolders = len(rfIDs)
	}
	if ids := byType["folder"]; len(ids) > 0 {
		if err := tx.Model(&models.Folder{}).
			Where("parent_id IN ?", ids).
			Update("parent_id", nil).Error; err != nil {
			return SeedDeleteResult{}, err
		}
		if err := tx.Where("id IN ?", ids).Delete(&models.Folder{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
		counts.Folders = len(ids)
	}
	if ids := byType["requirement"]; len(ids) > 0 {
		if err := tx.Where("requirement_id IN ?", ids).Delete(&models.RequirementTestCaseLink{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
		if err := tx.Where("id IN ?", ids).Delete(&models.Requirement{}).Error; err != nil {
			return SeedDeleteResult{}, err
		}
		counts.Requirements = len(ids)
	}
	if err := tx.Where("1 = 1").Delete(&models.DemoSeed{}).Error; err != nil {
		return SeedDeleteResult{}, err
	}

	return SeedDeleteResult{Deleted: counts}, nil
}
