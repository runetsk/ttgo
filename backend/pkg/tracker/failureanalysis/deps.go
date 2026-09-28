package failureanalysis

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/typesafe"
)

// ErrAIDisabled: the global AI master switch is off, so no failure data may be sent anywhere.
var ErrAIDisabled = errors.New("AI features are switched off")

// TriggerExplain is the resolver trigger for an explicit "Explain" request on a stored
// decision: a manual action that needs the LLM even when automatic explanations are off.
const TriggerExplain = "explain"

// AnalyzeDeps is what Analyze needs. Narrative nil = no generative provider; Decider nil = TypeSafe
// verdicts not permitted for this job. NarrativeSkipped = the operator switched explanations off
// in TypeSafe settings: a TypeSafe decision is stored without calling Narrative, which stays set
// so the generative fallback still works if TypeSafe fails. EscalateBelow (0..1, 0 = never): a
// TypeSafe verdict whose confidence is below it is handed to Narrative to decide instead.
// NoLLMFallback: a TypeSafe failure is recorded as a failed attempt instead of being handed to
// the LLM. LLMUnavailableReason says why Narrative is nil when an explanation was wanted.
// DeciderModel names the TypeSafe model for failed-attempt records.
type AnalyzeDeps struct {
	Narrative            llm.Provider
	NarrativeModel       string
	Decider              Decider
	DeciderModel         string
	NarrativeSkipped     bool
	EscalateBelow        float64
	NoLLMFallback        bool
	LLMUnavailableReason string
}

// SemanticDeps is what the semantic grouping pass needs.
type SemanticDeps struct {
	Client typesafe.Client
	Model  string
	Redact bool
}

// JobDeps is everything one analysis job needs, resolved per job from live settings.
// Semantic nil = semantic grouping not permitted for this trigger.
// Pricing carries the prices in force at resolve time for the cost ledger and budget estimates.
type JobDeps struct {
	Narrative            llm.Provider
	NarrativeModel       string
	Decider              Decider
	DeciderModel         string
	NarrativeSkipped     bool
	EscalateBelow        float64
	NoLLMFallback        bool
	LLMUnavailableReason string
	Semantic             *SemanticDeps
	// FewShotExamples is how many past triage decisions accompany each failure (0 = off), from the
	// failure-analysis settings at resolve time. Callers pass it to BuildContext.
	FewShotExamples int
	// Pricing is what this job's calls cost, captured when the dependencies were resolved.
	Pricing Pricing
	// TypeSafeTimeout and LLMCallTimeout are the per-call timeouts this job runs with (0 = that
	// engine is not attached); GroupDeadlineFor derives the group deadline from them.
	TypeSafeTimeout time.Duration
	LLMCallTimeout  time.Duration
	// HedgingOn: Narrative sends a hedged second request when the first is slow, so the
	// worst-case estimate doubles its LLM part.
	HedgingOn bool
}

// Analyze returns the analyzer-facing subset.
func (j JobDeps) Analyze() AnalyzeDeps {
	return AnalyzeDeps{Narrative: j.Narrative, NarrativeModel: j.NarrativeModel, Decider: j.Decider,
		DeciderModel: j.DeciderModel, NarrativeSkipped: j.NarrativeSkipped, EscalateBelow: j.EscalateBelow,
		NoLLMFallback: j.NoLLMFallback, LLMUnavailableReason: j.LLMUnavailableReason}
}

// Pipeline describes the route a job runs with, for the job record and for grading one job
// as a whole. It is derived from the resolved dependencies, never from settings directly.
type Pipeline struct {
	Decider          string `json:"decider,omitempty"`  // TypeSafe model, "" when the LLM decides
	Policy           string `json:"policy,omitempty"`   // TypeSafe question-set version
	Narrator         string `json:"narrator,omitempty"` // LLM model, "" when none is attached
	Explanations     bool   `json:"explanations"`
	TakeoverBelowPct int    `json:"takeover_below_pct,omitempty"`
	LLMFallback      bool   `json:"llm_fallback"`
	Semantic         bool   `json:"semantic_grouping"`
	ReplyTokenCap    int    `json:"reply_token_cap"`
}

// Pipeline snapshots the route these dependencies produce.
func (j JobDeps) Pipeline() Pipeline {
	p := Pipeline{Narrator: j.NarrativeModel, Semantic: j.Semantic != nil, ReplyTokenCap: ReplyTokenCap}
	if j.Narrative == nil {
		p.Narrator = ""
	}
	if j.Decider != nil {
		p.Decider, p.Policy = j.DeciderModel, PolicyVersion
		p.Explanations = !j.NarrativeSkipped && j.Narrative != nil
		p.TakeoverBelowPct = int(j.EscalateBelow*100 + 0.5)
		p.LLMFallback = !j.NoLLMFallback && j.Narrative != nil
	} else {
		p.Explanations = j.Narrative != nil
	}
	return p
}

// Label is a short human name for the route, such as "TypeSafe, minimax below 90%".
func (p Pipeline) Label() string {
	if p.Decider == "" {
		if p.Narrator == "" {
			return "no engine"
		}
		return p.Narrator
	}
	parts := []string{"TypeSafe " + p.Decider}
	switch {
	case p.Narrator == "":
		parts = append(parts, "no LLM")
	case p.Explanations:
		parts = append(parts, "explained by "+p.Narrator)
	default:
		parts = append(parts, "decisions only")
	}
	if p.TakeoverBelowPct > 0 && p.Narrator != "" {
		parts = append(parts, fmt.Sprintf("%s below %d%%", p.Narrator, p.TakeoverBelowPct))
	}
	return strings.Join(parts, ", ")
}

// CanAnalyze reports whether a job can produce analyses at all: it needs a narrator to
// decide and explain, or a TypeSafe decider (which can store a decision on its own).
func (j JobDeps) CanAnalyze() bool { return j.Narrative != nil || j.Decider != nil }

// DepsResolver is implemented in internal/api (it needs the store and settings) and injected
// into the worker and the manual-analysis handler. Declared here so the worker package does
// not import internal/api (which imports the worker). trigger is models.RunAnalysisJobTrigger*.
type DepsResolver func(trigger string) (JobDeps, error)
