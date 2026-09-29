package failureanalysis

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

// memoryGroups are three timeout groups that all pass the lexical prefilter with each other.
func memoryGroups() []*FailureGroup {
	t0 := time.Now()
	msg := "Timeout waiting for #checkout button after %dms"
	return []*FailureGroup{
		grp("A", "timeout", fmt.Sprintf(msg, 5000), t0, 2),
		grp("B", "timeout", fmt.Sprintf(msg, 6000), t0.Add(time.Minute), 1),
		grp("C", "timeout", fmt.Sprintf(msg, 7000), t0.Add(2*time.Minute), 1),
	}
}

// askedPairs lists the "a|b" excerpt-id pairs TypeSafe was asked about.
func askedPairs(fc *fakeClient) []string {
	var out []string
	for _, req := range fc.calls {
		failures := req.State.(map[string]any)["failures"].([]map[string]any)
		for _, q := range req.Questions {
			cmp := q.Instructions.(map[string]any)["compare"].([]int)
			a, b := failures[cmp[0]]["id"].(string), failures[cmp[1]]["id"].(string)
			if a > b {
				a, b = b, a
			}
			out = append(out, a+"|"+b)
		}
	}
	return out
}

func rememberFrom(m map[string]Remembered) func(a, b string) (Remembered, bool) {
	return func(a, b string) (Remembered, bool) {
		if a > b {
			a, b = b, a
		}
		r, ok := m[a+"|"+b]
		return r, ok
	}
}

func TestSemanticMemory_RememberedPairsAreNotAskedButStillMerge(t *testing.T) {
	fc := &fakeClient{fn: pairAnswerer(map[string]float64{"A|C": 0.9, "B|C": 0.9})}
	deps := SemanticDeps{Client: fc, Model: "jev-latest", Remember: rememberFrom(map[string]Remembered{
		"A|B": {P: 0.93, Source: SemanticSourceTypeSafe, CreatedAt: time.Now().Add(-48 * time.Hour)},
	})}
	out, rep, err := MergeGroupsSemantically(context.Background(), deps, memoryGroups(), func() bool { return false })
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"A|C", "B|C"}, askedPairs(fc), "the remembered pair is not sent")
	require.Equal(t, 3, rep.Candidates)
	require.Equal(t, 2, rep.Asked)
	require.Equal(t, 1, rep.Remembered)
	require.Zero(t, rep.HumanBlocked)
	require.Len(t, out, 1, "the remembered answer counts for complete linkage like a fresh one")
	require.Equal(t, 2, rep.Merged)

	require.Len(t, rep.Pairs, 3)
	for _, p := range rep.Pairs {
		require.True(t, p.Merged, "%s|%s", p.SigA, p.SigB)
		require.Less(t, p.SigA, p.SigB)
		require.NotNil(t, p.P)
		if p.SigA == "A" && p.SigB == "B" {
			require.Equal(t, SemanticSourceMemory, p.Source)
			require.InDelta(t, 0.93, *p.P, 1e-12)
			require.Equal(t, "", p.AnsweredModel)
			require.Equal(t, "A-0", p.ResultA)
			require.Equal(t, "B-0", p.ResultB)
		} else {
			require.Equal(t, SemanticSourceTypeSafe, p.Source)
			require.Equal(t, "jev-1.13.0", p.AnsweredModel)
		}
	}
}

func TestSemanticMemory_APersonsSplitKeepsThePairApart(t *testing.T) {
	fc := &fakeClient{fn: pairAnswerer(map[string]float64{"A|B": 0.95, "A|C": 0.95, "B|C": 0.95})}
	deps := SemanticDeps{Client: fc, Model: "jev-latest", Remember: rememberFrom(map[string]Remembered{
		"A|C": {Source: SemanticSourceHuman, CreatedAt: time.Now().AddDate(-1, 0, 0)},
	})}
	out, rep, err := MergeGroupsSemantically(context.Background(), deps, memoryGroups(), func() bool { return false })
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"A|B", "B|C"}, askedPairs(fc))
	require.Equal(t, 1, rep.HumanBlocked)
	require.Zero(t, rep.Remembered)
	require.Len(t, out, 2, "A and C never share a cluster: complete linkage needs every cross pair")
	for _, g := range out {
		keys := map[string]bool{}
		for _, m := range g.Members {
			keys[m.ID[:1]] = true
		}
		require.False(t, keys["A"] && keys["C"])
	}
	for _, p := range rep.Pairs {
		if p.SigA == "A" && p.SigB == "C" {
			require.Equal(t, SemanticSourceHuman, p.Source)
			require.NotNil(t, p.P)
			require.Zero(t, *p.P)
			require.False(t, p.Merged)
		}
	}
}

func TestSemanticMemory_EverythingRememberedMakesNoRequest(t *testing.T) {
	fc := &fakeClient{fn: func(typesafe.Request) (*typesafe.Response, error) {
		t.Fatal("no request expected")
		return nil, nil
	}}
	old := time.Now().Add(-time.Hour)
	deps := SemanticDeps{Client: fc, Model: "jev-latest", Remember: rememberFrom(map[string]Remembered{
		"A|B": {P: 0.2, Source: SemanticSourceTypeSafe, CreatedAt: old},
		"A|C": {P: 0.2, Source: SemanticSourceTypeSafe, CreatedAt: old},
		"B|C": {P: 0.2, Source: SemanticSourceTypeSafe, CreatedAt: old},
	})}
	out, rep, err := MergeGroupsSemantically(context.Background(), deps, memoryGroups(), func() bool { return false })
	require.NoError(t, err)
	require.Len(t, out, 3)
	require.Equal(t, 3, rep.Remembered)
	require.Zero(t, rep.Requests)
	require.Zero(t, rep.InputTokens)
	require.Len(t, rep.Pairs, 3)
	for _, p := range rep.Pairs {
		require.False(t, p.Merged)
	}
}

func TestSemanticMemory_WithoutRememberNothingChanges(t *testing.T) {
	fc := &fakeClient{fn: pairAnswerer(map[string]float64{"A|B": 0.95})}
	_, rep, err := MergeGroupsSemantically(context.Background(), SemanticDeps{Client: fc, Model: "jev-latest"},
		memoryGroups()[:2], func() bool { return false })
	require.NoError(t, err)
	require.Equal(t, 1, rep.Asked)
	require.Zero(t, rep.Remembered)
	require.Len(t, rep.Pairs, 1)
	require.True(t, rep.Pairs[0].Merged)
}

func TestSemanticReport_SummaryJSON(t *testing.T) {
	rep := SemanticReport{Blocks: 2, Candidates: 5, Asked: 3, Remembered: 1, HumanBlocked: 1, Merged: 2, Skipped: 0, Requests: 1,
		Pairs: []SemanticPairOutcome{{SigA: "a", SigB: "b"}}}
	var got map[string]int
	require.NoError(t, json.Unmarshal([]byte(rep.SummaryJSON()), &got))
	require.Equal(t, map[string]int{"blocks": 2, "candidates": 5, "asked": 3, "remembered": 1, "human_blocked": 1,
		"merged": 2, "skipped": 0, "requests": 1}, got, "the pairs are stored as rows, not in the summary")
}
