package store

import (
	"testing"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func pf(v float64) *float64 { return &v }

func TestSaveSemanticPairs_StoresOrientedRows(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.SaveSemanticPairs("job-1", "run-1", "jev-latest", failureanalysis.SemanticPolicyVersion,
		[]failureanalysis.SemanticPairOutcome{
			{SigA: "zz", SigB: "aa", ResultA: "r-z", ResultB: "r-a", P: pf(0.91), Source: failureanalysis.SemanticSourceTypeSafe,
				AnsweredModel: "jev-1.13", Merged: true},
			{SigA: "aa", SigB: "bb", ResultA: "r-a", ResultB: "r-b", P: pf(0), Source: failureanalysis.SemanticSourceHuman},
		}))
	rows, err := s.ListSemanticPairsForJob("job-1")
	require.NoError(t, err)
	require.Len(t, rows, 2)
	byB := map[string]*models.SemanticPair{}
	for _, r := range rows {
		byB[r.SigB] = r
	}
	z := byB["zz"]
	require.Equal(t, "aa", z.SigA)
	require.Equal(t, "r-a", z.ResultAID, "result ids follow the signatures")
	require.Equal(t, "r-z", z.ResultBID)
	require.InDelta(t, 0.91, *z.PSame, 1e-12)
	require.True(t, z.Merged)
	require.Equal(t, "jev-latest", z.Model)
	require.Equal(t, "jev-1.13", z.AnsweredModel)
	require.Nil(t, byB["bb"].PSame, "a person's split has no probability")

	got, err := s.SemanticPairFor("job-1", "zz", "aa")
	require.NoError(t, err)
	require.Equal(t, z.ID, got.ID)
	none, err := s.SemanticPairFor("job-1", "aa", "bb")
	require.NoError(t, err)
	require.Nil(t, none, "human rows are not the job's decision")
}

func insertPair(t *testing.T, s *Store, a, b, source, model string, p *float64, at time.Time) *models.SemanticPair {
	t.Helper()
	row := &models.SemanticPair{ID: a + b + source + at.String(), JobID: "j", RunID: "r", SigA: a, SigB: b, PSame: p,
		Model: model, PolicyVersion: failureanalysis.SemanticPolicyVersion, Source: source, CreatedAt: at}
	require.NoError(t, s.db.Create(row).Error)
	return row
}

func TestSemanticMemory_Rules(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()
	ts, mem, human := failureanalysis.SemanticSourceTypeSafe, failureanalysis.SemanticSourceMemory, failureanalysis.SemanticSourceHuman

	insertPair(t, s, "a", "b", ts, "jev-latest", pf(0.4), now.AddDate(0, 0, -10))
	insertPair(t, s, "a", "b", ts, "jev-latest", pf(0.9), now.AddDate(0, 0, -2))  // newest wins
	insertPair(t, s, "a", "c", ts, "jev-latest", pf(0.9), now.AddDate(0, 0, -31)) // too old
	insertPair(t, s, "a", "d", ts, "jev-other", pf(0.9), now.AddDate(0, 0, -1))   // other model
	insertPair(t, s, "a", "e", mem, "jev-latest", pf(0.9), now.AddDate(0, 0, -1)) // memory is not a source
	insertPair(t, s, "a", "f", ts, "jev-latest", pf(0.95), now.AddDate(0, 0, -1))
	insertPair(t, s, "a", "f", human, "", nil, now.AddDate(-1, 0, 0)) // a year-old split still wins
	row := insertPair(t, s, "a", "g", ts, "jev-latest", pf(0.8), now.AddDate(0, 0, -1))
	row.PolicyVersion = "fa-semantic-v0"
	require.NoError(t, s.db.Save(row).Error)

	remember := s.SemanticMemory("jev-latest", failureanalysis.SemanticPolicyVersion, now)
	r, ok := remember("b", "a")
	require.True(t, ok, "orientation does not matter")
	require.InDelta(t, 0.9, r.P, 1e-12)
	require.Equal(t, ts, r.Source)
	for _, sig := range []string{"c", "d", "e", "g", "zz"} {
		_, ok := remember("a", sig)
		require.False(t, ok, sig)
	}
	r, ok = remember("a", "f")
	require.True(t, ok)
	require.Equal(t, human, r.Source)
}

func TestRememberedSource(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC()
	src := insertPair(t, s, "a", "b", failureanalysis.SemanticSourceTypeSafe, "jev-latest", pf(0.9), now.Add(-time.Hour))
	m := insertPair(t, s, "a", "b", failureanalysis.SemanticSourceMemory, "jev-latest", pf(0.9), now)
	got, err := s.RememberedSource(m)
	require.NoError(t, err)
	require.Equal(t, src.ID, got.ID)
	none, err := s.RememberedSource(src)
	require.NoError(t, err)
	require.Nil(t, none)
}

func TestSetAnalysisJobSemanticReport(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	job, _, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	require.Equal(t, "", job.SemanticReport)
	require.NoError(t, s.SetAnalysisJobSemanticReport(job.ID, `{"asked":2}`))
	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, `{"asked":2}`, got.SemanticReport)
}
