package failureanalysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"ttgo/pkg/tracker/typesafe"
)

// ErrCancelled is returned when cancelled() reports true; the worker stops the job.
var ErrCancelled = errors.New("semantic grouping cancelled")

type SemanticReport struct {
	Blocks, Candidates, Asked, Requests, Merged, Skipped, InputTokens int
	Model, PolicyVersion                                              string
	// Remembered pairs reused an earlier TypeSafe answer; HumanBlocked pairs were kept apart by a
	// person's split. Neither was asked.
	Remembered, HumanBlocked int
	// Pairs is every pair the pass decided (asked, remembered or pinned), for the pair record.
	Pairs []SemanticPairOutcome
}

// SemanticPairOutcome is one decided pair of a pass: the two group signatures (SigA < SigB), their
// representative results in this job, the probability used, where it came from (typesafe = asked
// now; memory = reused, SourcePairID names the reused row — a TypeSafe answer, or a person's split,
// in which case P is nil and the pair was kept apart), the model that answered and whether the two
// groups ended merged. A pass never produces a human outcome: only a split writes those rows.
type SemanticPairOutcome struct {
	SigA, SigB       string
	ResultA, ResultB string
	P                *float64
	Source           string
	SourcePairID     string
	AnsweredModel    string
	Merged           bool
}

// SummaryJSON is the job's stored semantic_report: the counts, without the pairs (those are rows).
func (r SemanticReport) SummaryJSON() string {
	b, _ := json.Marshal(map[string]int{
		"blocks": r.Blocks, "candidates": r.Candidates, "asked": r.Asked, "remembered": r.Remembered,
		"human_blocked": r.HumanBlocked, "merged": r.Merged, "skipped": r.Skipped, "requests": r.Requests,
	})
	return string(b)
}

type semPair struct {
	k       int // job-wide index; question id is pair_<k>
	a, b    *FailureGroup
	jaccard float64
}

type semChunk struct {
	pairs []semPair
	reps  []*FailureGroup // in first-referenced order; index == chunk-local position
	local map[string]int  // group key -> chunk-local index
}

// MergeGroupsSemantically asks TypeSafe which representatives share a cause and merges groups
// with complete linkage (spec §7). Any client error or cancellation returns the input groups
// unchanged; partial chunk results are never applied.
func MergeGroupsSemantically(ctx context.Context, deps SemanticDeps, groups []*FailureGroup, cancelled func() bool) ([]*FailureGroup, SemanticReport, error) {
	rep := SemanticReport{PolicyVersion: SemanticPolicyVersion}
	if deps.Client == nil || len(groups) < 2 {
		return groups, rep, nil
	}
	if ctx.Err() != nil {
		return groups, rep, ctx.Err()
	}
	pairs, blocks := candidatePairs(groups)
	rep.Blocks, rep.Candidates = blocks, len(pairs)
	if len(pairs) == 0 {
		return groups, rep, nil
	}

	asked := map[[2]string]float64{} // (keyA,keyB) sorted -> noul
	var outcomes []SemanticPairOutcome
	decide := func(p semPair, v *float64, source, sourcePair, model string) {
		if v != nil {
			asked[pairKey(p.a.Key, p.b.Key)] = *v
		} else {
			asked[pairKey(p.a.Key, p.b.Key)] = 0 // kept apart by a person
		}
		outcomes = append(outcomes, SemanticPairOutcome{SigA: p.a.Key, SigB: p.b.Key, ResultA: p.a.Representative.ID,
			ResultB: p.b.Representative.ID, P: v, Source: source, SourcePairID: sourcePair, AnsweredModel: model})
	}
	// Memory first (spec Wave 4 §2): a person's split pins the pair apart; a remembered TypeSafe
	// answer is reused. Only the rest is asked.
	toAsk := pairs[:0:0]
	rememberedModel := ""
	for _, p := range pairs {
		if deps.Remember != nil {
			if r, ok := deps.Remember(p.a.Key, p.b.Key); ok {
				if r.Source == SemanticSourceHuman {
					decide(p, nil, SemanticSourceMemory, r.ID, "")
					rep.HumanBlocked++
				} else {
					v := r.P
					decide(p, &v, SemanticSourceMemory, r.ID, r.AnsweredModel)
					rep.Remembered++
					if rememberedModel == "" {
						rememberedModel = r.AnsweredModel
					}
				}
				continue
			}
		}
		toAsk = append(toAsk, p)
	}
	chunks, skipped := chunkPairs(toAsk, deps.Redact)
	rep.Skipped = skipped

	for _, ch := range chunks {
		if cancelled() {
			return groups, rep, ErrCancelled
		}
		answers, tokens, model, requests, chunkSkipped, err := askChunk(ctx, deps, ch, cancelled)
		rep.Requests += requests
		rep.Skipped += chunkSkipped
		rep.InputTokens += tokens
		if model != "" {
			rep.Model = model
		}
		if ctx.Err() != nil {
			return groups, rep, ctx.Err()
		}
		if cancelled() {
			return groups, rep, ErrCancelled
		}
		if err != nil {
			return groups, rep, err
		}
		for _, p := range ch.pairs {
			if v, ok := answers[p.k]; ok {
				pv := v
				decide(p, &pv, SemanticSourceTypeSafe, "", model)
				rep.Asked++
			}
		}
	}

	if rep.Model == "" {
		rep.Model = rememberedModel // a pass answered from memory names the model that answered then
	}
	merged := clusterCompleteLinkage(groups, asked)
	rep.Merged = len(groups) - len(merged)
	// A pair is merged when both representatives ended in the same output group.
	clusterOf := map[string]int{}
	for i, g := range merged {
		for _, m := range g.Members {
			clusterOf[m.ID] = i
		}
	}
	for i := range outcomes {
		o := &outcomes[i]
		o.Merged = clusterOf[o.ResultA] == clusterOf[o.ResultB]
	}
	rep.Pairs = outcomes
	return merged, rep, nil
}

func pairKey(a, b string) [2]string {
	if a < b {
		return [2]string{a, b}
	}
	return [2]string{b, a}
}

// tokenJaccard: lowercase words of >= 3 characters.
func tokenJaccard(a, b string) float64 {
	set := func(s string) map[string]struct{} {
		out := map[string]struct{}{}
		for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
			return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
		}) {
			if len(w) >= 3 {
				out[w] = struct{}{}
			}
		}
		return out
	}
	sa, sb := set(a), set(b)
	if len(sa) == 0 && len(sb) == 0 {
		return 0
	}
	inter := 0
	for w := range sa {
		if _, ok := sb[w]; ok {
			inter++
		}
	}
	union := len(sa) + len(sb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// candidatePairs blocks representatives by failure_type, caps each block, prefilters by Jaccard,
// then allocates MaxPairsPerJob across blocks. Deterministic for the same input.
func candidatePairs(groups []*FailureGroup) ([]semPair, int) {
	blocks := map[string][]*FailureGroup{}
	for _, g := range groups {
		ft := g.Representative.FailureType
		blocks[ft] = append(blocks[ft], g)
	}
	types := make([]string, 0, len(blocks))
	for ft := range blocks {
		types = append(types, ft)
	}
	sort.Strings(types)

	perBlock := make([][]semPair, 0, len(types))
	total := 0
	for _, ft := range types {
		reps := blocks[ft]
		sort.SliceStable(reps, func(i, j int) bool {
			if len(reps[i].Members) != len(reps[j].Members) {
				return len(reps[i].Members) > len(reps[j].Members)
			}
			return reps[i].Key < reps[j].Key
		})
		if len(reps) > MaxRepsPerBlock {
			reps = reps[:MaxRepsPerBlock]
		}
		var ps []semPair
		for i := 0; i < len(reps); i++ {
			for j := i + 1; j < len(reps); j++ {
				jac := tokenJaccard(normalize(reps[i].Representative.ErrorMessage), normalize(reps[j].Representative.ErrorMessage))
				if jac >= LexicalMin {
					a, b := reps[i], reps[j]
					if a.Key > b.Key {
						a, b = b, a
					}
					ps = append(ps, semPair{a: a, b: b, jaccard: jac})
				}
			}
		}
		sort.SliceStable(ps, func(i, j int) bool {
			if ps[i].jaccard != ps[j].jaccard {
				return ps[i].jaccard > ps[j].jaccard
			}
			if ps[i].a.Key != ps[j].a.Key {
				return ps[i].a.Key < ps[j].a.Key
			}
			return ps[i].b.Key < ps[j].b.Key
		})
		perBlock = append(perBlock, ps)
		total += len(ps)
	}

	if total > MaxPairsPerJob {
		alloc := make([]int, len(perBlock))
		used, largest := 0, 0
		for i, ps := range perBlock {
			alloc[i] = MaxPairsPerJob * len(ps) / total
			used += alloc[i]
			if len(ps) > len(perBlock[largest]) {
				largest = i
			}
		}
		alloc[largest] += MaxPairsPerJob - used // remainder to the largest block
		for i := range perBlock {
			if len(perBlock[i]) > alloc[i] {
				perBlock[i] = perBlock[i][:alloc[i]]
			}
		}
	}

	var out []semPair
	for _, ps := range perBlock {
		for _, p := range ps {
			p.k = len(out)
			out = append(out, p)
		}
	}
	return out, len(types)
}

func excerpt(g *FailureGroup, redact bool) map[string]any {
	r := g.Representative
	msg, stack, name, ftype := r.ErrorMessage, r.StackTrace, r.TestNameSnapshot, r.FailureType
	if redact {
		// failure_type is free text from the results API, so it is redacted like the rest.
		// The id is the group signature, a SHA-1 hash, and carries no failure text.
		msg, stack, name, ftype = Redact(msg), Redact(stack), Redact(name), Redact(ftype)
	}
	return map[string]any{
		"id":            g.Key,
		"test_name":     headRunes(name, TestNameCap),
		"failure_type":  headRunes(ftype, EnvFieldCap),
		"error_message": headRunes(msg, ExcerptErrorChars),
		"stack_head":    headRunes(stack, ExcerptStackChars),
	}
}

func sizeOf(v any) int {
	b, _ := json.Marshal(v)
	return len(b)
}

// chunkPairs packs pairs greedily under MaxQuestionsPerRequest and ChunkCharBudget. A pair that
// cannot fit alone is skipped.
func chunkPairs(pairs []semPair, redact bool) ([]semChunk, int) {
	questionSize := sizeOf(sameCauseQuestion(999, 999)) + 16
	excerptSize := map[string]int{}
	exSize := func(g *FailureGroup) int {
		if s, ok := excerptSize[g.Key]; ok {
			return s
		}
		s := sizeOf(excerpt(g, redact)) + 2
		excerptSize[g.Key] = s
		return s
	}
	var chunks []semChunk
	skipped := 0
	cur := semChunk{local: map[string]int{}}
	curSize := 32
	flush := func() {
		if len(cur.pairs) > 0 {
			chunks = append(chunks, cur)
		}
		cur = semChunk{local: map[string]int{}}
		curSize = 32
	}
	for _, p := range pairs {
		if exSize(p.a)+exSize(p.b)+questionSize+32 > ChunkCharBudget {
			skipped++
			continue
		}
		add := questionSize
		if _, ok := cur.local[p.a.Key]; !ok {
			add += exSize(p.a)
		}
		if _, ok := cur.local[p.b.Key]; !ok {
			add += exSize(p.b)
		}
		if len(cur.pairs) >= MaxQuestionsPerRequest || curSize+add > ChunkCharBudget {
			flush()
			add = questionSize + exSize(p.a) + exSize(p.b)
		}
		for _, g := range []*FailureGroup{p.a, p.b} {
			if _, ok := cur.local[g.Key]; !ok {
				cur.local[g.Key] = len(cur.reps)
				cur.reps = append(cur.reps, g)
			}
		}
		cur.pairs = append(cur.pairs, p)
		curSize += add
	}
	flush()
	return chunks, skipped
}

func buildRequest(ch semChunk, deps SemanticDeps) typesafe.Request {
	failures := make([]map[string]any, 0, len(ch.reps))
	for _, g := range ch.reps {
		failures = append(failures, excerpt(g, deps.Redact))
	}
	qs := make(map[string]typesafe.Question, len(ch.pairs))
	for _, p := range ch.pairs {
		qs[fmt.Sprintf("pair_%d", p.k)] = sameCauseQuestion(ch.local[p.a.Key], ch.local[p.b.Key])
	}
	return typesafe.Request{State: map[string]any{"failures": failures}, Model: deps.Model, Questions: qs}
}

// askChunk sends one chunk. On an Oversized reply it splits once into halves; a single-pair
// half that is still oversized is skipped; a multi-pair half that is still oversized is an error.
// Returns answers keyed by pair index, tokens, model, request count, skipped count, error.
func askChunk(ctx context.Context, deps SemanticDeps, ch semChunk, cancelled func() bool) (map[int]float64, int, string, int, int, error) {
	resp, err := deps.Client.Evaluate(ctx, buildRequest(ch, deps))
	if err == nil {
		return collect(ch, resp), resp.Usage.InputTokens, resp.Model, 1, 0, nil
	}
	var te *typesafe.Error
	if !errors.As(err, &te) || !te.Oversized {
		return nil, 0, "", 1, 0, err
	}
	if len(ch.pairs) == 1 {
		return map[int]float64{}, 0, "", 1, 1, nil
	}
	answers := map[int]float64{}
	tokens, requests, skipped := 0, 1, 0
	model := ""
	mid := len(ch.pairs) / 2
	for _, half := range [][]semPair{ch.pairs[:mid], ch.pairs[mid:]} {
		if ctx.Err() != nil || cancelled() {
			return answers, tokens, model, requests, skipped, nil // caller re-checks ctx/cancelled first
		}
		hc := rechunk(half)
		hr, herr := deps.Client.Evaluate(ctx, buildRequest(hc, deps))
		requests++
		if herr != nil {
			var he *typesafe.Error
			if errors.As(herr, &he) && he.Oversized && len(hc.pairs) == 1 {
				skipped++
				continue
			}
			return nil, tokens, model, requests, skipped, herr
		}
		tokens += hr.Usage.InputTokens
		model = hr.Model
		for k, v := range collect(hc, hr) {
			answers[k] = v
		}
	}
	return answers, tokens, model, requests, skipped, nil
}

func rechunk(pairs []semPair) semChunk {
	c := semChunk{local: map[string]int{}}
	for _, p := range pairs {
		for _, g := range []*FailureGroup{p.a, p.b} {
			if _, ok := c.local[g.Key]; !ok {
				c.local[g.Key] = len(c.reps)
				c.reps = append(c.reps, g)
			}
		}
		c.pairs = append(c.pairs, p)
	}
	return c
}

func collect(ch semChunk, resp *typesafe.Response) map[int]float64 {
	out := make(map[int]float64, len(ch.pairs))
	for _, p := range ch.pairs {
		if a, ok := resp.Answers[fmt.Sprintf("pair_%d", p.k)]; ok {
			out[p.k] = a.Noul
		}
	}
	return out
}

type semEdge struct {
	a, b string
	p    float64
}

// clusterCompleteLinkage merges groups over passed edges; two clusters merge only if every
// cross pair was asked and passed, and the merged cluster stays within MaxGroupsPerCluster.
func clusterCompleteLinkage(groups []*FailureGroup, asked map[[2]string]float64) []*FailureGroup {
	byKey := make(map[string]*FailureGroup, len(groups))
	for _, g := range groups {
		byKey[g.Key] = g
	}
	var edges []semEdge
	for k, p := range asked {
		if p >= SameCauseMin {
			edges = append(edges, semEdge{a: k[0], b: k[1], p: p})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].p != edges[j].p {
			return edges[i].p > edges[j].p
		}
		if edges[i].a != edges[j].a {
			return edges[i].a < edges[j].a
		}
		return edges[i].b < edges[j].b
	})

	cluster := make(map[string][]string, len(groups)) // cluster id (a member key) -> member keys
	owner := make(map[string]string, len(groups))     // group key -> cluster id
	for _, g := range groups {
		cluster[g.Key] = []string{g.Key}
		owner[g.Key] = g.Key
	}
	passed := func(x, y string) bool {
		p, ok := asked[pairKey(x, y)]
		return ok && p >= SameCauseMin
	}
	for _, e := range edges {
		ca, cb := owner[e.a], owner[e.b]
		if ca == cb {
			continue
		}
		if len(cluster[ca])+len(cluster[cb]) > MaxGroupsPerCluster {
			continue
		}
		ok := true
		for _, x := range cluster[ca] {
			for _, y := range cluster[cb] {
				if !passed(x, y) {
					ok = false
					break
				}
			}
			if !ok {
				break
			}
		}
		if !ok {
			continue
		}
		cluster[ca] = append(cluster[ca], cluster[cb]...)
		for _, y := range cluster[cb] {
			owner[y] = ca
		}
		delete(cluster, cb)
	}

	out := make([]*FailureGroup, 0, len(cluster))
	for _, keys := range cluster {
		if len(keys) == 1 {
			out = append(out, byKey[keys[0]])
			continue
		}
		merged := &FailureGroup{SemanticMembers: map[string]float64{}}
		originOf := map[string]string{} // result id -> original group key
		for _, k := range keys {
			for _, m := range byKey[k].Members {
				merged.Members = append(merged.Members, m)
				originOf[m.ID] = k
			}
		}
		sort.SliceStable(merged.Members, func(i, j int) bool { return earlier(merged.Members[i], merged.Members[j]) })
		merged.Representative = merged.Members[0]
		repOrigin := originOf[merged.Representative.ID]
		merged.Key = repOrigin
		for _, m := range merged.Members {
			if o := originOf[m.ID]; o != repOrigin {
				merged.SemanticMembers[m.ID] = asked[pairKey(repOrigin, o)]
			}
		}
		out = append(out, merged)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if len(out[i].Members) != len(out[j].Members) {
			return len(out[i].Members) > len(out[j].Members)
		}
		return out[i].Key < out[j].Key
	})
	return out
}
