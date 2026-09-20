package failureanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

func grp(key, failureType, msg string, start time.Time, n int) *FailureGroup {
	g := &FailureGroup{Key: key}
	for i := 0; i < n; i++ {
		r := &models.RunResult{ID: fmt.Sprintf("%s-%d", key, i), FailureType: failureType, ErrorMessage: msg,
			TestNameSnapshot: "test " + key, StackTrace: "at " + key, StartTime: start.Add(time.Duration(i) * time.Minute)}
		g.Members = append(g.Members, r)
	}
	g.Representative = g.Members[0]
	return g
}

// pairAnswerer answers every pair question by looking up the two excerpt ids in the request
// state, so tests can script probabilities by group key regardless of chunking or ordering.
func pairAnswerer(p map[string]float64) func(req typesafe.Request) (*typesafe.Response, error) {
	return func(req typesafe.Request) (*typesafe.Response, error) {
		failures := req.State.(map[string]any)["failures"].([]map[string]any)
		resp := &typesafe.Response{Model: "jev-1.13.0", Answers: map[string]typesafe.Answer{}, Usage: typesafe.Usage{InputTokens: 100}}
		for id, q := range req.Questions {
			ins := q.Instructions.(map[string]any)
			cmp := ins["compare"].([]int)
			a := failures[cmp[0]]["id"].(string)
			b := failures[cmp[1]]["id"].(string)
			v, ok := p[a+"|"+b]
			if !ok {
				v = p[b+"|"+a]
			}
			resp.Answers[id] = typesafe.Answer{Type: "noul", Noul: v}
		}
		return resp, nil
	}
}

func TestSemantic_BlocksByFailureTypeAndPrefilters(t *testing.T) {
	t0 := time.Now()
	groups := []*FailureGroup{
		grp("A", "timeout", "Timeout waiting for #checkout button after 5000ms", t0, 3),
		grp("B", "timeout", "Timeout waiting for #checkout button after 7000ms", t0.Add(time.Hour), 2),
		grp("C", "assertion", "Timeout waiting for #checkout button after 5000ms", t0, 2), // different failure_type: never paired with A
		grp("D", "timeout", "Cannot read properties of undefined reading map", t0, 1),     // no lexical overlap with A/B
	}
	fc := &fakeClient{fn: pairAnswerer(map[string]float64{"A|B": 0.95})}
	out, rep, err := MergeGroupsSemantically(context.Background(), SemanticDeps{Client: fc, Model: "jev-1.13.0"}, groups, func() bool { return false })
	require.NoError(t, err)
	require.Equal(t, 2, rep.Blocks)
	require.Equal(t, 1, rep.Candidates, "only A-B passes the Jaccard prefilter inside the timeout block")
	require.Equal(t, 1, rep.Merged)
	require.Len(t, out, 3)
	require.Equal(t, "A", out[0].Key, "merged group keeps the oldest representative's key and is largest")
	require.Len(t, out[0].Members, 5)
	require.InDelta(t, 0.95, out[0].SemanticMembers["B-0"], 1e-9)
	require.InDelta(t, 0.95, out[0].SemanticMembers["B-1"], 1e-9)
	_, hasA := out[0].SemanticMembers["A-1"]
	require.False(t, hasA, "same-signature siblings are not semantic members")
	require.Equal(t, "jev-1.13.0", rep.Model)
	require.Equal(t, SemanticPolicyVersion, rep.PolicyVersion)
	require.Equal(t, 100, rep.InputTokens)
}

func TestSemantic_EdgeThresholdAndCompleteLinkage(t *testing.T) {
	t0 := time.Now()
	msg := "Timeout waiting for #checkout button after %dms"
	groups := []*FailureGroup{
		grp("A", "timeout", fmt.Sprintf(msg, 5000), t0, 1),
		grp("B", "timeout", fmt.Sprintf(msg, 6000), t0.Add(time.Minute), 1),
		grp("C", "timeout", fmt.Sprintf(msg, 7000), t0.Add(2*time.Minute), 1),
	}
	fc := &fakeClient{fn: pairAnswerer(map[string]float64{"A|B": 0.9, "B|C": 0.85, "A|C": 0.3})}
	out, rep, err := MergeGroupsSemantically(context.Background(), SemanticDeps{Client: fc}, groups, func() bool { return false })
	require.NoError(t, err)
	require.Equal(t, 3, rep.Asked)
	require.Len(t, out, 2, "A~B merges (strongest edge), C is blocked by the failed A-C pair: two clusters, never three, never one")
	require.Equal(t, "A", out[0].Key)
	require.Len(t, out[0].Members, 2)
	require.Equal(t, "C", out[1].Key)

	fc2 := &fakeClient{fn: pairAnswerer(map[string]float64{"A|B": SameCauseMin - 0.01, "B|C": 0.1, "A|C": 0.1})}
	out2, _, err := MergeGroupsSemantically(context.Background(), SemanticDeps{Client: fc2}, groups, func() bool { return false })
	require.NoError(t, err)
	require.Len(t, out2, 3, "below threshold nothing merges")
}

func TestSemantic_ClusterSizeCap(t *testing.T) {
	t0 := time.Now()
	var groups []*FailureGroup
	probs := map[string]float64{}
	n := MaxGroupsPerCluster + 2
	for i := 0; i < n; i++ {
		k := fmt.Sprintf("G%02d", i)
		groups = append(groups, grp(k, "timeout", fmt.Sprintf("Timeout waiting for #checkout button after %d ms", 5000+i), t0.Add(time.Duration(i)*time.Minute), 1))
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			probs[fmt.Sprintf("G%02d|G%02d", i, j)] = 0.99
		}
	}
	fc := &fakeClient{fn: pairAnswerer(probs)}
	out, _, err := MergeGroupsSemantically(context.Background(), SemanticDeps{Client: fc}, groups, func() bool { return false })
	require.NoError(t, err)
	require.Len(t, out[0].Members, MaxGroupsPerCluster, "a cluster holds at most MaxGroupsPerCluster original groups")
	require.Len(t, out, 2, "the two groups that did not fit merge with each other (all pairs pass)")
	require.Len(t, out[1].Members, 2)
}

func TestSemantic_ChunkingIsChunkLocalAndDeterministic(t *testing.T) {
	// 30 reps in one block with pairwise overlap -> 435 candidates, capped to MaxPairsPerJob=400
	// and chunked at 100 questions: at least 4 requests; every answer must map back to its pair.
	t0 := time.Now()
	var groups []*FailureGroup
	for i := 0; i < 30; i++ {
		groups = append(groups, grp(fmt.Sprintf("R%02d", i), "timeout", fmt.Sprintf("Timeout waiting for selector checkout-%d after 5000ms", i), t0.Add(time.Duration(i)*time.Minute), 1))
	}
	// All 435 pairs tie on Jaccard (identical token sets), so the 400-cap keeps the pairs that
	// sort first by key; R00|R01 and R02|R03 are among them, R28|R29 would be cut.
	probs := map[string]float64{"R00|R01": 0.97, "R02|R03": 0.96}
	answerer := pairAnswerer(probs)
	seenLocal := map[int]bool{}
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		failures := req.State.(map[string]any)["failures"].([]map[string]any)
		for _, q := range req.Questions {
			cmp := q.Instructions.(map[string]any)["compare"].([]int)
			require.Less(t, cmp[0], len(failures))
			require.Less(t, cmp[1], len(failures))
			seenLocal[cmp[0]] = true
		}
		require.LessOrEqual(t, len(req.Questions), MaxQuestionsPerRequest)
		b, _ := json.Marshal(req)
		require.LessOrEqual(t, len(b), ChunkCharBudget+2000)
		return answerer(req)
	}}
	out1, rep1, err := MergeGroupsSemantically(context.Background(), SemanticDeps{Client: fc}, groups, func() bool { return false })
	require.NoError(t, err)
	require.LessOrEqual(t, rep1.Candidates, MaxPairsPerJob)
	require.GreaterOrEqual(t, rep1.Requests, 4)
	require.True(t, seenLocal[0], "chunk-local indexes start at 0 in every chunk")
	require.Equal(t, 2, rep1.Merged)

	fc2 := &fakeClient{fn: answerer}
	out2, rep2, err := MergeGroupsSemantically(context.Background(), SemanticDeps{Client: fc2}, groups, func() bool { return false })
	require.NoError(t, err)
	require.Equal(t, rep1.Candidates, rep2.Candidates)
	require.Equal(t, len(fc.calls), len(fc2.calls), "same input, same requests")
	for i := range out1 {
		require.Equal(t, out1[i].Key, out2[i].Key, "same input, same result")
	}
}

func TestSemantic_OversizedPairIsSkipped(t *testing.T) {
	t0 := time.Now()
	huge := strings.Repeat("Timeout waiting for #checkout button ", 3000) // excerpt caps keep this small...
	groups := []*FailureGroup{grp("A", "timeout", huge, t0, 1), grp("B", "timeout", huge+" x", t0, 1)}
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		return nil, &typesafe.Error{Status: 422, Category: typesafe.CategoryValidation, Oversized: true}
	}}
	out, rep, err := MergeGroupsSemantically(context.Background(), SemanticDeps{Client: fc}, groups, func() bool { return false })
	require.NoError(t, err, "an oversized single-pair chunk is skipped, not an error")
	require.Equal(t, 1, rep.Skipped)
	require.Len(t, out, 2)
}

func TestSemantic_OversizedChunkSplitsOnce(t *testing.T) {
	t0 := time.Now()
	groups := []*FailureGroup{
		grp("A", "timeout", "Timeout waiting for #checkout button after 5000ms", t0, 1),
		grp("B", "timeout", "Timeout waiting for #checkout button after 6000ms", t0, 1),
		grp("C", "timeout", "Timeout waiting for #checkout button after 7000ms", t0, 1),
	}
	answerer := pairAnswerer(map[string]float64{"A|B": 0.9, "B|C": 0.9, "A|C": 0.9})
	calls := 0
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		calls++
		if calls == 1 {
			return nil, &typesafe.Error{Status: 422, Category: typesafe.CategoryValidation, Oversized: true}
		}
		return answerer(req)
	}}
	out, rep, err := MergeGroupsSemantically(context.Background(), SemanticDeps{Client: fc}, groups, func() bool { return false })
	require.NoError(t, err)
	require.Equal(t, 3, calls, "one oversized chunk becomes two halves, each sent once")
	require.Equal(t, 3, rep.Requests)
	require.Len(t, out, 1)
}

func TestSemantic_ClientErrorLeavesGroupsUntouched(t *testing.T) {
	t0 := time.Now()
	groups := []*FailureGroup{
		grp("A", "timeout", "Timeout waiting for #checkout button after 5000ms", t0, 1),
		grp("B", "timeout", "Timeout waiting for #checkout button after 6000ms", t0, 1),
	}
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		return nil, &typesafe.Error{Status: 529, Category: typesafe.CategoryOverloaded}
	}}
	out, _, err := MergeGroupsSemantically(context.Background(), SemanticDeps{Client: fc}, groups, func() bool { return false })
	require.Error(t, err)
	require.Equal(t, groups, out)
}

func TestSemantic_CancellationBeforeChunkAndAfterResponse(t *testing.T) {
	t0 := time.Now()
	var groups []*FailureGroup
	for i := 0; i < 30; i++ {
		groups = append(groups, grp(fmt.Sprintf("R%02d", i), "timeout", fmt.Sprintf("Timeout waiting for selector checkout-%d after 5000ms", i), t0, 1))
	}
	answerer := pairAnswerer(map[string]float64{})
	calls := 0
	fc := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) { calls++; return answerer(req) }}
	cancelledAfterFirst := func() bool { return calls >= 1 }
	out, _, err := MergeGroupsSemantically(context.Background(), SemanticDeps{Client: fc}, groups, cancelledAfterFirst)
	require.ErrorIs(t, err, ErrCancelled)
	require.Equal(t, 1, calls, "cancellation is observed right after the first response")
	require.Equal(t, groups, out)

	errFC := &fakeClient{fn: func(req typesafe.Request) (*typesafe.Response, error) {
		return nil, &typesafe.Error{Status: 529, Category: typesafe.CategoryOverloaded}
	}}
	_, _, err = MergeGroupsSemantically(context.Background(), SemanticDeps{Client: errFC}, groups, func() bool { return true })
	require.ErrorIs(t, err, ErrCancelled, "cancellation wins over a client error")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = MergeGroupsSemantically(ctx, SemanticDeps{Client: &fakeClient{fn: answerer}}, groups, func() bool { return false })
	require.True(t, errors.Is(err, context.Canceled))
}

func TestSemantic_RedactionAppliedToExcerpts(t *testing.T) {
	t0 := time.Now()
	groups := []*FailureGroup{
		grp("A", "timeout", "Timeout with token Bearer abcdefghijklmnopqrstuvwxyz0123456789 at checkout", t0, 1),
		grp("B", "timeout", "Timeout with token Bearer abcdefghijklmnopqrstuvwxyz0123456789 at checkout again", t0, 1),
	}
	fc := &fakeClient{fn: pairAnswerer(map[string]float64{})}
	_, _, err := MergeGroupsSemantically(context.Background(), SemanticDeps{Client: fc, Redact: true}, groups, func() bool { return false })
	require.NoError(t, err)
	b, _ := json.Marshal(fc.calls[0].State)
	require.NotContains(t, string(b), "abcdefghijklmnopqrstuvwxyz0123456789")
}

func TestTokenJaccard(t *testing.T) {
	require.InDelta(t, 1.0, tokenJaccard("timeout waiting for button", "timeout waiting for button"), 1e-9)
	require.InDelta(t, 0.0, tokenJaccard("alpha beta", "gamma delta"), 1e-9)
	require.Greater(t, tokenJaccard("Timeout waiting for #checkout button after 5000ms", "Timeout waiting for #checkout button after 7000ms"), LexicalMin)
}
