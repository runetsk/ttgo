package failureanalysis

import (
	"testing"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func TestGroupDeadlineFor_SpecFormulaWithAFiveMinuteFloor(t *testing.T) {
	// 3 × 30 s + 2 × 30 s + 4 × 45 s + 2 × 30 s + 30 s + (3 × 30 s + 60 s) = 570 s: the defaults,
	// the last term being the narrative transfer check (one request, 3 attempts, 2 backoffs; R10).
	require.Equal(t, 570*time.Second, GroupDeadlineFor(30*time.Second, 45*time.Second))
	require.Equal(t, 870*time.Second, GroupDeadlineFor(30*time.Second, 120*time.Second))
	require.Equal(t, 330*time.Second, GroupDeadlineFor(0, 45*time.Second), "no TypeSafe, no check: 60 + 180 + 60 + 30")
	require.Equal(t, MinGroupDeadline, GroupDeadlineFor(0, 10*time.Second), "190 s is under the floor")
	require.Equal(t, MinGroupDeadline, GroupDeadlineFor(0, 0))
	require.Equal(t, 5*time.Minute, MinGroupDeadline)
}

func TestLLMLatencySettingsAreBounded(t *testing.T) {
	require.Equal(t, 45*time.Second, LLMCallTimeoutFor(0), "a row from before the setting")
	require.Equal(t, 45*time.Second, LLMCallTimeoutFor(9))
	require.Equal(t, 45*time.Second, LLMCallTimeoutFor(121))
	require.Equal(t, 10*time.Second, LLMCallTimeoutFor(models.MinLLMCallTimeoutSeconds))
	require.Equal(t, 120*time.Second, LLMCallTimeoutFor(models.MaxLLMCallTimeoutSeconds))

	require.Zero(t, HedgeAfterFor(0, 45*time.Second), "0 = off")
	require.Zero(t, HedgeAfterFor(2, 45*time.Second), "below the 3 s minimum")
	require.Zero(t, HedgeAfterFor(45, 45*time.Second), "not before the call timeout")
	require.Equal(t, 10*time.Second, HedgeAfterFor(10, 45*time.Second))
}

func TestLLMRetryOptionsCapRetryAfter(t *testing.T) {
	o := llmRetryOptions()
	require.Equal(t, TransportAttempts, o.MaxAttempts)
	require.Equal(t, 30*time.Second, o.MaxRetryAfter)
	require.Equal(t, LLMMaxRetryAfter, o.MaxRetryAfter)
}

func TestHedgeCostEvents_OnePerHedgeThatHadAWinner(t *testing.T) {
	price := 2.0
	deps := JobDeps{NarrativeModel: "m", Pricing: Pricing{LLMProviderID: "p1", LLMPromptPerMTok: &price, LLMCompletionPerMTok: &price}}
	jobID := "j"
	evs := HedgeCostEvents([]int{1000, 0, 500}, deps, CostRefs{RunID: "r", JobID: &jobID})
	require.Len(t, evs, 2, "a winner that reported no usage has nothing to price")
	ev := evs[0]
	require.Equal(t, models.AnalysisCostKindHedge, ev.Kind)
	require.Equal(t, models.AnalysisCostEngineLLM, ev.Engine)
	require.Equal(t, "m", ev.Model)
	require.Equal(t, 1000, ev.PromptTokens)
	require.Zero(t, ev.CompletionTokens, "the cancelled request's reply is not known")
	require.InDelta(t, 1000*2.0/1e6, *ev.EstimatedCost, 1e-12)
	require.Equal(t, "p1", *ev.ProviderID)
	require.Equal(t, "j", *ev.JobID)
	require.Nil(t, ev.AnalysisID)
	require.Equal(t, 500, evs[1].PromptTokens)
	require.Nil(t, HedgeCostEvents(nil, deps, CostRefs{RunID: "r"}))
}
