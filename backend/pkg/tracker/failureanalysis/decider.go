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

// NewTypeSafeDecider asks the verdict and defect_type questions in ONE request.
func NewTypeSafeDecider(c typesafe.Client, model string) Decider {
	return &typesafeDecider{client: c, model: model}
}

func (d *typesafeDecider) Decide(ctx context.Context, ev Evidence) (*Decision, error) {
	resp, meta, err := d.evaluate(ctx, ev)
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
		resp, meta, err = d.evaluate(ctx, ev)
	}
	if err != nil {
		return nil, err
	}
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
		Model:            resp.Model, InputTokens: resp.Usage.InputTokens, PolicyVersion: PolicyVersionFor(meta.ExamplesSent),
	}, nil
}

// suggestion turns the two answers into the stored defect-type suggestion (policies
// fa-verdict-v5 and v6). The verdict wins where it is sure:
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

// evaluate renders the state and asks both questions in one request. The meta says what the
// state carried after its drop ladder (the policy is stamped from its ExamplesSent).
func (d *typesafeDecider) evaluate(ctx context.Context, ev Evidence) (*typesafe.Response, PromptMeta, error) {
	state, meta := RenderState(ev)
	if meta.TruncationPrefix != "" {
		slog.Debug("failure-analysis: TypeSafe state trimmed", "prefix", meta.TruncationPrefix)
	}
	resp, err := d.client.Evaluate(ctx, typesafe.Request{
		State: state, Model: d.model,
		Questions: map[string]typesafe.Question{"verdict": verdictQuestion(), "defect_type": defectTypeQuestion()},
	})
	return resp, meta, err
}
