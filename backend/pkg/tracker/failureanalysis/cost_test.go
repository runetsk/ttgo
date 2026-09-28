package failureanalysis

import (
	"errors"
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func TestCostEvents_TypeSafeAndLLMForOneRepresentative(t *testing.T) {
	deps := JobDeps{NarrativeModel: "gpt-x", DeciderModel: "jev-1.13.0",
		Pricing: Pricing{LLMProviderID: "prov-1", LLMPromptPerMTok: f64ptr(2), LLMCompletionPerMTok: f64ptr(10), TypeSafePerMTok: 0.042}}
	jobID, analysisID := "job-1", "an-1"
	res := &AnalyzeResult{Engine: models.AnalysisEngineTypeSafe, ModelName: "jev-1.13.0-build7",
		TypeSafeInputTokens: 10000, TokenUsagePrompt: 1000, TokenUsageCompletion: 200}

	evs := CostEvents(models.AnalysisCostKindAnalysis, res, deps, CostRefs{RunID: "run-1", JobID: &jobID, AnalysisID: &analysisID})
	require.Len(t, evs, 2)
	ts, l := evs[0], evs[1]
	require.Equal(t, models.AnalysisCostEngineTypeSafe, ts.Engine)
	require.Equal(t, "jev-1.13.0-build7", ts.Model, "the versioned model TypeSafe answered with")
	require.Equal(t, 10000, ts.TypeSafeInputTokens)
	require.InDelta(t, 0.00042, *ts.EstimatedCost, 1e-12)
	require.Nil(t, ts.ProviderID)
	require.Equal(t, models.AnalysisCostEngineLLM, l.Engine)
	require.Equal(t, "gpt-x", l.Model, "the narrator's model: the row's model is TypeSafe's")
	require.Equal(t, "prov-1", *l.ProviderID)
	require.Equal(t, 1000, l.PromptTokens)
	require.Equal(t, 200, l.CompletionTokens)
	require.InDelta(t, 0.004, *l.EstimatedCost, 1e-12) // 1000×$2/M + 200×$10/M
	for _, e := range evs {
		require.Equal(t, models.AnalysisCostKindAnalysis, e.Kind)
		require.Equal(t, "run-1", e.RunID)
		require.Equal(t, "job-1", *e.JobID)
		require.Equal(t, "an-1", *e.AnalysisID)
	}
}

func TestCostEvents_OnlyWhatWasBilled(t *testing.T) {
	deps := JobDeps{NarrativeModel: "gpt-x", DeciderModel: "jev"} // the LLM has no prices
	gen := &AnalyzeResult{Engine: models.AnalysisEngineGenerative, ModelName: "gpt-x-0613", TokenUsagePrompt: 10, TokenUsageCompletion: 5}
	evs := CostEvents(models.AnalysisCostKindAnalysis, gen, deps, CostRefs{RunID: "r"})
	require.Len(t, evs, 1)
	require.Equal(t, "gpt-x-0613", evs[0].Model, "a generative row names the model that answered")
	require.Nil(t, evs[0].EstimatedCost, "an unpriced provider records tokens with no cost")
	require.Nil(t, evs[0].ProviderID)

	// Takeover: TypeSafe decided first, then the LLM decided. Both billed.
	takeover := &AnalyzeResult{Engine: models.AnalysisEngineGenerative, ModelName: "gpt-x", TypeSafeInputTokens: 800, TokenUsagePrompt: 10}
	evs = CostEvents(models.AnalysisCostKindAnalysis, takeover, deps, CostRefs{RunID: "r"})
	require.Len(t, evs, 2)
	require.Equal(t, "jev", evs[0].Model)
	require.InDelta(t, 0, *evs[0].EstimatedCost, 1e-12, "TypeSafe priced at 0 is free, not unknown")

	require.Empty(t, CostEvents(models.AnalysisCostKindAnalysis, &AnalyzeResult{Engine: models.AnalysisEngineTypeSafe}, deps, CostRefs{RunID: "r"}),
		"an attempt that used no tokens bills nothing")
	require.Empty(t, CostEvents(models.AnalysisCostKindAnalysis, nil, deps, CostRefs{RunID: "r"}))
}

func TestSemanticCostEvents(t *testing.T) {
	deps := JobDeps{Semantic: &SemanticDeps{Model: "jev-1.13.0"}, Pricing: Pricing{TypeSafePerMTok: 0.5}}
	jobID := "job-1"
	evs := SemanticCostEvents(SemanticReport{InputTokens: 2000}, deps, CostRefs{RunID: "run-1", JobID: &jobID})
	require.Len(t, evs, 1)
	require.Equal(t, models.AnalysisCostKindSemantic, evs[0].Kind)
	require.Equal(t, models.AnalysisCostEngineTypeSafe, evs[0].Engine)
	require.Equal(t, "jev-1.13.0", evs[0].Model)
	require.Equal(t, 2000, evs[0].TypeSafeInputTokens)
	require.InDelta(t, 0.001, *evs[0].EstimatedCost, 1e-12)
	require.Nil(t, evs[0].AnalysisID)

	evs = SemanticCostEvents(SemanticReport{InputTokens: 10, Model: "jev-1.13.0-build7"}, deps, CostRefs{RunID: "run-1"})
	require.Equal(t, "jev-1.13.0-build7", evs[0].Model, "the model TypeSafe reported wins")
	require.Empty(t, SemanticCostEvents(SemanticReport{}, deps, CostRefs{RunID: "run-1"}), "no pairs asked, nothing billed")
}

func TestRefsFor(t *testing.T) {
	jobID := "job-1"
	require.Equal(t, CostRefs{RunID: "r", JobID: &jobID}, RefsFor("r", &jobID, nil), "a row that failed to store still bills its call")
	row := &models.RunResultAnalysis{ID: "an-1"}
	refs := RefsFor("r", nil, row)
	require.Equal(t, "an-1", *refs.AnalysisID)
	require.Nil(t, refs.JobID)
}

func TestEstimateCallUSD_IsWorstCasePerRoute(t *testing.T) {
	priced := Pricing{LLMPromptPerMTok: f64ptr(1), LLMCompletionPerMTok: f64ptr(4), TypeSafePerMTok: 0.042}
	// One LLM stage at worst: the first call and the JSON repair, each with its transient retry.
	llmStage := 4 * (float64(PromptCharCap/4)*1 + float64(ReplyTokenCap)*4) / 1e6
	tsCall := float64(TypeSafeStateCharCap/4) * 0.042 / 1e6
	decider := NewUnavailableDecider(errors.New("unused"))

	llmOnly := JobDeps{Narrative: &stubProvider{}, Pricing: priced}
	require.InDelta(t, llmStage, *EstimateCallUSD(llmOnly), 1e-12)

	hedged := llmOnly
	hedged.HedgingOn = true
	require.InDelta(t, 2*llmStage, *EstimateCallUSD(hedged), 1e-12, "hedging may send every attempt twice")

	explained := JobDeps{Narrative: &stubProvider{}, Decider: decider, Pricing: priced}
	require.InDelta(t, tsCall+llmStage, *EstimateCallUSD(explained), 1e-12)

	decisionsOnly := JobDeps{Narrative: &stubProvider{}, Decider: decider, NarrativeSkipped: true, Pricing: priced}
	require.InDelta(t, tsCall, *EstimateCallUSD(decisionsOnly), 1e-12, "the LLM is only the fallback here")

	takeover := decisionsOnly
	takeover.EscalateBelow = 0.9
	require.InDelta(t, tsCall+llmStage, *EstimateCallUSD(takeover), 1e-12)

	unpriced := JobDeps{Narrative: &stubProvider{}}
	require.Nil(t, EstimateCallUSD(unpriced), "cost unknown: budget checks are skipped")

	require.InDelta(t, llmStage, *EstimateExplainUSD(explained), 1e-12)
	explainedHedged := explained
	explainedHedged.HedgingOn = true
	require.InDelta(t, 2*llmStage, *EstimateExplainUSD(explainedHedged), 1e-12)
	require.Nil(t, EstimateExplainUSD(JobDeps{Decider: decider, Pricing: priced}), "no narrator, nothing to explain with")

	require.InDelta(t, 3*llmStage, *EstimateJobUSD(llmOnly, 3), 1e-12)
	require.Nil(t, EstimateJobUSD(unpriced, 3))
}

func TestPlannedGroups(t *testing.T) {
	mk := func(id, msg string) *models.RunResult {
		return &models.RunResult{ID: id, FailureType: "assertion", ErrorMessage: msg}
	}
	failures := []*models.RunResult{mk("a", "boom"), mk("b", "boom"), mk("c", "other")}
	require.Equal(t, 2, PlannedGroups(failures, true, 50))
	require.Equal(t, 3, PlannedGroups(failures, false, 50))
	require.Equal(t, 1, PlannedGroups(failures, true, 1), "capped at max analyses per run")
	require.Equal(t, 0, PlannedGroups(nil, true, 50))
}
