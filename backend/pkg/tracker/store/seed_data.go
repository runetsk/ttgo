package store

import (
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
)

// demoNamespace is the fixed UUID v5 namespace used to derive deterministic IDs for
// all demo entities. Using the same namespace + name across re-seeds guarantees the
// same IDs, which simplifies idempotency checks and simplifies the DemoSeed lookup.
var demoNamespace = uuid.MustParse("d5e3f1a2-bc44-4e8c-9f0d-1a2b3c4d5e6f")

// demoID returns a deterministic UUID v5 string for the given logical name.
func demoID(name string) string {
	return uuid.NewSHA1(demoNamespace, []byte(name)).String()
}

// seedDataset is the full in-memory representation of the demo dataset.
type seedDataset struct {
	Folders             []models.Folder
	Categories          []models.Category
	TestCases           []models.TestCase
	Steps               []models.TestStep
	CategoryAssignments []models.CategoryTestCase
	RunFolders          []models.RunFolder
	TestRuns            []models.TestRun
	RunResults          []models.RunResult
	Requirements        []models.Requirement
	RequirementLinks    []models.RequirementTestCaseLink
	Defects             []models.Defect     // native demo: Task 9
	DefectLinks         []models.DefectLink // native demo: Task 9
	LLMProviders        []models.LLMProviderConfig
	RunResultAnalyses   []models.RunResultAnalysis
	RunAnalysisJobs     []models.RunAnalysisJob
}

// ptr is a tiny helper that returns a pointer to the given string value.
func ptr(s string) *string { return &s }

// demoDataset builds and returns the complete hardcoded demo dataset.
// All IDs are deterministic (UUID v5) so that re-seeding after removal produces
// identical rows and the same DemoSeed lookup entries.
func demoDataset() seedDataset {
	now := time.Now()
	catalog := buildDemoCatalog(now)
	runs := buildDemoRuns(now, catalog)
	requirements := buildDemoRequirements(now, catalog)
	ai := buildDemoAIData(now)
	defects, defectLinks := buildDemoDefects(now)

	return seedDataset{
		Folders:             catalog.Folders,
		Categories:          catalog.Categories,
		TestCases:           catalog.TestCases,
		Steps:               catalog.Steps,
		CategoryAssignments: catalog.CategoryAssignments,
		RunFolders:          runs.RunFolders,
		TestRuns:            runs.TestRuns,
		RunResults:          runs.RunResults,
		Requirements:        requirements.Requirements,
		RequirementLinks:    requirements.RequirementLinks,
		Defects:             defects,
		DefectLinks:         defectLinks,
		LLMProviders:        ai.Providers,
		RunResultAnalyses:   ai.Analyses,
		RunAnalysisJobs:     ai.Jobs,
	}
}
