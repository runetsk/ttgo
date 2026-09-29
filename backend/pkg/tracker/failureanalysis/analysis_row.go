package failureanalysis

import "ttgo/pkg/tracker/models"

// AnalysisRowFrom maps an analyzer result onto a persistable row (representative, not clone).
// It lives here (not in the worker package) because it is a pure mapper shared by the
// background worker and the synchronous HTTP handler — neither should depend on the other.
func AnalysisRowFrom(res *AnalyzeResult, resultID string) *models.RunResultAnalysis {
	status := res.DecisionStatus
	if status == "" {
		status = models.DecisionStatusOK
	}
	return &models.RunResultAnalysis{
		RunResultID: resultID, Verdict: res.Verdict, Confidence: res.Confidence,
		Summary: res.Summary, NextAction: res.NextAction, Rationale: res.Rationale,
		RawResponse: res.RawResponse, ModelName: res.ModelName,
		TokenUsagePrompt: res.TokenUsagePrompt, TokenUsageCompletion: res.TokenUsageCompletion,
		Engine: res.Engine, ConfidenceScore: res.ConfidenceScore, VerdictProbabilities: res.VerdictProbabilities,
		SuggestedDefectType: res.SuggestedDefectType, SuggestedDefectTypeConfidence: res.SuggestedDefectTypeConfidence,
		DefectTypeProbabilities: res.DefectTypeProbabilities, NarrativeStatus: res.NarrativeStatus,
		SuggestionSource: res.SuggestionSource,
		PolicyVersion:    res.PolicyVersion, TypeSafeInputTokens: res.TypeSafeInputTokens, HistoryAvailable: res.HistoryAvailable,
		Signals:        res.Signals,
		DecisionStatus: status, ErrorCategory: res.ErrorCategory,
		TakeoverFromVerdict: res.TakeoverFromVerdict, TakeoverFromConfidence: res.TakeoverFromConfidence,
		TakeoverFromDefectType: res.TakeoverFromDefectType,
		DecisionMs:             res.DecisionMs, LLMMs: res.LLMMs, LLMCalls: res.LLMCalls, FinishReason: res.FinishReason,
	}
}
