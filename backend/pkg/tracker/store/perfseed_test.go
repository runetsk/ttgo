package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
