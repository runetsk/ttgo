package worker

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

// pairCounter wraps tsVerdict and counts the same-cause questions TypeSafe was asked.
func pairCounter(pairs *atomic.Int32) *fakeTS {
	base := tsVerdict("flaky_test", "automation_bug", 0.95)
	return &fakeTS{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		for id := range req.Questions {
			if strings.HasPrefix(id, "pair_") {
				pairs.Add(1)
			}
		}
		return base(req)
	}}
}

func semanticDeps(s *store.Store, ts *fakeTS) failureanalysis.JobDeps {
	return failureanalysis.JobDeps{
		Decider: failureanalysis.NewTypeSafeDecider(ts, "jev-1.13.0"), DeciderModel: "jev-1.13.0", NarrativeSkipped: true,
		Semantic: &failureanalysis.SemanticDeps{Client: ts, Model: "jev-latest",
			Remember: s.SemanticMemory("jev-latest", failureanalysis.SemanticPolicyVersion, time.Now().Add(time.Hour))},
	}
}

func dedupSettings(t *testing.T, s *store.Store) {
	t.Helper()
	_, err := s.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		MaxAnalysesPerRun: 10, DedupEnabled: true, RedactionEnabled: false,
		PromptTemplate: failureanalysis.DefaultPromptTemplate, ParallelGroups: 1,
	})
	require.NoError(t, err)
}

func runJob(t *testing.T, s *store.Store, runID string, deps failureanalysis.JobDeps) *models.RunAnalysisJob {
	t.Helper()
	job, created, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	require.True(t, created)
	require.NoError(t, NewWorker(s, staticResolver(deps), nil, 10*time.Millisecond).processOnce(context.Background()))
	return job
}

func TestWorker_RecordsSemanticPairsAndReusesThemNextJob(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
		{"timeout", "Timeout waiting for #checkout button after 7000ms"},
	})
	dedupSettings(t, s)

	var asked atomic.Int32
	ts := pairCounter(&asked)
	job1 := runJob(t, s, run.ID, semanticDeps(s, ts))
	require.EqualValues(t, 1, asked.Load())
	rows, err := s.ListSemanticPairsForJob(job1.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, failureanalysis.SemanticSourceTypeSafe, rows[0].Source)
	require.True(t, rows[0].Merged)
	require.Equal(t, "jev-latest", rows[0].Model)
	require.Equal(t, "jev-1.13.0", rows[0].AnsweredModel)
	got, err := s.GetAnalysisJob(job1.ID)
	require.NoError(t, err)
	var rep map[string]int
	require.NoError(t, json.Unmarshal([]byte(got.SemanticReport), &rep))
	require.Equal(t, 1, rep["asked"])
	require.Equal(t, 1, rep["merged"])

	asked.Store(0)
	job2 := runJob(t, s, run.ID, semanticDeps(s, ts))
	require.Zero(t, asked.Load(), "the pair is remembered, not asked again")
	rows, err = s.ListSemanticPairsForJob(job2.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, failureanalysis.SemanticSourceMemory, rows[0].Source)
	require.True(t, rows[0].Merged)
	got, err = s.GetAnalysisJob(job2.ID)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal([]byte(got.SemanticReport), &rep))
	require.Equal(t, 1, rep["remembered"])
	require.Zero(t, rep["asked"])

	current, err := s.GetCurrentAnalysesByRun(run.ID)
	require.NoError(t, err)
	semantic := 0
	for _, a := range current {
		if a.DedupMethod == models.DedupMethodSemantic {
			semantic++
			require.Equal(t, "jev-1.13.0", a.DedupModel, "a remembered merge names the model that answered")
		}
	}
	require.Equal(t, 1, semantic)
}

func TestWorker_AFailedSemanticPassRecordsNothing(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
		{"timeout", "Timeout waiting for #checkout button after 7000ms"},
	})
	dedupSettings(t, s)
	base := tsVerdict("flaky_test", "automation_bug", 0.95)
	ts := &fakeTS{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		for id := range req.Questions {
			if strings.HasPrefix(id, "pair_") {
				return nil, &typesafe.Error{Status: 500, Message: "down"}
			}
		}
		return base(req)
	}}
	job := runJob(t, s, run.ID, semanticDeps(s, ts))
	rows, err := s.ListSemanticPairsForJob(job.ID)
	require.NoError(t, err)
	require.Empty(t, rows)
	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, "", got.SemanticReport)
}

func TestWorker_AScopedJobAnalyzesOnlyItsResults(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
		{"assertion", "expected total 10 to equal 12"},
		{"assertion", "expected total 10 to equal 12"},
	})
	dedupSettings(t, s)
	failing, err := s.ListLatestFailingResults(run.ID)
	require.NoError(t, err)
	var scoped []string
	for _, r := range failing {
		if r.FailureType == "assertion" {
			scoped = append(scoped, r.ID)
		}
	}
	require.Len(t, scoped, 2)
	job, created, err := s.EnqueueScopedAnalysis(run.ID, "", scoped, "", nil)
	require.NoError(t, err)
	require.True(t, created)
	var asked atomic.Int32
	require.NoError(t, NewWorker(s, staticResolver(semanticDeps(s, pairCounter(&asked))), nil, 10*time.Millisecond).
		processOnce(context.Background()))

	current, err := s.GetCurrentAnalysesByRun(run.ID)
	require.NoError(t, err)
	require.Len(t, current, 2, "only the scoped results were analyzed")
	for _, id := range scoped {
		require.NotNil(t, current[id])
		require.Equal(t, job.ID, *current[id].JobID)
	}
	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, 2, got.TotalFailures)
	require.Equal(t, 1, got.UniqueGroups)
}
