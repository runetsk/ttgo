package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func TestPerfTierSmall(t *testing.T) {
	cfg, err := PerfTier("small")
	require.NoError(t, err)
	assert.Equal(t, uint64(1), cfg.Seed)
	assert.Equal(t, 800, cfg.TestCases)
	assert.Equal(t, 10_000, cfg.Results)
	assert.Equal(t, 50, cfg.Runs) // must divide Results exactly: 50 × 200 = 10,000
	assert.Equal(t, 500, cfg.IngestPoolCases)
	assert.Equal(t, 90, cfg.DaysSpread)
}

func TestPerfTierUnknown(t *testing.T) {
	_, err := PerfTier("huge")
	assert.ErrorContains(t, err, "unknown perf tier")
}

func TestIsPerfDBPath(t *testing.T) {
	assert.True(t, IsPerfDBPath("perf-small.db"))
	assert.True(t, IsPerfDBPath("/tmp/scratch/perf-anything.db"))
	assert.False(t, IsPerfDBPath("tracker.db"))
	assert.False(t, IsPerfDBPath("perf-small.db.bak"))
	assert.False(t, IsPerfDBPath(":memory:"))
	assert.False(t, IsPerfDBPath("/data/perf/production.db")) // perf dir name is not enough
}

func smallTestCfg() PerfSeedConfig {
	cfg, _ := PerfTier("small")
	// Shrink for test speed; ratios stay realistic.
	cfg.Folders = 4
	cfg.Categories = 3
	cfg.TestCases = 40
	cfg.Runs = 6
	cfg.Results = 180 // 30 per run — must stay ≤ TestCases (distinct per run)
	cfg.IngestPoolCases = 25
	return cfg
}

func TestSeedPerfDatasetCounts(t *testing.T) {
	s := newTestStore(t)
	cfg := smallTestCfg()

	res, err := s.SeedPerfDataset(cfg)
	require.NoError(t, err)

	assert.Equal(t, cfg.Folders+2, res.Folders)       // + "Perf" root + "Perf Ingest Pool"
	assert.Equal(t, cfg.Categories+1, res.Categories) // + "perf-ingest"
	assert.Equal(t, cfg.TestCases, res.TestCases)
	assert.Equal(t, cfg.Runs, res.TestRuns)
	assert.Equal(t, cfg.Runs*(cfg.Results/cfg.Runs), res.RunResults)
	require.Len(t, res.IngestPool, cfg.IngestPoolCases)
	assert.Equal(t, "Ingest TC 0001", res.IngestPool[0].Name)

	// The ingest pool must have zero historical results — load tests write there.
	poolIDs := make([]string, len(res.IngestPool))
	for i, c := range res.IngestPool {
		poolIDs[i] = c.ID
	}
	var poolResults int64
	err = s.db.Model(&models.RunResult{}).
		Where("test_case_id IN ?", poolIDs).
		Count(&poolResults).Error
	require.NoError(t, err)
	assert.Zero(t, poolResults)

	// Every result row is well-formed for analytics: run FK + snapshot name set.
	var badRows int64
	err = s.db.Model(&models.RunResult{}).
		Where("test_run_id = '' OR test_name_snapshot = ''").
		Count(&badRows).Error
	require.NoError(t, err)
	assert.Zero(t, badRows)

	// Both PASS and FAIL results exist (analytics needs signal, not all-green).
	var failCount int64
	err = s.db.Model(&models.RunResult{}).Where("status = ?", models.StatusFail).Count(&failCount).Error
	require.NoError(t, err)
	assert.Positive(t, failCount)
}

func TestSeedPerfDatasetDeterministic(t *testing.T) {
	s1 := newTestStore(t)
	s2 := newTestStore(t)
	cfg := smallTestCfg()

	res1, err := s1.SeedPerfDataset(cfg)
	require.NoError(t, err)
	res2, err := s2.SeedPerfDataset(cfg)
	require.NoError(t, err)

	// Same seed => identical IDs (uuid.NewSHA1 over seed+kind+index).
	assert.Equal(t, res1.IngestPool, res2.IngestPool)

	// Different seed => disjoint IDs.
	s3 := newTestStore(t)
	cfg.Seed = 2
	res3, err := s3.SeedPerfDataset(cfg)
	require.NoError(t, err)
	assert.NotEqual(t, res1.IngestPool[0].ID, res3.IngestPool[0].ID)
}

func TestSeedPerfDatasetRejectsImpossibleConfig(t *testing.T) {
	s := newTestStore(t)
	cfg := smallTestCfg()
	cfg.Results = cfg.TestCases * cfg.Runs * 2 // per-run results would exceed distinct cases
	_, err := s.SeedPerfDataset(cfg)
	assert.ErrorContains(t, err, "results per run")
}

func TestSeedPerfPrincipals(t *testing.T) {
	s := newTestStore(t)

	p, err := s.SeedPerfPrincipals(3, 5, "pw-for-tests")
	require.NoError(t, err)
	require.Len(t, p.Tokens, 5)
	require.Len(t, p.UserEmails, 3)

	// Raw tokens must validate as write-scoped.
	tok, err := s.ValidateToken(p.Tokens[0])
	require.NoError(t, err)
	require.NotNil(t, tok)
	assert.Equal(t, "write", tok.Scope)

	// Users exist, active, member role.
	u, err := s.FindUserByEmail("perf-user-01@perf.local")
	require.NoError(t, err)
	require.NotNil(t, u)
	assert.True(t, u.Active)
	assert.Equal(t, "member", u.Role)
}

func TestSeedPerfDatasetRejectsNonPositiveCounts(t *testing.T) {
	s := newTestStore(t)
	for name, mutate := range map[string]func(*PerfSeedConfig){
		"folders":     func(c *PerfSeedConfig) { c.Folders = 0 },
		"categories":  func(c *PerfSeedConfig) { c.Categories = -1 },
		"ingest pool": func(c *PerfSeedConfig) { c.IngestPoolCases = 0 },
	} {
		cfg := smallTestCfg()
		mutate(&cfg)
		_, err := s.SeedPerfDataset(cfg)
		assert.ErrorContains(t, err, "must be positive", name)
	}
}

func TestSeedPerfDatasetRejectsUnevenResults(t *testing.T) {
	s := newTestStore(t)
	cfg := smallTestCfg()
	cfg.Results = cfg.Runs*30 + 7 // would silently truncate 7 results today
	_, err := s.SeedPerfDataset(cfg)
	assert.ErrorContains(t, err, "divisible")
}

func TestPerfTierMediumAndLarge(t *testing.T) {
	med, err := PerfTier("medium")
	require.NoError(t, err)
	assert.Equal(t, 100_000, med.Results)
	assert.Equal(t, 200, med.Runs) // 500 results per run
	assert.Equal(t, 2000, med.TestCases)

	lg, err := PerfTier("large")
	require.NoError(t, err)
	assert.Equal(t, 1_000_000, lg.Results)
	assert.Equal(t, 1000, lg.Runs) // 1000 results per run
	assert.Equal(t, 8000, lg.TestCases)
}

// Every tier must satisfy the seeder's own guards and the k6 contract.
func TestPerfTierInvariants(t *testing.T) {
	for _, name := range []string{"small", "medium", "large"} {
		cfg, err := PerfTier(name)
		require.NoError(t, err, name)
		require.Positive(t, cfg.Runs, name)
		assert.Zero(t, cfg.Results%cfg.Runs, "%s: results must divide runs evenly", name)
		assert.LessOrEqual(t, cfg.Results/cfg.Runs, cfg.TestCases,
			"%s: per-run results must not exceed distinct cases", name)
		assert.Equal(t, 500, cfg.IngestPoolCases,
			"%s: ingest pool size is a contract with k6 RESULTS_PER_RUN validation", name)
	}
}

func TestSeedPerfDatasetHistoricalRunIDs(t *testing.T) {
	s := newTestStore(t)
	res, err := s.SeedPerfDataset(smallTestCfg())
	require.NoError(t, err)
	// smallTestCfg has 6 runs — fewer than the 50 cap, so all are exported.
	require.Len(t, res.HistoricalRunIDs, 6)
	assert.Equal(t, perfID(1, "run", 0), res.HistoricalRunIDs[0])
}

func TestSeedPerfDatasetCapsHistoricalRunIDs(t *testing.T) {
	s := newTestStore(t)
	cfg := smallTestCfg()
	cfg.Runs = 60
	cfg.Results = 60 // 1 per run; still ≤ TestCases
	res, err := s.SeedPerfDataset(cfg)
	require.NoError(t, err)
	assert.Len(t, res.HistoricalRunIDs, 50)
}
