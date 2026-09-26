package store

import (
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func addFailingResult(t *testing.T, s *Store, runID string) *models.RunResult {
	t.Helper()
	rr := &models.RunResult{TestRunID: runID, TestNameSnapshot: "t", AttemptNumber: 1, Status: models.StatusFail, ErrorMessage: "boom"}
	require.NoError(t, s.AddRunResult(rr))
	return rr
}

func TestAnalysisJobOutcomes_CountsRepresentativesAndFailedRows(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	job, _, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	other := "another-job"
	jobID := job.ID

	create := func(a *models.RunResultAnalysis) *models.RunResultAnalysis {
		a.RunResultID = addFailingResult(t, s, runID).ID
		if a.JobID == nil {
			a.JobID = &jobID
		}
		out, err := s.CreateAnalysis(a)
		require.NoError(t, err)
		return out
	}
	decided := create(&models.RunResultAnalysis{Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh})
	create(&models.RunResultAnalysis{Verdict: models.VerdictUnknown, Confidence: models.ConfidenceLow})
	create(&models.RunResultAnalysis{Verdict: models.VerdictFlakyTest, Confidence: models.ConfidenceHigh, NarrativeStatus: models.NarrativeStatusSkipped})
	create(&models.RunResultAnalysis{Verdict: models.VerdictEnvironment, Confidence: models.ConfidenceMedium, NarrativeStatus: models.NarrativeStatusUnavailable})
	create(&models.RunResultAnalysis{Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, TakeoverFromVerdict: models.VerdictUnknown})
	failed := create(&models.RunResultAnalysis{Verdict: models.VerdictUnknown, Confidence: models.ConfidenceLow,
		DecisionStatus: models.DecisionStatusFailed, ErrorCategory: "timeout", NarrativeStatus: models.NarrativeStatusUnavailable})
	// Clones follow their representative: only FailedRows counts them.
	create(&models.RunResultAnalysis{Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, SourceAnalysisID: &decided.ID})
	create(&models.RunResultAnalysis{Verdict: models.VerdictUnknown, Confidence: models.ConfidenceLow,
		DecisionStatus: models.DecisionStatusFailed, SourceAnalysisID: &failed.ID})
	// Another job's rows are not counted.
	create(&models.RunResultAnalysis{Verdict: models.VerdictUnknown, Confidence: models.ConfidenceLow, DecisionStatus: models.DecisionStatusFailed, JobID: &other})

	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobOutcomes{
		Groups: 6, Decided: 5, Failed: 1, Unknown: 1, NoExplanation: 1, ExplanationSkipped: 1, TakenOver: 1, FailedRows: 2,
	}, o)

	empty, err := s.AnalysisJobOutcomes("no-such-job")
	require.NoError(t, err)
	require.Zero(t, empty)
}

func TestFailedResultIDsForRun_UsesTheCurrentVersion(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	fixed := addFailingResult(t, s, runID)
	still := addFailingResult(t, s, runID)
	for _, id := range []string{fixed.ID, still.ID} {
		_, err := s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: id, Verdict: models.VerdictUnknown,
			Confidence: models.ConfidenceLow, DecisionStatus: models.DecisionStatusFailed})
		require.NoError(t, err)
	}
	_, err := s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: fixed.ID, Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh})
	require.NoError(t, err)

	ids, err := s.FailedResultIDsForRun(runID)
	require.NoError(t, err)
	require.Equal(t, map[string]bool{still.ID: true}, ids, "a later successful version clears the failure")
}

func TestUpdateAnalysisNarrative_KeepsDecisionAndAddsUsage(t *testing.T) {
	s := newTestStore(t)
	rr := seedFailingResult(t, s)
	score := 0.95
	a, err := s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: rr.ID, Engine: models.AnalysisEngineTypeSafe,
		Verdict: models.VerdictFlakyTest, Confidence: models.ConfidenceHigh, ConfidenceScore: &score,
		NarrativeStatus: models.NarrativeStatusSkipped, TokenUsagePrompt: 10, LLMCalls: 1, LLMMs: 100})
	require.NoError(t, err)

	got, err := s.UpdateAnalysisNarrative(a.ID, NarrativeUpdate{Summary: "S", NextAction: "N", Rationale: "R",
		NarrativeStatus: models.NarrativeStatusOK, AddPrompt: 300, AddCompletion: 40, AddLLMMs: 900, AddLLMCalls: 1, FinishReason: "stop"})
	require.NoError(t, err)
	require.Equal(t, "S", got.Summary)
	require.Equal(t, models.NarrativeStatusOK, got.NarrativeStatus)
	require.Equal(t, 310, got.TokenUsagePrompt)
	require.Equal(t, 40, got.TokenUsageCompletion)
	require.Equal(t, 1000, got.LLMMs)
	require.Equal(t, 2, got.LLMCalls)
	require.Equal(t, "stop", got.FinishReason)
	require.Equal(t, models.VerdictFlakyTest, got.Verdict, "the decision is untouched")
	require.Equal(t, a.Version, got.Version, "an explanation is not a new version")
}

func TestEnqueueRetryFailedForRun(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	job, created, err := s.EnqueueRetryFailedForRun(runID, "")
	require.NoError(t, err)
	require.True(t, created)
	require.True(t, job.RetryFailedOnly)

	again, created, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	require.False(t, created, "one active job per run, whatever its kind")
	require.Equal(t, job.ID, again.ID)

	require.NoError(t, s.SetAnalysisJobPipeline(job.ID, `{"decider":"jev"}`, "TypeSafe jev, no LLM"))
	jobs, err := s.ListAnalysisJobsForRun(runID)
	require.NoError(t, err)
	require.Len(t, jobs, 1)
	require.Equal(t, "TypeSafe jev, no LLM", jobs[0].PipelineLabel)
	require.JSONEq(t, `{"decider":"jev"}`, jobs[0].Pipeline)
}

func TestBackfillFailedAnalyses(t *testing.T) {
	s := newTestStore(t)
	rr := seedFailingResult(t, s)
	mk := func(a *models.RunResultAnalysis) string {
		a.RunResultID = rr.ID
		out, err := s.CreateAnalysis(a)
		require.NoError(t, err)
		return out.ID
	}
	providerErr := mk(&models.RunResultAnalysis{Verdict: models.VerdictUnknown, Confidence: models.ConfidenceLow, Summary: "analysis failed: provider 502"})
	unparseable := mk(&models.RunResultAnalysis{Engine: models.AnalysisEngineGenerative, ModelName: "m", Verdict: models.VerdictUnknown,
		Confidence: models.ConfidenceLow, Summary: "AI returned unparseable response after retry"})
	realUnknown := mk(&models.RunResultAnalysis{Engine: models.AnalysisEngineGenerative, ModelName: "m", Verdict: models.VerdictUnknown,
		Confidence: models.ConfidenceLow, Summary: "Not enough evidence to classify."})

	require.NoError(t, s.backfillFailedAnalyses())
	require.NoError(t, s.backfillFailedAnalyses(), "idempotent")

	check := func(id, status, category string) {
		a, err := s.GetAnalysisByID(id)
		require.NoError(t, err)
		require.Equal(t, status, a.DecisionStatus, a.Summary)
		require.Equal(t, category, a.ErrorCategory, a.Summary)
	}
	check(providerErr, models.DecisionStatusFailed, "error")
	check(unparseable, models.DecisionStatusFailed, "unparseable")
	check(realUnknown, models.DecisionStatusOK, "")
}
func TestAnalysisJobOutcomes_CountsConfigurationFailures(t *testing.T) {
	s := newTestStore(t)
	runID := seedRun(t, s)
	job, _, err := s.MaybeEnqueueForRun(runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	mk := func(category string, source *string) *models.RunResultAnalysis {
		out, err := s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: addFailingResult(t, s, runID).ID, JobID: &job.ID,
			Verdict: models.VerdictUnknown, Confidence: models.ConfidenceLow, DecisionStatus: models.DecisionStatusFailed,
			ErrorCategory: category, SourceAnalysisID: source})
		require.NoError(t, err)
		return out
	}
	rep := mk("configuration", nil)
	mk("configuration", &rep.ID) // a clone follows its representative and is not counted again
	mk("timeout", nil)

	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, 2, o.Failed)
	require.Equal(t, 1, o.FailedConfiguration)
	require.Equal(t, 3, o.FailedRows)
}
