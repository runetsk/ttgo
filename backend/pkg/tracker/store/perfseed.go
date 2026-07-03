package store

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PerfSeedConfig controls the size and shape of a generated performance
// dataset. Results is a total distributed evenly across Runs, so the created
// row count is Runs * (Results / Runs).
type PerfSeedConfig struct {
	Seed            uint64  // drives pseudo-random choices and deterministic entity IDs
	Folders         int     // area folders under the "Perf" root folder
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
