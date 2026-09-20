package failureanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

func TestDecider_MapsAnswersAndSendsBothQuestions(t *testing.T) {
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		return decisionResponse(models.VerdictFlakyTest, 0.93, 0.95, "automation_bug", 0.88, 0.9), nil
	}}
	d, err := NewTypeSafeDecider(fc, "jev-1.13.0").Decide(context.Background(), BuildEvidence(baseContext()))
	require.NoError(t, err)
	require.Equal(t, models.VerdictFlakyTest, d.Verdict)
	require.InDelta(t, 0.93, d.VerdictConfidence, 1e-9)
	require.Equal(t, "automation_bug", d.SuggestedDefectType)
	require.InDelta(t, 0.88, d.DefectTypeConfidence, 1e-9)
	require.Equal(t, "jev-1.13.0", d.Model)
	require.Equal(t, 777, d.InputTokens)
	require.Equal(t, PolicyVersion, d.PolicyVersion)
	require.Len(t, d.VerdictProbabilities, 6)
	require.Len(t, d.DefectTypeProbabilities, 4)

	require.Len(t, fc.calls, 1)
	req := fc.calls[0]
	require.Equal(t, "jev-1.13.0", req.Model)
	require.Contains(t, req.Questions, "verdict")
	require.Contains(t, req.Questions, "defect_type")
	b, _ := json.Marshal(req.State)
	require.Contains(t, string(b), "expected 401, got 500", "state carries the failure")
}

func TestDecider_AbstainsOnInsufficientEvidence(t *testing.T) {
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		return decisionResponse(models.VerdictUnknown, 0.6, 0.7, DefectTypeInsufficient, 0.95, 0.96), nil
	}}
	d, err := NewTypeSafeDecider(fc, "m").Decide(context.Background(), BuildEvidence(baseContext()))
	require.NoError(t, err)
	require.Equal(t, "", d.SuggestedDefectType)
	require.InDelta(t, 0.95, d.DefectTypeConfidence, 1e-9, "confidence is kept even when abstaining")
}

func TestDecider_AbstainsBelowThreshold(t *testing.T) {
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		return decisionResponse(models.VerdictProductBug, 0.7, 0.8, "product_bug", DefectTypeSuggestMin-0.01, 0.5), nil
	}}
	d, err := NewTypeSafeDecider(fc, "m").Decide(context.Background(), BuildEvidence(baseContext()))
	require.NoError(t, err)
	require.Equal(t, "", d.SuggestedDefectType)
	require.Equal(t, models.VerdictProductBug, d.Verdict, "verdict is independent of the abstention")
}

func TestDecider_UnknownVerdictWithSuggestionIsValid(t *testing.T) {
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		return decisionResponse(models.VerdictUnknown, 0.5, 0.6, "automation_bug", 0.9, 0.9), nil
	}}
	d, err := NewTypeSafeDecider(fc, "m").Decide(context.Background(), BuildEvidence(baseContext()))
	require.NoError(t, err)
	require.Equal(t, models.VerdictUnknown, d.Verdict)
	require.Equal(t, "automation_bug", d.SuggestedDefectType)
}

func TestDecider_PropagatesClientErrors(t *testing.T) {
	want := &typesafe.Error{Category: typesafe.CategoryRateLimit, Status: 429}
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) { return nil, want }}
	_, err := NewTypeSafeDecider(fc, "m").Decide(context.Background(), BuildEvidence(baseContext()))
	require.True(t, errors.Is(err, want))
}
