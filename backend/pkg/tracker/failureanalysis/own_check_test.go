package failureanalysis

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

func ownContext() AnalyzeContext {
	return AnalyzeContext{
		Result: &models.RunResult{ID: "rr-clone", TestNameSnapshot: "checkout clone", FailureType: "assertion",
			ErrorMessage: "POST /api/cart returned 503"},
		GroupMembers: []string{"a member line that must never be sent"},
	}
}

func injectionAnswer(p float64) func(typesafe.Request) (*typesafe.Response, error) {
	return func(req typesafe.Request) (*typesafe.Response, error) {
		return &typesafe.Response{Model: "jev-1.13.0", Usage: typesafe.Usage{InputTokens: 500},
			Answers: map[string]typesafe.Answer{questionInjection: {Type: "noul", Noul: p}}}, nil
	}
}

func TestCheckOwnEvidence_AsksOnlyTheInjectionQuestionAboutTheOwnEvidence(t *testing.T) {
	fc := &fakeClient{fn: injectionAnswer(0.03)}
	got, err := CheckOwnEvidence(context.Background(), TransferDeps{Client: fc, Model: "jev-1.13.0"}, ownContext())
	require.NoError(t, err)
	require.Len(t, fc.calls, 1)
	req := fc.calls[0]
	require.Equal(t, "jev-1.13.0", req.Model)
	require.Len(t, req.Questions, 1)
	require.Contains(t, req.Questions, questionInjection)
	body := fmt.Sprintf("%v", req.State)
	require.Contains(t, body, "POST /api/cart returned 503", "the clone's own failure")
	require.NotContains(t, body, "a member line", "no group members on this path")

	require.NotNil(t, got.Signals.Injection)
	require.InDelta(t, 0.03, *got.Signals.Injection, 1e-12)
	require.False(t, got.Signals.InjectionFlagged())
	require.Contains(t, got.Signals.CheckedBlocks, "failure.error", "the blocks the state carried were checked")
	_, hasGroup := fc.calls[0].State.(map[string]any)["group"]
	require.False(t, hasGroup, "no related failures in the state")
	require.NotEmpty(t, got.Signals.EvidenceHash)
	require.Equal(t, 500, got.InputTokens)
	require.Equal(t, "jev-1.13.0", got.Model)

	fc.fn = injectionAnswer(0.95)
	got, err = CheckOwnEvidence(context.Background(), TransferDeps{Client: fc}, ownContext())
	require.NoError(t, err)
	require.True(t, got.Signals.InjectionFlagged())
}

func TestCheckOwnEvidence_FailuresAreErrors(t *testing.T) {
	_, err := CheckOwnEvidence(context.Background(), TransferDeps{}, ownContext())
	require.ErrorIs(t, err, ErrOwnCheckUnavailable)

	fc := &fakeClient{fn: func(typesafe.Request) (*typesafe.Response, error) { return nil, errors.New("down") }}
	_, err = CheckOwnEvidence(context.Background(), TransferDeps{Client: fc}, ownContext())
	require.Error(t, err)

	fc.fn = func(typesafe.Request) (*typesafe.Response, error) {
		return &typesafe.Response{Model: "jev-1.13.0", Answers: map[string]typesafe.Answer{}, Usage: typesafe.Usage{InputTokens: 7}}, nil
	}
	got, err := CheckOwnEvidence(context.Background(), TransferDeps{Client: fc}, ownContext())
	require.Error(t, err, "no injection answer is no check")
	require.Equal(t, 7, got.InputTokens, "still billed")
}

func TestOwnCheckCostsAndEstimate(t *testing.T) {
	deps := JobDeps{Narrative: &stubProvider{}, Transfer: &TransferDeps{Model: "jev-1.13.0"},
		Pricing: Pricing{LLMPromptPerMTok: f64ptr(1), LLMCompletionPerMTok: f64ptr(4), TypeSafePerMTok: 0.042}}
	an := "clone-1"
	evs := OwnCheckCostEvents(OwnEvidenceCheck{InputTokens: 500}, deps, CostRefs{RunID: "r", AnalysisID: &an})
	require.Len(t, evs, 1)
	require.Equal(t, models.AnalysisCostKindExplain, evs[0].Kind)
	require.Equal(t, models.AnalysisCostEngineTypeSafe, evs[0].Engine)
	require.Equal(t, "jev-1.13.0", evs[0].Model)
	require.Equal(t, 500, evs[0].TypeSafeInputTokens)
	require.Equal(t, "clone-1", *evs[0].AnalysisID)
	require.Nil(t, OwnCheckCostEvents(OwnEvidenceCheck{}, deps, CostRefs{RunID: "r"}))

	llmStage := 4 * (float64(PromptCharCap/4)*1 + float64(ReplyTokenCap)*4) / 1e6
	tsCall := float64(TypeSafeStateCharCap/4) * 0.042 / 1e6
	require.InDelta(t, llmStage+tsCall, *EstimateOwnExplainUSD(deps), 1e-12)
	noTS := deps
	noTS.Transfer = nil
	require.InDelta(t, llmStage, *EstimateOwnExplainUSD(noTS), 1e-12)
	require.Nil(t, EstimateOwnExplainUSD(JobDeps{Transfer: deps.Transfer}), "no narrator, nothing to explain with")
}
