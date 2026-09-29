package failureanalysis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"
)

// Decision is what TypeSafe decided for one failure. SuggestedDefectType is "" when it abstained.
type Decision struct {
	Verdict                 string
	VerdictConfidence       float64
	VerdictProbabilities    map[string]float64
	SuggestedDefectType     string
	DefectTypeConfidence    float64
	DefectTypeProbabilities map[string]float64
	SuggestionSource        string // models.SuggestionSourceVerdict when derived from the verdict
	Model                   string // versioned id from the response
	InputTokens             int
	PolicyVersion           string
	// Signals are the companion answers of the same request (spec Wave 3 §1): the injection
	// guard, flaky_history, recurring, outside_app and known_defect. Zero when none came back.
	Signals Signals
}

// Decider produces a Decision from evidence. The analyzer treats nil as "not permitted".
type Decider interface {
	Decide(ctx context.Context, ev Evidence) (*Decision, error)
}

// unavailableDecider stands in for TypeSafe when it is selected but cannot be used as
// configured. Every decision fails with the configured error, so the analyzer applies the
// LLM-fallback switch exactly as it does for an outage: with the fallback on the LLM decides
// and the analysis says TypeSafe was unavailable; with it off the attempt is recorded as
// failed and no failure data reaches the LLM.
type unavailableDecider struct{ err error }

// NewUnavailableDecider returns a Decider whose every call fails with err.
func NewUnavailableDecider(err error) Decider { return unavailableDecider{err: err} }

func (d unavailableDecider) Decide(context.Context, Evidence) (*Decision, error) { return nil, d.err }

type typesafeDecider struct {
	client typesafe.Client
	model  string
}

// NewTypeSafeDecider asks the verdict, defect_type and companion questions in ONE request.
func NewTypeSafeDecider(c typesafe.Client, model string) Decider {
	return &typesafeDecider{client: c, model: model}
}

func (d *typesafeDecider) Decide(ctx context.Context, ev Evidence) (*Decision, error) {
	e, err := d.evaluate(ctx, ev)
	var te *typesafe.Error
	if errors.As(err, &te) && te.Oversized {
		// The budget is sized in characters against a token limit; when the
		// vendor still rejects the state, shrink once and let the ladder drop
		// the log rather than losing the decision to the generative fallback.
		bound := ev.StateCap
		if bound <= 0 {
			bound = StateCharCap
		}
		ev.StateCap = bound / 2
		slog.Warn("failure-analysis: TypeSafe rejected the state as oversized; retrying at half the bound", "bound", ev.StateCap)
		e, err = d.evaluate(ctx, ev)
	}
	if err != nil {
		return nil, err
	}
	resp := e.resp
	v := resp.Answers["verdict"]
	if !models.ValidVerdicts[v.Choice] { // impossible after client validation, checked anyway
		return nil, &typesafe.Error{Category: typesafe.CategoryParse, Message: fmt.Sprintf("verdict %q is not a known verdict", v.Choice)}
	}
	dt := resp.Answers["defect_type"]
	suggested, source, suggestedConf := suggestion(v, dt)
	return &Decision{
		Verdict: v.Choice, VerdictConfidence: v.Confidence, VerdictProbabilities: v.Probabilities,
		SuggestedDefectType: suggested, DefectTypeConfidence: suggestedConf, DefectTypeProbabilities: dt.Probabilities,
		SuggestionSource: source,
		Model:            resp.Model, InputTokens: resp.Usage.InputTokens, PolicyVersion: PolicyVersionFor(e.meta.ExamplesSent),
		Signals: signalsFrom(e),
	}, nil
}

// suggestion turns the two answers into the stored defect-type suggestion (policies
// fa-verdict-v5 to v8). The verdict wins where it is sure:
//
//   - A verdict at VerdictDecidesSuggestionMin or above, other than unknown, decides the
//     suggestion through the same mapping the generative path uses, whether the defect-type
//     question abstained or named another source. It carries the verdict's confidence and is
//     marked SuggestionSourceVerdict so calibration grades it separately. When the question
//     agrees, its own answer and confidence are kept.
//   - Under an unknown verdict, only automation_bug stands: a fault in the test code (a wrong
//     locator or assertion) is the one cause the verdict options cannot name, while a product
//     or system source the verdict could not pick is not claimed.
//   - Otherwise the question's answer stands when it names a source at DefectTypeSuggestMin
//     or above.
//
// The defect-type confidence is returned even when nothing is suggested, as before.
func suggestion(v, dt typesafe.Answer) (defectType, source string, confidence float64) {
	answered := dt.Choice
	if answered == DefectTypeInsufficient || dt.Confidence < DefectTypeSuggestMin || !models.ValidDefectTypes[answered] {
		answered = ""
	}
	if v.Choice != models.VerdictUnknown && v.Confidence >= VerdictDecidesSuggestionMin {
		if mapped := models.SuggestedDefectType(v.Choice); mapped != "" {
			if answered == mapped {
				return answered, "", dt.Confidence
			}
			return mapped, models.SuggestionSourceVerdict, v.Confidence
		}
	}
	if v.Choice == models.VerdictUnknown && answered != "automation_bug" {
		return "", "", dt.Confidence
	}
	return answered, "", dt.Confidence
}

// evaluation is one verdict request: the response, the evidence it was built from and what the
// state carried after its drop ladder (the policy is stamped from meta.ExamplesSent; the
// conditional companions, the known_defect mapping and the checked blocks use sent), and the
// questions that were asked.
type evaluation struct {
	resp      *typesafe.Response
	meta      PromptMeta
	built     Evidence
	sent      Evidence
	questions map[string]typesafe.Question
}

// evaluate renders the state and asks the verdict, defect_type and companion questions in one
// request. Conditional companions are chosen from what the state carries, not from what was built.
func (d *typesafeDecider) evaluate(ctx context.Context, ev Evidence) (evaluation, error) {
	state, meta, sent := renderState(ev)
	if meta.TruncationPrefix != "" {
		slog.Debug("failure-analysis: TypeSafe state trimmed", "prefix", meta.TruncationPrefix)
	}
	questions := decisionQuestions(sent)
	resp, err := d.client.Evaluate(ctx, typesafe.Request{State: state, Model: d.model, Questions: questions})
	return evaluation{resp: resp, meta: meta, built: ev, sent: sent, questions: questions}, err
}

// signalsFrom maps the companion answers of one request. A signal is set only for a question
// that was asked and answered with the expected type. known_defect maps its option id back to
// the key that was in the state (spec R5) and names nothing for `none`. When the injection
// question was answered the decision is guarded, and the blocks it covered — those the sent
// state carried as built — are recorded with their hashes (spec R8); MembersChecked follows.
func signalsFrom(e evaluation) Signals {
	noul := func(key string) *float64 {
		if _, asked := e.questions[key]; !asked {
			return nil
		}
		a, ok := e.resp.Answers[key]
		if !ok || a.Type != "noul" {
			return nil
		}
		v := a.Noul
		return &v
	}
	s := Signals{
		Injection:    noul(questionInjection),
		FlakyHistory: noul(questionFlakyHistory),
		Recurring:    noul(questionRecurring),
		OutsideApp:   noul(questionOutsideApp),
	}
	if _, asked := e.questions[questionKnownDefect]; asked {
		if a, ok := e.resp.Answers[questionKnownDefect]; ok && a.Type == "choice" {
			if i, ok := knownDefectIndex(a.Choice); ok && i < len(e.sent.LinkedDefects) {
				s.KnownDefect = &KnownDefectSignal{Key: e.sent.LinkedDefects[i].Key, Confidence: a.Confidence}
			}
		}
	}
	if s.Guarded() {
		s.RecordChecked(e.built, e.sent)
	}
	return s
}
