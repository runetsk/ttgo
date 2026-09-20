package failureanalysis

import "ttgo/pkg/tracker/models"

// AnalysisRowFrom maps an analyzer result onto a persistable row (representative, not clone).
// It lives here (not in the worker package) because it is a pure mapper shared by the
// background worker and the synchronous HTTP handler — neither should depend on the other.
func AnalysisRowFrom(res *AnalyzeResult, resultID string) *models.RunResultAnalysis {
	return &models.RunResultAnalysis{
		RunResultID: resultID, Verdict: res.Verdict, Confidence: res.Confidence,
		Summary: res.Summary, NextAction: res.NextAction, Rationale: res.Rationale,
		RawResponse: res.RawResponse, ModelName: res.ModelName,
		TokenUsagePrompt: res.TokenUsagePrompt, TokenUsageCompletion: res.TokenUsageCompletion,
		Engine: res.Engine, ConfidenceScore: res.ConfidenceScore, VerdictProbabilities: res.VerdictProbabilities,
		SuggestedDefectType: res.SuggestedDefectType, SuggestedDefectTypeConfidence: res.SuggestedDefectTypeConfidence,
		DefectTypeProbabilities: res.DefectTypeProbabilities, NarrativeStatus: res.NarrativeStatus,
		PolicyVersion: res.PolicyVersion, TypeSafeInputTokens: res.TypeSafeInputTokens,
	}
}
