package failureanalysis

import (
	"context"
	"ttgo/pkg/tracker/typesafe"
)

// fakeClient answers through fn and records every request. Shared by decider, analyzer
// and semantic tests in this package.
type fakeClient struct {
	fn    func(req typesafe.Request) (*typesafe.Response, error)
	calls []typesafe.Request
}

func (f *fakeClient) Evaluate(_ context.Context, req typesafe.Request) (*typesafe.Response, error) {
	f.calls = append(f.calls, req)
	return f.fn(req)
}

func (f *fakeClient) ListModels(context.Context) ([]typesafe.Model, error) { return nil, nil }

// decisionResponse builds a valid two-answer response. winnerP is the winner's probability;
// the remainder is spread evenly over the other options.
func decisionResponse(verdict string, verdictConf, verdictP float64, defect string, defectConf, defectP float64) *typesafe.Response {
	spread := func(opts []string, winner string, p float64) map[string]float64 {
		m := make(map[string]float64, len(opts))
		rest := (1 - p) / float64(len(opts)-1)
		for _, o := range opts {
			if o == winner {
				m[o] = p
			} else {
				m[o] = rest
			}
		}
		return m
	}
	return &typesafe.Response{
		Model: "jev-1.13.0",
		Answers: map[string]typesafe.Answer{
			"verdict":     {Type: "choice", Choice: verdict, Confidence: verdictConf, Probabilities: spread(verdictOptions(), verdict, verdictP)},
			"defect_type": {Type: "choice", Choice: defect, Confidence: defectConf, Probabilities: spread(defectTypeOptions(), defect, defectP)},
		},
		Usage: typesafe.Usage{InputTokens: 777, OutputTokens: 9},
	}
}
