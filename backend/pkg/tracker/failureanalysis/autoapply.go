package failureanalysis

import "ttgo/pkg/tracker/models"

// Accuracy gate for auto-apply (spec §3.4, R4): TypeSafe's direct defect-type suggestions at or
// above the threshold, graded against the people who triaged them over the last
// AutoApplyGateWindowDays under the current question policies, must number at least
// AutoApplyGateMinGraded and agree at least AutoApplyGateMinAccuracy of the time.
const (
	AutoApplyGateMinGraded   = 50
	AutoApplyGateMinAccuracy = 0.95
	AutoApplyGateWindowDays  = 90
)

// GateStatus is the gate's figures at one threshold. Policies are the question-policy versions
// the figures cover; a policy bump closes the gate until enough new decisions are graded.
type GateStatus struct {
	Graded        int      `json:"graded"`
	Agreed        int      `json:"agreed"`
	Accuracy      float64  `json:"accuracy"`
	Open          bool     `json:"open"`
	Policies      []string `json:"policies"`
	MinConfidence float64  `json:"min_confidence"`
	WindowDays    int      `json:"window_days"`
	MinGraded     int      `json:"min_graded"`
	MinAccuracy   float64  `json:"min_accuracy"`
}

// NewGateStatus derives accuracy and the open flag from the counts.
func NewGateStatus(graded, agreed int, minConfidence float64, policies []string) GateStatus {
	g := GateStatus{Graded: graded, Agreed: agreed, MinConfidence: minConfidence,
		Policies: append([]string(nil), policies...), WindowDays: AutoApplyGateWindowDays,
		MinGraded: AutoApplyGateMinGraded, MinAccuracy: AutoApplyGateMinAccuracy}
	if graded > 0 {
		g.Accuracy = float64(agreed) / float64(graded)
	}
	g.Open = graded >= AutoApplyGateMinGraded && g.Accuracy >= AutoApplyGateMinAccuracy
	return g
}

// AutoApplyDeps is present on a job that may label results by itself. MinConfidence is the
// defect-type confidence (0..1) a decision needs.
type AutoApplyDeps struct {
	MinConfidence float64
}

// autoApplyTypes are the conclusive defect types auto-apply may write.
var autoApplyTypes = map[string]bool{"product_bug": true, "automation_bug": true, "system_issue": true}

// AutoApplyValue says whether a decision may label its results by itself, and with which defect
// type (spec §3.3). Only a direct TypeSafe answer qualifies: engine typesafe, a decision (not a
// failed attempt), no LLM takeover, a suggestion answered by the defect-type question (not
// derived from the verdict), no suspected prompt injection, one of the three conclusive types,
// and its defect-type confidence at or above minConfidence. A threshold below the settings floor
// is treated as a misconfiguration and qualifies nothing.
func AutoApplyValue(res *AnalyzeResult, minConfidence float64) (string, bool) {
	if res == nil || minConfidence < float64(models.MinAutoApplyMinConfidence)/100 {
		return "", false
	}
	if res.Engine != models.AnalysisEngineTypeSafe || res.DecisionStatus != models.DecisionStatusOK {
		return "", false
	}
	if res.TakeoverFromVerdict != "" || res.SuggestionSource != "" {
		return "", false
	}
	if ParseSignals(res.Signals).InjectionFlagged() {
		return "", false
	}
	if !autoApplyTypes[res.SuggestedDefectType] {
		return "", false
	}
	if res.SuggestedDefectTypeConfidence == nil || *res.SuggestedDefectTypeConfidence < minConfidence {
		return "", false
	}
	return res.SuggestedDefectType, true
}

// AutoApplyTarget is AutoApplyValue at this job's threshold; false when auto-apply is off or
// paused for the job.
func (j JobDeps) AutoApplyTarget(res *AnalyzeResult) (string, bool) {
	if j.AutoApply == nil {
		return "", false
	}
	return AutoApplyValue(res, j.AutoApply.MinConfidence)
}

// AutoApplyTargetRow is AutoApplyTarget for a stored decision: group Explain maintains the
// semantic clones' labels from the representative row it re-read (spec R10).
func (j JobDeps) AutoApplyTargetRow(a *models.RunResultAnalysis) (string, bool) {
	if a == nil {
		return "", false
	}
	return j.AutoApplyTarget(&AnalyzeResult{Engine: a.Engine, DecisionStatus: a.DecisionStatus,
		TakeoverFromVerdict: a.TakeoverFromVerdict, SuggestionSource: a.SuggestionSource, Signals: a.Signals,
		SuggestedDefectType: a.SuggestedDefectType, SuggestedDefectTypeConfidence: a.SuggestedDefectTypeConfidence})
}
