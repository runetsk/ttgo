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
	Model                   string // versioned id from the response
	InputTokens             int
	PolicyVersion           string
}

// Decider produces a Decision from evidence. The analyzer treats nil as "not permitted".
type Decider interface {
	Decide(ctx context.Context, ev Evidence) (*Decision, error)
}

type typesafeDecider struct {
	client typesafe.Client
	model  string
}

// NewTypeSafeDecider asks the verdict and defect_type questions in ONE request.
func NewTypeSafeDecider(c typesafe.Client, model string) Decider {
	return &typesafeDecider{client: c, model: model}
}

func (d *typesafeDecider) Decide(ctx context.Context, ev Evidence) (*Decision, error) {
	resp, err := d.evaluate(ctx, ev)
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
		resp, err = d.evaluate(ctx, ev)
	}
	if err != nil {
		return nil, err
	}
	v := resp.Answers["verdict"]
	if !models.ValidVerdicts[v.Choice] { // impossible after client validation, checked anyway
		return nil, &typesafe.Error{Category: typesafe.CategoryParse, Message: fmt.Sprintf("verdict %q is not a known verdict", v.Choice)}
	}
	dt := resp.Answers["defect_type"]
	suggested := dt.Choice
	if suggested == DefectTypeInsufficient || dt.Confidence < DefectTypeSuggestMin || !models.ValidDefectTypes[suggested] {
		suggested = ""
	}
	return &Decision{
		Verdict: v.Choice, VerdictConfidence: v.Confidence, VerdictProbabilities: v.Probabilities,
		SuggestedDefectType: suggested, DefectTypeConfidence: dt.Confidence, DefectTypeProbabilities: dt.Probabilities,
		Model: resp.Model, InputTokens: resp.Usage.InputTokens, PolicyVersion: PolicyVersion,
	}, nil
}

// evaluate renders the state and asks both questions in one request.
func (d *typesafeDecider) evaluate(ctx context.Context, ev Evidence) (*typesafe.Response, error) {
	state, meta := RenderState(ev)
	if meta.TruncationPrefix != "" {
		slog.Debug("failure-analysis: TypeSafe state trimmed", "prefix", meta.TruncationPrefix)
	}
	return d.client.Evaluate(ctx, typesafe.Request{
		State: state, Model: d.model,
		Questions: map[string]typesafe.Question{"verdict": verdictQuestion(), "defect_type": defectTypeQuestion()},
	})
}
