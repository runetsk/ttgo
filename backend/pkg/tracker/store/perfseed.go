package store

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// PerfSeedConfig controls the size and shape of a generated performance
// dataset. Results is a total distributed evenly across Runs, so the created
// row count is Runs * (Results / Runs).
type PerfSeedConfig struct {
	Seed            uint64 // drives pseudo-random choices and deterministic entity IDs
	Folders         int    // area folders under the "Perf" root folder
	Categories      int
	TestCases       int     // historical catalog size (excludes the ingest pool)
	Runs            int     // historical runs, spread over DaysSpread
	Results         int     // total historical results (see note above)
	IngestPoolCases int     // test cases reserved for load-test ingestion (no history)
	FlakyFraction   float64 // fraction of catalog cases that alternate PASS/FAIL per run
	FailRate        float64 // failure probability for non-flaky cases
	DaysSpread      int     // runs are spread over the trailing N days
}

// PerfTier returns the preset config for a named dataset tier. Phase 1 ships
// "small"; "medium" and "large" arrive with the read-path scenarios (phase 2).
func PerfTier(name string) (PerfSeedConfig, error) {
	switch name {
	case "small":
		return PerfSeedConfig{
			Seed:            1,
			Folders:         20,
			Categories:      10,
			TestCases:       800,
			Runs:            50, // divides Results exactly (200 per run)
			Results:         10_000,
			IngestPoolCases: 500,
			FlakyFraction:   0.05,
			FailRate:        0.15,
			DaysSpread:      90,
		}, nil
	default:
		return PerfSeedConfig{}, fmt.Errorf("unknown perf tier %q (phase 1 supports: small)", name)
	}
}

// IsPerfDBPath reports whether path names a perf scratch database — basename
// "perf-*.db". The perfseed CLI refuses anything else so it can never be
// pointed at a real TTGO database.
func IsPerfDBPath(path string) bool {
	base := filepath.Base(path)
	return strings.HasPrefix(base, "perf-") && strings.HasSuffix(base, ".db")
}

// perfID returns a deterministic UUID for a seeded entity, namespaced by the
// config seed so different seeds produce disjoint IDs. Reseeding the same DB
// without wiping collides on primary keys by design (loud, not duplicated).
func perfID(seed uint64, kind string, n int) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("ttgo-perf:%d:%s:%d", seed, kind, n))).String()
}

// PerfCase identifies one ingest-pool test case. Name rides along so the k6
// manifest carries {id,name} pairs — scenarios send test_name_snapshot straight
// from the manifest instead of re-deriving the Go naming format in JS (a
// cross-language contract that would drift silently).
type PerfCase struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PerfSeedResult reports what SeedPerfDataset created. IngestPool is consumed
// by cmd/perfseed to build the k6 seed manifest.
type PerfSeedResult struct {
	Folders    int
	Categories int
	TestCases  int
	TestRuns   int
	RunResults int
	IngestPool []PerfCase
}

var perfBrowsers = [...]string{"chromium", "firefox", "webkit"}

// SeedPerfDataset generates a deterministic performance dataset: a catalog of
// folders/categories/test cases with historical runs and results spread over
// the trailing cfg.DaysSpread days, plus a reserved "ingest pool" of test
// cases (no history) that load-test scenarios post fresh results against.
func (s *Store) SeedPerfDataset(cfg PerfSeedConfig) (PerfSeedResult, error) {
	if cfg.Runs <= 0 || cfg.TestCases <= 0 {
		return PerfSeedResult{}, fmt.Errorf("runs and test cases must be positive")
	}
	perRun := cfg.Results / cfg.Runs
	if perRun > cfg.TestCases {
		return PerfSeedResult{}, fmt.Errorf(
			"results per run (%d) exceeds distinct test cases (%d): results must be unique per (run, case)",
			perRun, cfg.TestCases)
	}

	rng := rand.New(rand.NewPCG(cfg.Seed, cfg.Seed))
	now := time.Now()

	// --- Folders ---------------------------------------------------------
	rootID := perfID(cfg.Seed, "folder-root", 0)
	ingestRootID := perfID(cfg.Seed, "folder-ingest", 0)
	folders := []models.Folder{
		{ID: rootID, Name: "Perf", CreatedAt: now, UpdatedAt: now},
		{ID: ingestRootID, Name: "Perf Ingest Pool", CreatedAt: now, UpdatedAt: now},
	}
	areaIDs := make([]string, cfg.Folders)
	for i := 0; i < cfg.Folders; i++ {
		id := perfID(cfg.Seed, "folder", i)
		areaIDs[i] = id
		parent := rootID
		folders = append(folders, models.Folder{
			ID: id, Name: fmt.Sprintf("Perf Area %02d", i+1), ParentID: &parent,
			CreatedAt: now, UpdatedAt: now,
		})
	}

	// --- Categories ------------------------------------------------------
	ingestCatID := perfID(cfg.Seed, "category-ingest", 0)
	categories := []models.Category{
		{ID: ingestCatID, Name: "perf-ingest", Description: "Reserved for perf load-test ingestion", CreatedAt: now, UpdatedAt: now},
	}
	catIDs := make([]string, cfg.Categories)
	for i := 0; i < cfg.Categories; i++ {
		id := perfID(cfg.Seed, "category", i)
		catIDs[i] = id
		categories = append(categories, models.Category{
			ID: id, Name: fmt.Sprintf("perf-cat-%02d", i+1),
			Description: "Perf catalog category", CreatedAt: now, UpdatedAt: now,
		})
	}

	// --- Catalog test cases (with history) --------------------------------
	cases := make([]models.TestCase, 0, cfg.TestCases+cfg.IngestPoolCases)
	assignments := make([]models.CategoryTestCase, 0, cfg.TestCases+cfg.IngestPoolCases)
	for i := 0; i < cfg.TestCases; i++ {
		id := perfID(cfg.Seed, "case", i)
		cases = append(cases, models.TestCase{
			ID: id, FolderID: areaIDs[i%len(areaIDs)], Name: fmt.Sprintf("Perf TC %04d", i+1),
			Description: fmt.Sprintf(
				"Validates workflow %d of the perf catalog. Covers boundary and regression checks for area %02d, including retry and timeout handling.",
				i+1, (i%len(areaIDs))+1),
			CreatedAt: now, UpdatedAt: now,
		})
		assignments = append(assignments, models.CategoryTestCase{
			CategoryID: catIDs[i%len(catIDs)], TestCaseID: id,
		})
	}

	// --- Ingest pool test cases (no history) -------------------------------
	pool := make([]PerfCase, cfg.IngestPoolCases)
	for i := 0; i < cfg.IngestPoolCases; i++ {
		id := perfID(cfg.Seed, "ingest-case", i)
		name := fmt.Sprintf("Ingest TC %04d", i+1)
		pool[i] = PerfCase{ID: id, Name: name}
		cases = append(cases, models.TestCase{
			ID: id, FolderID: ingestRootID, Name: name,
			Description: "Reserved test case for perf load-test result ingestion.",
			CreatedAt:   now, UpdatedAt: now,
		})
		assignments = append(assignments, models.CategoryTestCase{
			CategoryID: ingestCatID, TestCaseID: id,
		})
	}

	// --- Historical runs + results ----------------------------------------
	flakyCount := int(float64(cfg.TestCases) * cfg.FlakyFraction)
	runs := make([]models.TestRun, 0, cfg.Runs)
	results := make([]models.RunResult, 0, cfg.Runs*perRun)
	for r := 0; r < cfg.Runs; r++ {
		runID := perfID(cfg.Seed, "run", r)
		dayOffset := r * cfg.DaysSpread / cfg.Runs
		runCreated := now.Add(-time.Duration(dayOffset) * 24 * time.Hour)
		catID := catIDs[r%len(catIDs)]
		runFailed := false

		for j := 0; j < perRun; j++ {
			caseIdx := (r*17 + j) % cfg.TestCases // deterministic per-run case mix, distinct while perRun <= TestCases
			status := models.StatusPass
			errMsg, stack, failureType := "", "", ""
			switch {
			case caseIdx < flakyCount:
				if r%2 == 1 {
					status = models.StatusFail
				}
			case rng.Float64() < cfg.FailRate:
				status = models.StatusFail
			case rng.Float64() < 0.03:
				status = models.StatusSkip
			}
			if status == models.StatusFail {
				runFailed = true
				failureType = [...]string{"assertion", "timeout"}[caseIdx%2]
				errMsg = fmt.Sprintf("Assertion failed: expected state %d to match baseline", caseIdx)
				stack = fmt.Sprintf(
					"Error: %s\n    at validateState (perf/area%02d/tc%04d.spec.js:42:11)\n    at runStep (lib/runner.js:118:9)\n    at execute (lib/runner.js:73:5)\n    at process (lib/pipeline.js:31:3)",
					errMsg, caseIdx%len(areaIDs), caseIdx)
			}
			dur := 50 + rng.Int64N(4950)
			start := runCreated.Add(time.Duration(j) * time.Second)
			id := cases[caseIdx].ID // catalog cases occupy indices 0..TestCases-1
			results = append(results, models.RunResult{
				ID:               perfID(cfg.Seed, "result", r*perRun+j),
				TestRunID:        runID,
				TestCaseID:       &id,
				AttemptNumber:    1,
				TestNameSnapshot: cases[caseIdx].Name,
				Status:           status,
				DurationMs:       dur,
				StartTime:        start,
				EndTime:          start.Add(time.Duration(dur) * time.Millisecond),
				ErrorMessage:     errMsg,
				StackTrace:       stack,
				FailureType:      failureType,
				Browser:          perfBrowsers[j%len(perfBrowsers)],
				Environment:      "perf",
			})
		}

		runStatus := models.StatusPass
		if runFailed {
			runStatus = models.StatusFail
		}
		runs = append(runs, models.TestRun{
			ID: runID, Name: fmt.Sprintf("Perf Run %03d", r+1), CategoryID: &catID,
			Status: runStatus, CreatedAt: runCreated, UpdatedAt: runCreated,
		})
	}

	// --- Insert ------------------------------------------------------------
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.CreateInBatches(&folders, 200).Error; err != nil {
			return fmt.Errorf("insert folders: %w", err)
		}
		if err := tx.CreateInBatches(&categories, 200).Error; err != nil {
			return fmt.Errorf("insert categories: %w", err)
		}
		if err := tx.CreateInBatches(&cases, 200).Error; err != nil {
			return fmt.Errorf("insert test cases: %w", err)
		}
		if err := tx.CreateInBatches(&assignments, 500).Error; err != nil {
			return fmt.Errorf("insert category assignments: %w", err)
		}
		if err := tx.CreateInBatches(&runs, 200).Error; err != nil {
			return fmt.Errorf("insert runs: %w", err)
		}
		if err := tx.CreateInBatches(&results, 500).Error; err != nil {
			return fmt.Errorf("insert results: %w", err)
		}
		return nil
	})
	if err != nil {
		return PerfSeedResult{}, err
	}

	return PerfSeedResult{
		Folders:    len(folders),
		Categories: len(categories),
		TestCases:  cfg.TestCases,
		TestRuns:   len(runs),
		RunResults: len(results),
		IngestPool: pool,
	}, nil
}

// PerfPrincipals carries raw load-test credentials. Raw token values exist
// only in memory here and in the manifest file cmd/perfseed writes (0600,
// gitignored) — they are never persisted server-side (only hashes are).
type PerfPrincipals struct {
	Tokens     []string // raw write-scoped bearer tokens
	UserEmails []string
}

// SeedPerfPrincipals creates nUsers member users (sharing one bcrypt hash of
// password — they exist for connection-cap fan-out, not for auth strength)
// and nTokens write-scoped API tokens for load-test clients. Tokens rather
// than logins because POST /api/auth/login is rate-limited per IP.
func (s *Store) SeedPerfPrincipals(nUsers, nTokens int, password string) (PerfPrincipals, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return PerfPrincipals{}, err
	}
	var p PerfPrincipals
	for i := 1; i <= nUsers; i++ {
		email := fmt.Sprintf("perf-user-%02d@perf.local", i)
		if _, err := s.CreateUser(email, fmt.Sprintf("Perf User %02d", i), string(hash), "member"); err != nil {
			return PerfPrincipals{}, fmt.Errorf("create perf user %s: %w", email, err)
		}
		p.UserEmails = append(p.UserEmails, email)
	}
	for i := 1; i <= nTokens; i++ {
		_, raw, err := s.CreateToken(fmt.Sprintf("perf-token-%02d", i), "write", nil)
		if err != nil {
			return PerfPrincipals{}, fmt.Errorf("create perf token %d: %w", i, err)
		}
		p.Tokens = append(p.Tokens, raw)
	}
	return p, nil
}
