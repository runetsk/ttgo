package ai_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

// fitsClient answers fits_ questions with fit and the injection question with injection (or
// fails every request with err), and keeps the requests.
type fitsClient struct {
	mu        sync.Mutex
	fit       float64
	injection float64
	err       error
	reqs      []typesafe.Request
}

func (f *fitsClient) Evaluate(_ context.Context, req typesafe.Request) (*typesafe.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, req)
	if f.err != nil {
		return nil, f.err
	}
	resp := &typesafe.Response{Model: "jev-1.13.0", Answers: map[string]typesafe.Answer{}, Usage: typesafe.Usage{InputTokens: 90}}
	for id := range req.Questions {
		p := f.fit
		if id == "injection" {
			p = f.injection
		}
		resp.Answers[id] = typesafe.Answer{Type: "noul", Noul: p}
	}
	return resp, nil
}
func (f *fitsClient) ListModels(context.Context) ([]typesafe.Model, error) { return nil, nil }

type transferGroup struct {
	repRR, sigRR, semRR *models.RunResult
	rep, sig, sem       *models.RunResultAnalysis
}

// seedTransferGroup stores a TypeSafe group — representative, signature and semantic clone —
// with the given narrative status on every row, and the semantic clone's fit.
func seedTransferGroup(t *testing.T, e *quickEnv, narrative string, fit *float64) transferGroup {
	t.Helper()
	add := func(name, msg string) *models.RunResult {
		rr := &models.RunResult{TestRunID: e.runID, TestNameSnapshot: name, AttemptNumber: 1,
			Status: models.StatusFail, FailureType: "timeout", ErrorMessage: msg}
		require.NoError(t, e.s.AddRunResult(rr))
		return rr
	}
	var g transferGroup
	g.repRR = add("checkout representative", "rep-only failure line after 5000ms")
	g.sigRR = add("checkout signature", "rep-only failure line after 5000ms")
	g.semRR = add("checkout clone", "clone-only failure line: POST /api/cart returned 503")
	score := 0.95
	mk := func(rr *models.RunResult, a *models.RunResultAnalysis) *models.RunResultAnalysis {
		a.RunResultID, a.Engine, a.ModelName = rr.ID, models.AnalysisEngineTypeSafe, "jev"
		a.Verdict, a.Confidence, a.ConfidenceScore = models.VerdictFlakyTest, models.ConfidenceHigh, &score
		a.SuggestedDefectType, a.NarrativeStatus = "automation_bug", narrative
		out, err := e.s.CreateAnalysis(a)
		require.NoError(t, err)
		return out
	}
	g.rep = mk(g.repRR, &models.RunResultAnalysis{Summary: "Shared cause", Rationale: "R0"})
	g.sig = mk(g.sigRR, &models.RunResultAnalysis{SourceAnalysisID: &g.rep.ID, DedupMethod: models.DedupMethodSignature,
		Summary: "Shared cause", Rationale: "[Grouped from representative analysis] R0"})
	g.sem = mk(g.semRR, &models.RunResultAnalysis{SourceAnalysisID: &g.rep.ID, DedupMethod: models.DedupMethodSemantic,
		Summary: "Shared cause", Rationale: "[Grouped semantically with representative analysis] R0", NarrativeFit: fit})
	return g
}

func transferDeps(prov *promptProvider, ts typesafe.Client) func(string) (failureanalysis.JobDeps, error) {
	return func(string) (failureanalysis.JobDeps, error) {
		d := failureanalysis.JobDeps{Narrative: prov, NarrativeModel: "mock", Pricing: failureanalysis.Pricing{TypeSafePerMTok: 0.042}}
		if ts != nil {
			d.Transfer = &failureanalysis.TransferDeps{Client: ts, Model: "jev-1.13.0"}
		}
		return d, nil
	}
}

func costKinds(t *testing.T, e *quickEnv) map[string][]*models.AIAnalysisCostEvent {
	t.Helper()
	events, err := e.s.ListAnalysisCostEventsForRun(e.runID)
	require.NoError(t, err)
	out := map[string][]*models.AIAnalysisCostEvent{}
	for _, ev := range events {
		out[ev.Kind] = append(out[ev.Kind], ev)
	}
	return out
}

func explainOwn(e *quickEnv, g transferGroup, query string) *httptest.ResponseRecorder {
	return serveQuery(e.h.ExplainAnalysis, "?scope=result"+query, map[string]string{"id": g.semRR.ID, "analysisId": g.sem.ID})
}

func TestExplainAnalysis_GroupExplainChecksTheSemanticClones(t *testing.T) {
	prov := &promptProvider{reply: explainReply}
	ts := &fitsClient{fit: 0.3}
	e := newQuickEnv(t, transferDeps(prov, ts))
	g := seedTransferGroup(t, e, models.NarrativeStatusSkipped, nil)

	rec := serve(e.h.ExplainAnalysis, "POST", map[string]string{"id": g.repRR.ID, "analysisId": g.rep.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	sem, err := e.s.GetAnalysisByID(g.sem.ID)
	require.NoError(t, err)
	require.Equal(t, "Shared cause", sem.Summary, "the explanation reached the clone")
	require.NotNil(t, sem.NarrativeFit)
	require.InDelta(t, 0.3, *sem.NarrativeFit, 1e-12)
	for _, id := range []string{g.rep.ID, g.sig.ID} {
		a, err := e.s.GetAnalysisByID(id)
		require.NoError(t, err)
		require.Nil(t, a.NarrativeFit)
	}

	require.Len(t, ts.reqs, 1)
	failures := ts.reqs[0].State.(map[string]any)["failures"].([]map[string]any)
	require.Len(t, failures, 2, "the representative and the semantic clone; never the signature clone")
	require.Contains(t, failures[1]["error"], "POST /api/cart returned 503")

	kinds := costKinds(t, e)
	require.Len(t, kinds[models.AnalysisCostKindExplain], 1)
	require.Len(t, kinds[models.AnalysisCostKindTransfer], 1)
	ev := kinds[models.AnalysisCostKindTransfer][0]
	require.Equal(t, 90, ev.TypeSafeInputTokens)
	require.Equal(t, g.rep.ID, *ev.AnalysisID)
}

func TestExplainAnalysis_AFailedCheckKeepsTheGroupExplanation(t *testing.T) {
	prov := &promptProvider{reply: explainReply}
	ts := &fitsClient{err: errors.New("typesafe down")}
	e := newQuickEnv(t, transferDeps(prov, ts))
	g := seedTransferGroup(t, e, models.NarrativeStatusSkipped, nil)

	rec := serve(e.h.ExplainAnalysis, "POST", map[string]string{"id": g.repRR.ID, "analysisId": g.rep.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	sem, err := e.s.GetAnalysisByID(g.sem.ID)
	require.NoError(t, err)
	require.Equal(t, models.NarrativeStatusOK, sem.NarrativeStatus)
	require.Nil(t, sem.NarrativeFit)
	require.Empty(t, costKinds(t, e)[models.AnalysisCostKindTransfer], "nothing billed")
}

func TestExplainAnalysis_ScopeResultExplainsAMismatchedCloneAlone(t *testing.T) {
	prov := &promptProvider{reply: `{"summary":"Own cause","next_action":"Fix the cart API","rationale":"R"}`}
	ts := &fitsClient{fit: 0.9, injection: 0.02}
	e := newQuickEnv(t, transferDeps(prov, ts))
	low := 0.2
	g := seedTransferGroup(t, e, models.NarrativeStatusOK, &low)

	rec := explainOwn(e, g, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decodeRow(t, rec.Body.Bytes())
	require.Equal(t, g.sem.ID, got.ID)
	require.Equal(t, "Own cause", got.Summary)
	require.Equal(t, "Fix the cart API", got.NextAction)
	require.Equal(t, "[Grouped semantically with representative analysis] R", got.Rationale)
	require.True(t, got.NarrativeSplit)
	require.InDelta(t, 0.2, *got.NarrativeFit, 1e-12, "the fit is unchanged")
	require.Equal(t, 200, got.TokenUsagePrompt, "billed to this row")
	require.Equal(t, 2, got.NarrativeRevision, "claimed, then applied")
	require.Equal(t, models.VerdictFlakyTest, got.Verdict, "the decision is untouched")

	rep, err := e.s.GetAnalysisByID(g.rep.ID)
	require.NoError(t, err)
	require.Equal(t, "Shared cause", rep.Summary, "the group keeps its explanation")
	require.Zero(t, rep.NarrativeRevision)

	// R9: one injection-only request over the clone's own evidence, before the LLM.
	require.Len(t, ts.reqs, 1)
	require.Len(t, ts.reqs[0].Questions, 1)
	require.Contains(t, ts.reqs[0].Questions, "injection")

	require.Equal(t, 1, prov.calls)
	require.Contains(t, prov.prompt, "checkout clone", "the clone's own evidence")
	require.Contains(t, prov.prompt, "POST /api/cart returned 503")
	require.NotContains(t, prov.prompt, "rep-only failure line", "no group members")

	kinds := costKinds(t, e)
	require.Len(t, kinds[models.AnalysisCostKindExplain], 2, "the LLM call and the injection check")
	engines := map[string]bool{}
	for _, ev := range kinds[models.AnalysisCostKindExplain] {
		require.Equal(t, g.sem.ID, *ev.AnalysisID)
		engines[ev.Engine] = true
	}
	require.True(t, engines[models.AnalysisCostEngineLLM])
	require.True(t, engines[models.AnalysisCostEngineTypeSafe])
	require.Empty(t, kinds[models.AnalysisCostKindTransfer])
}

func TestExplainAnalysis_ScopeResultInjectionNeedsTheOverride(t *testing.T) {
	prov := &promptProvider{reply: explainReply}
	ts := &fitsClient{injection: 0.95}
	e := newQuickEnv(t, transferDeps(prov, ts))
	low := 0.2
	g := seedTransferGroup(t, e, models.NarrativeStatusOK, &low)

	rec := explainOwn(e, g, "")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "possible prompt injection")
	require.Zero(t, prov.calls, "nothing reaches the LLM")
	row, err := e.s.GetAnalysisByID(g.sem.ID)
	require.NoError(t, err)
	require.False(t, row.NarrativeSplit, "refused before the claim")
	require.Equal(t, models.NarrativeStatusOK, row.NarrativeStatus)
	require.Len(t, costKinds(t, e)[models.AnalysisCostKindExplain], 1, "the check itself was billed")

	require.Equal(t, http.StatusBadRequest, explainOwn(e, g, "&override_injection=maybe").Code)

	rec = explainOwn(e, g, "&override_injection=true")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, prov.calls)
}

func TestExplainAnalysis_ScopeResultWithoutTypeSafeNeedsTheOverride(t *testing.T) {
	prov := &promptProvider{reply: explainReply}
	e := newQuickEnv(t, transferDeps(prov, nil))
	low := 0.2
	g := seedTransferGroup(t, e, models.NarrativeStatusOK, &low)

	rec := explainOwn(e, g, "")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "could not check")
	require.Zero(t, prov.calls)

	rec = explainOwn(e, g, "&override_injection=1")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, 1, prov.calls)
}

func TestExplainAnalysis_ScopeResultRetriesAFailedOwnExplanation(t *testing.T) {
	prov := &promptProvider{err: &llm.ProviderError{Category: llm.ErrCatProvider, StatusCode: 502, Message: "upstream"}}
	e := newQuickEnv(t, transferDeps(prov, &fitsClient{injection: 0.02}))
	low := 0.2
	g := seedTransferGroup(t, e, models.NarrativeStatusOK, &low)

	rec := explainOwn(e, g, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decodeRow(t, rec.Body.Bytes())
	require.Equal(t, models.NarrativeStatusUnavailable, got.NarrativeStatus)
	require.True(t, got.NarrativeSplit)

	prov.err, prov.reply = nil, explainReply
	rec = explainOwn(e, g, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got = decodeRow(t, rec.Body.Bytes())
	require.Equal(t, models.NarrativeStatusOK, got.NarrativeStatus, "a failed own explanation can be retried (R9)")

	rec = explainOwn(e, g, "")
	require.Equal(t, http.StatusConflict, rec.Code, "a written own explanation is not explained again")
}

func TestExplainAnalysis_ScopeResultRefusesAnythingButAMismatchedClone(t *testing.T) {
	prov := &promptProvider{reply: explainReply}
	ts := &fitsClient{fit: 0.9, injection: 0.02}
	e := newQuickEnv(t, transferDeps(prov, ts))
	fine := 0.8
	g := seedTransferGroup(t, e, models.NarrativeStatusOK, &fine)
	for _, c := range []struct {
		rr *models.RunResult
		a  *models.RunResultAnalysis
	}{{g.repRR, g.rep}, {g.sigRR, g.sig}, {g.semRR, g.sem}} {
		rec := serveQuery(e.h.ExplainAnalysis, "?scope=result", map[string]string{"id": c.rr.ID, "analysisId": c.a.ID})
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), "may not apply")
	}

	low := 0.1
	pending := seedTransferGroup(t, e, models.NarrativeStatusPending, &low)
	rec := explainOwn(e, pending, "")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "already being explained")
	require.Zero(t, prov.calls)
	require.Empty(t, ts.reqs, "refused before anything is sent")
}
