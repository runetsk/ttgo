package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// stallsFirst blocks its first call until that call's context ends and answers every later one.
type stallsFirst struct{ calls atomic.Int32 }

func (p *stallsFirst) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if p.calls.Add(1) == 1 {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return (&verdictProvider{verdict: "product_bug"}).Chat(ctx, req)
}

func TestJobGroupDeadline_FollowsTheJobTimeouts(t *testing.T) {
	deps := failureanalysis.JobDeps{TypeSafeTimeout: 30 * time.Second, LLMCallTimeout: 45 * time.Second}
	require.Equal(t, 570*time.Second, jobGroupDeadline(deps))
	require.Equal(t, failureanalysis.MinGroupDeadline, jobGroupDeadline(failureanalysis.JobDeps{}))

	old := GroupDeadline
	GroupDeadline = 50 * time.Millisecond
	defer func() { GroupDeadline = old }()
	require.Equal(t, 50*time.Millisecond, jobGroupDeadline(deps), "a lowered GroupDeadline (tests) caps every group")
}

func TestWorker_RecordsCallTimeouts(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"assertion", "expected 1 got 2"}})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	deps := failureanalysis.JobDeps{Narrative: llm.WithCallTimeout(&stallsFirst{}, 50*time.Millisecond), NarrativeModel: "mock"}
	w := NewWorker(s, staticResolver(deps), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobStatusCompleted, got.Status)
	require.Equal(t, 1, got.CallTimeouts, "the stalled call was cut once and the retry answered")
	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, 1, o.Decided)
	require.Equal(t, 1, o.CallTimeouts)
}

func TestWorker_RecordsHedgesAndTheirCost(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"assertion", "expected 1 got 2"}})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	in, out := 1.0, 2.0
	deps := failureanalysis.JobDeps{
		Narrative: llm.WithHedge(&stallsFirst{}, 20*time.Millisecond), NarrativeModel: "mock", HedgingOn: true,
		Pricing: failureanalysis.Pricing{LLMProviderID: "prov-1", LLMPromptPerMTok: &in, LLMCompletionPerMTok: &out},
	}
	w := NewWorker(s, staticResolver(deps), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobStatusCompleted, got.Status)
	require.Equal(t, 1, got.HedgesFired)
	require.Equal(t, 1, got.HedgesWon)
	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, 1, o.HedgesFired)
	require.Equal(t, 1, o.HedgesWon)

	events, err := s.ListAnalysisCostEventsForRun(run.ID)
	require.NoError(t, err)
	var hedge *models.AIAnalysisCostEvent
	for _, e := range events {
		if e.Kind == models.AnalysisCostKindHedge {
			require.Nil(t, hedge, "one event per fired hedge")
			hedge = e
		}
	}
	require.NotNil(t, hedge)
	require.Equal(t, models.AnalysisCostEngineLLM, hedge.Engine)
	require.Equal(t, 10, hedge.PromptTokens, "priced at the winner's prompt")
	require.InDelta(t, 10*1.0/1e6, *hedge.EstimatedCost, 1e-12)
	require.Equal(t, job.ID, *hedge.JobID)
	require.Nil(t, hedge.AnalysisID, "job-level: the counter is shared by the job's groups")
}
