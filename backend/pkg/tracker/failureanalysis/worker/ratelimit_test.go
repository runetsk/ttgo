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

// rateLimitedOnce answers 429 to its first call and a verdict afterwards.
type rateLimitedOnce struct{ calls atomic.Int32 }

func (p *rateLimitedOnce) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if p.calls.Add(1) == 1 {
		return nil, &llm.ProviderError{StatusCode: 429, Category: llm.ErrCatRateLimit, Message: "slow down"}
	}
	return (&verdictProvider{verdict: "product_bug"}).Chat(ctx, req)
}

func TestWorker_CountsRateLimitHitsIncludingRetriedOnes(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"assertion", "expected 1 got 2"}})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	w := NewWorker(s, staticResolver(failureanalysis.JobDeps{Narrative: &rateLimitedOnce{}, NarrativeModel: "mock"}), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	got, err := s.GetAnalysisJob(job.ID)
	require.NoError(t, err)
	require.Equal(t, models.RunAnalysisJobStatusCompleted, got.Status)
	require.Equal(t, 1, got.RateLimitHits, "the 429 counts although the retry succeeded")
	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, 1, o.Decided)
	require.Equal(t, 1, o.RateLimitHits)
}
