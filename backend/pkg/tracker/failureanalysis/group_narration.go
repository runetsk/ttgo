package failureanalysis

import (
	"strings"
	"ttgo/pkg/tracker/models"
)

// MaxGroupMemberErrors bounds the related-failure lines one group narration receives (spec §A4):
// the same limit the prompt block applies.
const MaxGroupMemberErrors = GroupMembersMax

// GroupMemberErrors picks the raw error messages of a group's other members for
// AnalyzeContext.GroupMembers: distinct by failure signature, never the representative's own
// signature (so a signature group adds nothing), blanks skipped, at most MaxGroupMemberErrors.
// Redaction and the per-line cap happen in BuildEvidence.
func GroupMemberErrors(rep *models.RunResult, members []*models.RunResult) []string {
	if rep == nil {
		return nil
	}
	seen := map[string]bool{Signature(rep.FailureType, rep.ErrorMessage): true}
	var out []string
	for _, m := range members {
		if m == nil || m.ID == rep.ID || strings.TrimSpace(m.ErrorMessage) == "" {
			continue
		}
		sig := Signature(m.FailureType, m.ErrorMessage)
		if seen[sig] {
			continue
		}
		seen[sig] = true
		out = append(out, m.ErrorMessage)
		if len(out) == MaxGroupMemberErrors {
			break
		}
	}
	return out
}

// DecidedFromRow rebuilds a stored decision as Decide returns one whose explanation is still to
// be written, so Narrate can explain it later (on-demand Explain). Only the decision is carried:
// no usage, no narrative text.
func DecidedFromRow(a *models.RunResultAnalysis) *AnalyzeResult {
	return &AnalyzeResult{
		Verdict: a.Verdict, Confidence: a.Confidence, ModelName: a.ModelName, Engine: a.Engine,
		ConfidenceScore: a.ConfidenceScore, VerdictProbabilities: a.VerdictProbabilities,
		SuggestedDefectType: a.SuggestedDefectType, SuggestedDefectTypeConfidence: a.SuggestedDefectTypeConfidence,
		SuggestionSource: a.SuggestionSource, DefectTypeProbabilities: a.DefectTypeProbabilities,
		PolicyVersion: a.PolicyVersion, HistoryAvailable: a.HistoryAvailable, Signals: a.Signals,
		DecisionStatus: a.DecisionStatus, NarrativeStatus: models.NarrativeStatusPending,
	}
}

// DecidedForExplain is DecidedFromRow for an on-demand Explain of a group, or of a clone through
// its representative (spec R8). in is the context Explain rebuilt from the store; rows and
// members may have changed since the decision, so a guarded decision keeps only the checked
// blocks whose text is unchanged (VerifiedAgainst over the TypeSafe-budget evidence), and
// Narrate sends only those. An unguarded row (before policy v7) is returned as DecidedFromRow
// does; Narrate then drops its related failures. The stored row is not changed.
func DecidedForExplain(a *models.RunResultAnalysis, in AnalyzeContext) *AnalyzeResult {
	d := DecidedFromRow(a)
	if s := ParseSignals(a.Signals); s.Guarded() {
		d.Signals = SignalsJSON(s.VerifiedAgainst(BuildEvidenceWithBudget(in, TypeSafeBudget())))
	}
	return d
}
