package failureanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

// The golden file holds what Analyze stored for every route of spec §A1 before the
// Decide/Narrate split: the result (wall-clock timings zeroed), the error text and every LLM
// request. Record it once on the unchanged code with
//
//	go test -tags sqlite_fts5 ./pkg/tracker/failureanalysis/ -run TestAnalyze_ParityWithGolden -args -update-parity
//
// and never re-record it to make a later change pass: differences the spec asks for go in
// parityIntended instead.
var updateParity = flag.Bool("update-parity", false, "rewrite "+parityGolden+" from the current Analyze")

const parityGolden = "testdata/analyze_parity.golden.json"

// narrJSON is a complete narrative reply.
const narrJSON = `{"summary":"S","next_action":"N","rationale":"R"}`

// parityIntended lists, per case, the differences from the pre-Wave-2 output that spec §A1
// asks for. Each is applied to the golden (legacy) record before comparing.
var parityIntended = map[string]func(*AnalyzeResult){
	// §A1: a failed attempt is built inside Decide. It keeps HistoryAvailable and the LLM calls
	// and finish reason it spent (DecisionMs and LLMMs are wall-clock and zeroed on both sides).
	"ts_down_no_fallback":    func(r *AnalyzeResult) { r.HistoryAvailable = true },
	"ts_down_no_narrator":    func(r *AnalyzeResult) { r.HistoryAvailable = true },
	"ts_down_fallback_error": func(r *AnalyzeResult) { r.HistoryAvailable, r.LLMCalls = true, 1 },
	"gen_error":              func(r *AnalyzeResult) { r.HistoryAvailable, r.LLMCalls = true, 1 },
	"gen_repair_error":       func(r *AnalyzeResult) { r.HistoryAvailable, r.LLMCalls = true, 2 },
	"gen_truncated":          func(r *AnalyzeResult) { r.HistoryAvailable, r.LLMCalls, r.FinishReason = true, 2, "length" },
	"gen_no_provider":        func(r *AnalyzeResult) { r.HistoryAvailable = true },
	// §A1 route table: a failed takeover keeps the failed attempt's calls and finish reason,
	// not only its tokens.
	"takeover_error":     func(r *AnalyzeResult) { r.LLMCalls = 1 },
	"takeover_truncated": func(r *AnalyzeResult) { r.LLMCalls, r.FinishReason = 2, "length" },
}

// tape records every request a provider receives and forwards it.
type tape struct {
	p    llm.Provider
	reqs []llm.ChatRequest
}

func (t *tape) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	t.reqs = append(t.reqs, req)
	return t.p.Chat(ctx, req)
}

type parityRequest struct {
	Model     string            `json:"model"`
	MaxTokens int               `json:"max_tokens"`
	Messages  []llm.ChatMessage `json:"messages"`
}

type parityRecord struct {
	Err      string          `json:"err,omitempty"`
	Result   *AnalyzeResult  `json:"result"`
	Requests []parityRequest `json:"requests,omitempty"`
}

type parityCase struct {
	name  string
	build func() (AnalyzeDeps, AnalyzeContext)
}

// parityContext is baseContext with this test's history, so HistoryAvailable is true.
func parityContext() AnalyzeContext {
	in := baseContext()
	in.SimilarFailures = []SimilarFailure{{RunStartedAt: time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC),
		Status: "FAIL", ErrorMessage: "expected 401, got 500", DefectType: "product_bug"}}
	in.SimilarFailuresRollup = "product_bug " + timesGlyph + "1"
	return in
}

func parityCases() []parityCase {
	boom := errors.New("boom")
	llmTimeout := &llm.ProviderError{Category: llm.ErrCatTimeout, Message: "deadline"}
	tsDown := &typesafe.Error{Category: typesafe.CategoryRateLimit, Status: 429, Message: "slow down"}
	decided := func(verdict string, conf float64) *Decision {
		d := flakyDecision()
		d.Verdict, d.VerdictConfidence = verdict, conf
		return d
	}
	closeCall := func() *Decision {
		d := flakyDecision()
		d.VerdictProbabilities = map[string]float64{"flaky_test": 0.5, "product_bug": 0.42, "environment": 0.08}
		return d
	}
	ts := func(p llm.Provider, d *Decision) AnalyzeDeps {
		return AnalyzeDeps{Narrative: p, NarrativeModel: "gpt-test", Decider: fixedDecider{d: d}, DeciderModel: "jev-latest"}
	}
	takeover := func(p llm.Provider, d *Decision) AnalyzeDeps {
		deps := ts(p, d)
		deps.EscalateBelow = 0.90
		return deps
	}
	tsFails := func(p llm.Provider) AnalyzeDeps {
		deps := AnalyzeDeps{Decider: fixedDecider{err: tsDown}, DeciderModel: "jev-latest", NarrativeModel: "gpt-test"}
		if p != nil {
			deps.Narrative = p
		}
		return deps
	}
	gen := func(p llm.Provider) AnalyzeDeps { return AnalyzeDeps{Narrative: p, NarrativeModel: "gpt-test"} }
	stub := func(rs ...string) *stubProvider { return &stubProvider{responses: rs} }
	failing := func(errs ...error) *stubProvider {
		return &stubProvider{responses: make([]string, len(errs)), errs: errs}
	}
	cut := func(content string) *finishProvider {
		return &finishProvider{replies: [][2]string{{content, "length"}, {content, "length"}}}
	}
	c := func(name string, deps func() AnalyzeDeps) parityCase {
		return parityCase{name: name, build: func() (AnalyzeDeps, AnalyzeContext) { return deps(), parityContext() }}
	}
	return []parityCase{
		// TypeSafe decides, the LLM explains.
		c("ts_explained", func() AnalyzeDeps { return ts(stub(narrJSON), flakyDecision()) }),
		c("ts_explained_runner_up", func() AnalyzeDeps { return ts(stub(narrJSON), closeCall()) }),
		c("ts_narration_repaired", func() AnalyzeDeps { return ts(stub(`{}`, narrJSON), flakyDecision()) }),
		c("ts_narration_unparseable", func() AnalyzeDeps { return ts(stub("nope", "still nope"), flakyDecision()) }),
		c("ts_narration_error", func() AnalyzeDeps { return ts(failing(boom), flakyDecision()) }),
		c("ts_narration_repair_error", func() AnalyzeDeps {
			return ts(&stubProvider{responses: []string{"nope", ""}, errs: []error{nil, boom}}, flakyDecision())
		}),
		c("ts_narration_timeout", func() AnalyzeDeps { return ts(failing(llmTimeout), flakyDecision()) }),
		c("ts_narration_truncated", func() AnalyzeDeps { return ts(cut(`{"summary":"The`), flakyDecision()) }),
		{name: "ts_narration_template_error", build: func() (AnalyzeDeps, AnalyzeContext) {
			in := parityContext()
			in.PromptTemplate = "{{ .Broken"
			return ts(stub(narrJSON), flakyDecision()), in
		}},
		// TypeSafe decides, nothing to narrate.
		c("ts_explanations_off", func() AnalyzeDeps {
			d := ts(stub(narrJSON), flakyDecision())
			d.NarrativeSkipped = true
			return d
		}),
		c("ts_no_narrator", func() AnalyzeDeps {
			return AnalyzeDeps{Decider: fixedDecider{d: flakyDecision()}, DeciderModel: "jev-latest"}
		}),
		c("ts_no_narrator_reason", func() AnalyzeDeps {
			return AnalyzeDeps{Decider: fixedDecider{d: flakyDecision()}, DeciderModel: "jev-latest",
				LLMUnavailableReason: "the LLM key cannot be decrypted"}
		}),
		// Takeover below the threshold.
		c("takeover_ok", func() AnalyzeDeps { return takeover(stub(goodVerdict), decided(models.VerdictUnknown, 0.48)) }),
		c("takeover_not_needed", func() AnalyzeDeps { return takeover(stub(narrJSON), flakyDecision()) }),
		c("takeover_explanations_off", func() AnalyzeDeps {
			d := takeover(stub(goodVerdict), decided(models.VerdictUnknown, 0.52))
			d.NarrativeSkipped = true
			return d
		}),
		c("takeover_error", func() AnalyzeDeps { return takeover(failing(llmTimeout), decided(models.VerdictFlakyTest, 0.60)) }),
		c("takeover_unparseable", func() AnalyzeDeps {
			return takeover(&finishProvider{replies: [][2]string{{"not json", "stop"}, {`{"type": "json_object"}`, "stop"}}},
				decided(models.VerdictFlakyTest, 0.60))
		}),
		c("takeover_truncated", func() AnalyzeDeps { return takeover(cut(cutOff), decided(models.VerdictFlakyTest, 0.60)) }),
		// TypeSafe unavailable.
		c("ts_down_fallback", func() AnalyzeDeps { return tsFails(stub(goodVerdict)) }),
		c("ts_down_fallback_error", func() AnalyzeDeps { return tsFails(failing(boom)) }),
		c("ts_down_no_fallback", func() AnalyzeDeps {
			d := tsFails(stub(goodVerdict))
			d.NoLLMFallback = true
			return d
		}),
		c("ts_down_no_narrator", func() AnalyzeDeps { return tsFails(nil) }),
		// Generative-only.
		c("gen_ok", func() AnalyzeDeps { return gen(stub(goodVerdict)) }),
		c("gen_repaired", func() AnalyzeDeps { return gen(stub("nope", goodVerdict)) }),
		c("gen_unparseable", func() AnalyzeDeps { return gen(stub("nope", "still nope")) }),
		c("gen_error", func() AnalyzeDeps { return gen(failing(errors.New("provider unavailable"))) }),
		c("gen_repair_error", func() AnalyzeDeps {
			return gen(&stubProvider{responses: []string{"nope", ""}, errs: []error{nil, boom}})
		}),
		c("gen_truncated", func() AnalyzeDeps { return gen(cut(cutOff)) }),
		c("gen_no_provider", func() AnalyzeDeps { return AnalyzeDeps{} }),
	}
}

// runParity runs one case through Analyze and records what a caller stores.
func runParity(t *testing.T, pc parityCase) parityRecord {
	t.Helper()
	deps, in := pc.build()
	var tp *tape
	if deps.Narrative != nil {
		tp = &tape{p: deps.Narrative}
		deps.Narrative = tp
	}
	res, err := Analyze(context.Background(), deps, in)
	rec := parityRecord{}
	if err != nil {
		rec.Err = err.Error()
		if res == nil { // an error before Decide built its failed result (none in these cases)
			res = FailedResult(err, deps, in)
		}
	}
	require.NotNil(t, res, pc.name)
	res.DecisionMs, res.LLMMs = 0, 0 // wall-clock timings are never byte-stable
	rec.Result = res
	if tp != nil {
		for _, r := range tp.reqs {
			rec.Requests = append(rec.Requests, parityRequest{Model: r.Model, MaxTokens: r.MaxTokens, Messages: r.Messages})
		}
	}
	return rec
}

func TestAnalyze_ParityWithGolden(t *testing.T) {
	got := map[string]parityRecord{}
	for _, pc := range parityCases() {
		got[pc.name] = runParity(t, pc)
	}
	if *updateParity {
		b, err := json.MarshalIndent(got, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(parityGolden), 0o755))
		require.NoError(t, os.WriteFile(parityGolden, append(b, '\n'), 0o644))
		t.Skip("golden rewritten")
	}
	raw, err := os.ReadFile(parityGolden)
	require.NoError(t, err, "record the golden once, on the pre-change code, with -args -update-parity")
	var want map[string]parityRecord
	require.NoError(t, json.Unmarshal(raw, &want))
	require.Len(t, got, len(want), "a case was added or removed; the golden covers exactly parityCases()")
	for name, g := range got {
		w, ok := want[name]
		require.True(t, ok, "case %s missing from the golden", name)
		if fix := parityIntended[name]; fix != nil {
			fix(w.Result)
		}
		wb, err := json.Marshal(w)
		require.NoError(t, err)
		gb, err := json.Marshal(g)
		require.NoError(t, err)
		require.JSONEq(t, string(wb), string(gb), name)
	}
}
