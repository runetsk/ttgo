package store

import (
	"ttgo/pkg/tracker/models"

	"gorm.io/gorm"
)

// ResetCounts holds per-table counts for the reset-all operation.
type ResetCounts struct {
	RunResults        int64 `json:"run_results"`
	RunMetrics        int64 `json:"run_metrics"`
	FlakyStats        int64 `json:"flaky_stats"`
	TestRuns          int64 `json:"test_runs"`
	RunFolders        int64 `json:"run_folders"`
	DefectLinks       int64 `json:"defect_links"`
	CategoryTestCases int64 `json:"category_test_cases"`
	ReqLinks          int64 `json:"requirement_links"`
	TestSteps         int64 `json:"test_steps"`
	Versions          int64 `json:"test_case_versions"`
	CustomValues      int64 `json:"custom_field_values"`
	TestCases         int64 `json:"test_cases"`
	Categories        int64 `json:"categories"`
	Requirements      int64 `json:"requirements"`
	Folders           int64 `json:"folders"`
	DemoSeeds         int64 `json:"demo_seeds"`
	WebhookLogs       int64 `json:"webhook_logs"`
	Configs           int64 `json:"configs"`
	AuditLogs         int64 `json:"audit_logs"`
}

// ResetAllDataTx deletes ALL application data in FK-safe order within a
// single transaction. User accounts and sessions are preserved.
func (s *Store) ResetAllDataTx() (ResetCounts, error) {
	var c ResetCounts
	err := s.db.Transaction(func(tx *gorm.DB) error {
		del := func(model interface{}) (int64, error) {
			r := tx.Unscoped().Where("1 = 1").Delete(model)
			return r.RowsAffected, r.Error
		}

		var err error
		if c.RunResults, err = del(&models.RunResult{}); err != nil {
			return err
		}
		if c.RunMetrics, err = del(&models.RunMetric{}); err != nil {
			return err
		}
		if c.FlakyStats, err = del(&models.FlakyStat{}); err != nil {
			return err
		}
		if c.TestRuns, err = del(&models.TestRun{}); err != nil {
			return err
		}
		tx.Model(&models.RunFolder{}).Where("1 = 1").Update("parent_id", nil)
		if c.RunFolders, err = del(&models.RunFolder{}); err != nil {
			return err
		}
		if c.DefectLinks, err = del(&models.DefectLink{}); err != nil {
			return err
		}
		if c.CategoryTestCases, err = del(&models.CategoryTestCase{}); err != nil {
			return err
		}
		if c.ReqLinks, err = del(&models.RequirementTestCaseLink{}); err != nil {
			return err
		}
		if c.TestSteps, err = del(&models.TestStep{}); err != nil {
			return err
		}
		if c.Versions, err = del(&models.TestCaseVersion{}); err != nil {
			return err
		}
		if c.CustomValues, err = del(&models.CustomFieldValue{}); err != nil {
			return err
		}
		if c.TestCases, err = del(&models.TestCase{}); err != nil {
			return err
		}
		if c.Categories, err = del(&models.Category{}); err != nil {
			return err
		}
		if c.Requirements, err = del(&models.Requirement{}); err != nil {
			return err
		}
		tx.Model(&models.Folder{}).Where("1 = 1").Update("parent_id", nil)
		if c.Folders, err = del(&models.Folder{}); err != nil {
			return err
		}
		if c.DemoSeeds, err = del(&models.DemoSeed{}); err != nil {
			return err
		}
		if c.WebhookLogs, err = del(&models.WebhookDispatchLog{}); err != nil {
			return err
		}

		var n int64
		if n, err = del(&models.JiraConfig{}); err != nil {
			return err
		}
		c.Configs += n
		if n, err = del(&models.ConfluenceConfig{}); err != nil {
			return err
		}
		c.Configs += n
		if n, err = del(&models.LLMProviderConfig{}); err != nil {
			return err
		}
		c.Configs += n
		if n, err = del(&models.AIGenTemplate{}); err != nil {
			return err
		}
		c.Configs += n
		if n, err = del(&models.WebhookConfig{}); err != nil {
			return err
		}
		c.Configs += n
		if n, err = del(&models.CustomFieldDefinition{}); err != nil {
			return err
		}
		c.Configs += n
		if n, err = del(&models.ApiToken{}); err != nil {
			return err
		}
		c.Configs += n
		// Preserve the audit log: deleting it would erase the forensic trail of
		// this destructive reset itself and of all prior security events (F-037).
		c.AuditLogs = 0
		return nil
	})
	return c, err
}
