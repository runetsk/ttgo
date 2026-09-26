package worker

import (
	"context"
	"testing"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// Three failures collapse into one group: the job bills its semantic pass, then one TypeSafe
// and one LLM call for the representative. The two clones bill nothing.
func TestWorker_RecordsACostEventPerBillableCall(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
		{"timeout", "Timeout waiting for #checkout button after 7000ms"},
	})
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	ts := &fakeTS{fn: tsVerdict("flaky_test", "automation_bug", 0.95)}
	in, out := 1.0, 2.0
	deps := failureanalysis.JobDeps{
		Narrative: &verdictProvider{verdict: "product_bug"}, NarrativeModel: "mock",
		Decider: failureanalysis.NewTypeSafeDecider(ts, "jev-1.13.0"), DeciderModel: "jev-1.13.0",
		Semantic: &failureanalysis.SemanticDeps{Client: ts, Model: "jev-1.13.0", Redact: true},
		Pricing:  failureanalysis.Pricing{LLMProviderID: "prov-1", LLMPromptPerMTok: &in, LLMCompletionPerMTok: &out, TypeSafePerMTok: 0.042},
	}
	w := NewWorker(s, staticResolver(deps), nil, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	events, err := s.ListAnalysisCostEventsForRun(run.ID)
	require.NoError(t, err)
	require.Len(t, events, 3)
	byKey := map[string]*models.AIAnalysisCostEvent{}
	for _, e := range events {
		byKey[e.Kind+"/"+e.Engine] = e
		require.Equal(t, job.ID, *e.JobID)
	}

	sem := byKey["semantic/typesafe"]
	require.NotNil(t, sem)
	require.Equal(t, 500, sem.TypeSafeInputTokens)
	require.Nil(t, sem.AnalysisID)
	require.InDelta(t, 500*0.042/1e6, *sem.EstimatedCost, 1e-12)

	tsEv := byKey["analysis/typesafe"]
	require.NotNil(t, tsEv)
	require.Equal(t, 500, tsEv.TypeSafeInputTokens)
	require.NotNil(t, tsEv.AnalysisID)
	rep, err := s.GetAnalysisByID(*tsEv.AnalysisID)
	require.NoError(t, err)
	require.Nil(t, rep.SourceAnalysisID, "events point at the representative, never a clone")

	llmEv := byKey["analysis/llm"]
	require.NotNil(t, llmEv)
	require.Equal(t, 10, llmEv.PromptTokens)
	require.Equal(t, 5, llmEv.CompletionTokens)
	require.Equal(t, "prov-1", *llmEv.ProviderID)
	require.InDelta(t, (10*1.0+5*2.0)/1e6, *llmEv.EstimatedCost, 1e-12)
	require.Equal(t, *tsEv.AnalysisID, *llmEv.AnalysisID)
}
