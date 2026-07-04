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
