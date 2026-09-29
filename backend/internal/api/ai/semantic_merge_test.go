package ai_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"ttgo/internal/api/ai"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/require"
)

// mergeFixture is a completed job whose semantic pass merged three signature groups: the
// representative's (rep), X (two results) and Y (one result). The pair record says so.
type mergeFixture struct {
	job                *models.RunAnalysisJob
	repRR, x1, x2, y   *models.RunResult
	rep, cx1, cx2, cy  *models.RunResultAnalysis
	repSig, xSig, ySig string
}

func seedMerge(t *testing.T, e *quickEnv) mergeFixture {
	t.Helper()
	var f mergeFixture
	add := func(name, msg string) *models.RunResult {
		rr := &models.RunResult{TestRunID: e.runID, TestNameSnapshot: name, AttemptNumber: 1, Status: models.StatusFail,
			FailureType: "http", ErrorMessage: msg}
		require.NoError(t, e.s.AddRunResult(rr))
		return rr
	}
	f.repRR = add("pay rep", "POST /api/pay returned 503: gateway circuit open")
	f.x1 = add("pay x1", "Payment failed: gateway unavailable (HTTP 503)")
	f.x2 = add("pay x2", "Payment failed: gateway unavailable (HTTP 503)")
	f.y = add("pay y", "Payment authorization error: 503 from gateway")
	f.repSig = failureanalysis.Signature("http", f.repRR.ErrorMessage)
	f.xSig = failureanalysis.Signature("http", f.x1.ErrorMessage)
	f.ySig = failureanalysis.Signature("http", f.y.ErrorMessage)

	job, _, err := e.s.MaybeEnqueueForRun(e.runID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)
	err = e.s.DB().Model(job).Update("status", models.RunAnalysisJobStatusCompleted).Error
	require.NoError(t, err)
	f.job = job
	score, p := 0.95, 0.9
	mk := func(rr *models.RunResult, clone bool) *models.RunResultAnalysis {
		a := &models.RunResultAnalysis{RunResultID: rr.ID, Engine: models.AnalysisEngineTypeSafe, ModelName: "jev",
			Verdict: models.VerdictInfrastructure, Confidence: models.ConfidenceHigh, ConfidenceScore: &score,
			NarrativeStatus: models.NarrativeStatusOK, JobID: &job.ID, DedupGroupKey: &f.repSig}
		if clone {
			a.SourceAnalysisID, a.DedupMethod, a.DedupPSame, a.DedupModel = &f.rep.ID, models.DedupMethodSemantic, &p, "jev-1.13"
		}
		out, err := e.s.CreateAnalysis(a)
		require.NoError(t, err)
		return out
	}
	f.rep = mk(f.repRR, false)
	f.cx1, f.cx2, f.cy = mk(f.x1, true), mk(f.x2, true), mk(f.y, true)
	pp := func(v float64) *float64 { return &v }
	require.NoError(t, e.s.SaveSemanticPairs(job.ID, e.runID, "jev-latest", failureanalysis.SemanticPolicyVersion,
		[]failureanalysis.SemanticPairOutcome{
			{SigA: f.repSig, SigB: f.xSig, ResultA: f.repRR.ID, ResultB: f.x1.ID, P: pp(0.91), Source: failureanalysis.SemanticSourceTypeSafe, AnsweredModel: "jev-1.13", Merged: true},
			{SigA: f.repSig, SigB: f.ySig, ResultA: f.repRR.ID, ResultB: f.y.ID, P: pp(0.88), Source: failureanalysis.SemanticSourceTypeSafe, AnsweredModel: "jev-1.13", Merged: true},
			{SigA: f.xSig, SigB: f.ySig, ResultA: f.x1.ID, ResultB: f.y.ID, P: pp(0.86), Source: failureanalysis.SemanticSourceTypeSafe, AnsweredModel: "jev-1.13", Merged: true},
		}))
	return f
}

func inspect(e *quickEnv, rr *models.RunResult, a *models.RunResultAnalysis) (int, ai.SemanticMergeView) {
	rec := serve(e.h.GetSemanticMerge, "GET", map[string]string{"id": rr.ID, "analysisId": a.ID})
	var v ai.SemanticMergeView
	if rec.Code == http.StatusOK {
		_ = json.Unmarshal(rec.Body.Bytes(), &v)
	}
	return rec.Code, v
}

func split(e *quickEnv, rr *models.RunResult, a *models.RunResultAnalysis) (int, string) {
	rec := serve(e.h.SplitSemanticMerge, "POST", map[string]string{"id": rr.ID, "analysisId": a.ID})
	return rec.Code, rec.Body.String()
}

func noDeps(string) (failureanalysis.JobDeps, error) { return failureanalysis.JobDeps{}, nil }

func TestGetSemanticMerge_ShowsThePairAndTheSplitSize(t *testing.T) {
	e := newQuickEnv(t, noDeps)
	f := seedMerge(t, e)
	code, v := inspect(e, f.x2, f.cx2)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, f.repRR.ID, v.Representative.ResultID)
	require.Equal(t, "pay x2", v.Result.TestName)
	require.NotNil(t, v.Pair)
	require.InDelta(t, 0.91, *v.Pair.PSame, 1e-12, "the (representative, X) pair")
	require.Equal(t, failureanalysis.SemanticSourceTypeSafe, v.Pair.Source)
	require.Equal(t, "jev-latest", v.Pair.Model)
	require.Nil(t, v.Pair.RememberedFrom)
	require.Equal(t, 4, v.GroupSize)
	require.Equal(t, 2, v.SplitGroupSize, "both results with X's error go")
	require.Equal(t, 2, v.OtherGroups, "kept apart from the representative's and Y's signatures")
	require.True(t, v.CanSplit, v.SplitBlockedReason)
}

func TestGetSemanticMerge_RememberedPairNamesItsSource(t *testing.T) {
	e := newQuickEnv(t, noDeps)
	f := seedMerge(t, e)
	rows, err := e.s.ListSemanticPairsForJob(f.job.ID)
	require.NoError(t, err)
	var src *models.SemanticPair
	for _, r := range rows {
		if r.SigB == f.xSig || r.SigA == f.xSig {
			if r.SigA == f.repSig || r.SigB == f.repSig {
				src = r
			}
		}
	}
	require.NotNil(t, src)
	require.NoError(t, e.s.DB().Model(src).Updates(map[string]interface{}{"source": failureanalysis.SemanticSourceMemory}).Error)
	orig := *src
	orig.ID, orig.JobID, orig.Source = "orig-row", "older-job", failureanalysis.SemanticSourceTypeSafe
	orig.CreatedAt = orig.CreatedAt.AddDate(0, 0, -3)
	require.NoError(t, e.s.DB().Create(&orig).Error)
	require.NoError(t, e.s.DB().Model(src).Update("source_pair_id", "orig-row").Error)

	code, v := inspect(e, f.x1, f.cx1)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, failureanalysis.SemanticSourceMemory, v.Pair.Source)
	require.NotNil(t, v.Pair.RememberedFrom)
	require.WithinDuration(t, orig.CreatedAt, *v.Pair.RememberedFrom, 0)
}

func TestSplitSemanticMerge_PinsEveryClusterSignatureAndQueuesTheGroup(t *testing.T) {
	e := newQuickEnv(t, noDeps)
	f := seedMerge(t, e)
	code, body := split(e, f.x1, f.cx1)
	require.Equal(t, http.StatusCreated, code, body)
	var job models.RunAnalysisJob
	require.NoError(t, json.Unmarshal([]byte(body), &job))
	require.Equal(t, f.cx1.ID, *job.SplitFromAnalysisID)
	require.ElementsMatch(t, []string{f.x1.ID, f.x2.ID}, keys(store.ScopeResultIDs(&job)))

	remember := e.s.SemanticMemory("jev-latest", failureanalysis.SemanticPolicyVersion, job.CreatedAt)
	for _, other := range []string{f.repSig, f.ySig} {
		r, ok := remember(f.xSig, other)
		require.True(t, ok)
		require.Equal(t, failureanalysis.SemanticSourceHuman, r.Source)
	}
	r, ok := remember(f.repSig, f.ySig)
	require.True(t, ok)
	require.Equal(t, failureanalysis.SemanticSourceTypeSafe, r.Source, "the rest of the group is untouched")

	code, body = split(e, f.y, f.cy)
	require.Equal(t, http.StatusConflict, code, "the split's own job is running")
	require.Contains(t, body, "an analysis is running")
	_, v := inspect(e, f.y, f.cy)
	require.False(t, v.CanSplit)
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestSplitSemanticMerge_Refusals(t *testing.T) {
	e := newQuickEnv(t, noDeps)
	f := seedMerge(t, e)

	code, _ := split(e, f.repRR, f.rep)
	require.Equal(t, http.StatusConflict, code, "a representative is not a semantic clone")
	code, _ = split(e, f.x1, f.cy)
	require.Equal(t, http.StatusNotFound, code, "the analysis belongs to another result")

	// A newer analysis of the result supersedes the merge.
	_, err := e.s.CreateAnalysis(&models.RunResultAnalysis{RunResultID: f.y.ID, Engine: models.AnalysisEngineGenerative,
		Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh})
	require.NoError(t, err)
	code, body := split(e, f.y, f.cy)
	require.Equal(t, http.StatusConflict, code)
	require.Contains(t, body, "newer analysis")

	// X's results now pass: nothing is left to split.
	for _, rr := range []*models.RunResult{f.x1, f.x2} {
		require.NoError(t, e.s.DB().Model(rr).Update("status", models.StatusPass).Error)
	}
	code, body = split(e, f.x1, f.cx1)
	require.Equal(t, http.StatusConflict, code)
	require.Contains(t, body, "no longer the latest failing attempts")

	var n int64
	require.NoError(t, e.s.DB().Model(&models.SemanticPair{}).Where("source = ?", failureanalysis.SemanticSourceHuman).Count(&n).Error)
	require.Zero(t, n, "a refused split records nothing")
}

func TestSplitSemanticMerge_RefusedWhileAIIsOff(t *testing.T) {
	e := newQuickEnv(t, noDeps)
	f := seedMerge(t, e)
	_, err := e.s.UpdateAIFeatureSettings(false)
	require.NoError(t, err)
	code, body := split(e, f.x1, f.cx1)
	require.Equal(t, http.StatusConflict, code)
	require.Contains(t, body, "switched off")
}
