package failureanalysis

import (
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/typesafe"
)

// AnalyzeDeps is what Analyze needs. Narrative nil = no generative provider; Decider nil = TypeSafe
// verdicts not permitted for this job.
type AnalyzeDeps struct {
	Narrative      llm.Provider
	NarrativeModel string
	Decider        Decider
}

// SemanticDeps is what the semantic grouping pass needs.
type SemanticDeps struct {
	Client typesafe.Client
	Model  string
	Redact bool
}

// JobDeps is everything one analysis job needs, resolved per job from live settings.
// Semantic nil = semantic grouping not permitted for this trigger.
type JobDeps struct {
	Narrative      llm.Provider
	NarrativeModel string
	Decider        Decider
	Semantic       *SemanticDeps
}

// Analyze returns the analyzer-facing subset.
func (j JobDeps) Analyze() AnalyzeDeps {
	return AnalyzeDeps{Narrative: j.Narrative, NarrativeModel: j.NarrativeModel, Decider: j.Decider}
}

// DepsResolver is implemented in internal/api (it needs the store and settings) and injected
// into the worker and the manual-analysis handler. Declared here so the worker package does
// not import internal/api (which imports the worker). trigger is models.RunAnalysisJobTrigger*.
type DepsResolver func(trigger string) (JobDeps, error)
