package store

import (
	"sync"
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func TestCreateAnalysisBumpsVersion(t *testing.T) {
	s := newTestStore(t)
	rr := seedFailingResult(t, s)

	a1, err := s.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID: rr.ID, Verdict: models.VerdictFlakyTest, Confidence: models.ConfidenceMedium,
		Summary: "first", ModelName: "x",
	})
	require.NoError(t, err)
	require.Equal(t, 1, a1.Version)

	a2, err := s.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID: rr.ID, Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh,
		Summary: "second", ModelName: "x",
	})
	require.NoError(t, err)
	require.Equal(t, 2, a2.Version)
}

func TestCreateAnalysisVersionUnderConcurrentWrites(t *testing.T) {
	// Use a temp-file DB — :memory: with default GORM pooling hands each
	// connection a distinct empty DB, which makes a true concurrency test
	// impossible.
	dir := t.TempDir()
	s, err := New(dir + "/test.db")
	require.NoError(t, err)
	// Close the DB before t.TempDir() removal: Windows can't delete an open SQLite file.
	t.Cleanup(func() { _ = s.Close() })
	rr := seedFailingResult(t, s)

	const N = 10
	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.CreateAnalysis(&models.RunResultAnalysis{
				RunResultID: rr.ID, Verdict: models.VerdictUnknown, Confidence: models.ConfidenceLow,
				Summary: "concurrent", ModelName: "x",
			})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}

	list, err := s.ListAnalysesForResult(rr.ID)
	require.NoError(t, err)
	require.Len(t, list, N)
	seen := map[int]bool{}
	for _, a := range list {
		require.False(t, seen[a.Version], "duplicate version %d", a.Version)
		seen[a.Version] = true
		require.GreaterOrEqual(t, a.Version, 1)
		require.LessOrEqual(t, a.Version, N)
	}
}

func TestGetCurrentAnalysesByRun_EmptyMapForNoAnalyses(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	m, err := s.GetCurrentAnalysesByRun(runID)
	require.NoError(t, err)
	require.Empty(t, m)
}

func TestGetCurrentAnalysesByRun_ReturnsHighestVersionPerResult(t *testing.T) {
	s := newTestStore(t)
	rr := seedFailingResult(t, s)

	_, err := s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: rr.ID, Verdict: "flaky_test", Confidence: "low", ModelName: "m"})
	require.NoError(t, err)
	a2, err := s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: rr.ID, Verdict: "product_bug", Confidence: "high", ModelName: "m"})
	require.NoError(t, err)

	m, err := s.GetCurrentAnalysesByRun(rr.TestRunID)
	require.NoError(t, err)
	require.Len(t, m, 1)
	require.Equal(t, a2.ID, m[rr.ID].ID)
	require.Equal(t, 2, m[rr.ID].Version)
}

func seedRun(t *testing.T, s *Store) string {
	t.Helper()
	run := &models.TestRun{Name: "r"}
	require.NoError(t, s.CreateTestRun(run))
	return run.ID
}

func seedFailingResult(t *testing.T, s *Store) *models.RunResult {
	t.Helper()
	runID := seedRun(t, s)
	rr := &models.RunResult{
		TestRunID: runID, TestNameSnapshot: "t",
		AttemptNumber: 1, Status: models.StatusFail, ErrorMessage: "boom",
	}
	require.NoError(t, s.AddRunResult(rr))
	return rr
}
