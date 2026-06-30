package store

import "time"

// SeedCounts holds per-entity-type counts used in seed operation responses.
type SeedCounts struct {
	Folders           int `json:"folders"`
	Categories        int `json:"categories"`
	RunFolders        int `json:"run_folders"`
	TestCases         int `json:"test_cases"`
	TestRuns          int `json:"test_runs"`
	RunResults        int `json:"run_results"`
	Requirements      int `json:"requirements"`
	Defects           int `json:"defects"`
	DefectLinks       int `json:"defect_links"`
	LLMProviders      int `json:"llm_providers"`
	RunResultAnalyses int `json:"run_result_analyses"`
	Comments          int `json:"comments"`
}

// SeedStatus is returned by GetSeedStatus.
type SeedStatus struct {
	HasDemoData bool        `json:"has_demo_data"`
	SeededAt    *time.Time  `json:"seeded_at,omitempty"`
	Counts      *SeedCounts `json:"counts,omitempty"`
}

// SeedResult is returned by SeedDemo.
type SeedResult struct {
	ReplacedExisting bool       `json:"replaced_existing"`
	Created          SeedCounts `json:"created"`
}

// SeedDeleteResult is returned by RemoveSeed.
type SeedDeleteResult struct {
	Deleted SeedCounts `json:"deleted"`
}
