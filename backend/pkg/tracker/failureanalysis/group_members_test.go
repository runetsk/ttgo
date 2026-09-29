package failureanalysis

import (
	"context"
	"strings"
	"testing"
	"ttgo/pkg/tracker/llm"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

func TestBuildEvidence_GroupMembersRedactedCappedDistinct(t *testing.T) {
	const secret = "abcdefghijklmnopqrstuvwxyz0123456789"
	in := baseContext() // redaction on; the representative's error is "expected 401, got 500"
	in.GroupMembers = []string{
		"expected 401, got 500", // the representative's own error: skipped
		"   ",                   // blank: skipped
		"timeout after 30000 ms waiting for #login",
		"timeout after 45000 ms waiting for #login", // same line once ids are normalized: skipped
		"auth failed: Bearer " + secret,
		strings.Repeat("x", GroupMemberMsgCap+100),
		"line one\nline two",
		"sixth distinct",
		"seventh distinct", // past GroupMembersMax
	}
	ev := BuildEvidence(in)
	require.Equal(t, []string{
		"timeout after 30000 ms waiting for #login",
		"auth failed: Bearer <REDACTED_TOKEN>",
		strings.Repeat("x", GroupMemberMsgCap),
		"line one line two",
		"sixth distinct",
	}, ev.GroupMembers)
	require.Equal(t, "auth failed: Bearer "+secret, in.GroupMembers[4], "the caller's slice is never rewritten")

	in.RedactionEnabled = false
	require.Contains(t, BuildEvidence(in).GroupMembers[1], secret, "redaction off sends the line as recorded")
}

func TestBuildEvidence_SignatureGroupAddsNothing(t *testing.T) {
	in := baseContext()
	in.Result.ErrorMessage = "order 12345 failed at 2026-09-01T10:00:00Z"
	in.GroupMembers = []string{"order 67890 failed at 2026-09-02T11:00:00Z", "order 13579 failed at 2026-09-03T12:00:00Z"}
	require.Empty(t, BuildEvidence(in).GroupMembers, "identical normalized errors add no block")
	require.Nil(t, BuildEvidence(baseContext()).GroupMembers)
}

func TestBuildPrompt_RelatedFailuresBlock(t *testing.T) {
	in := PromptInput{Template: DefaultPromptTemplate, TestName: "Login", ErrorMessage: "expected 401, got 500"}
	plain, meta, err := BuildPrompt(in)
	require.NoError(t, err)
	want, err := render(DefaultPromptTemplate, in)
	require.NoError(t, err)
	require.Equal(t, want, plain, "without members the prompt is exactly the rendered template")
	require.Empty(t, meta.TruncationPrefix)

	in.GroupMembers = []string{"socket hang up", strings.Repeat("y", GroupMemberMsgCap+50)}
	got, meta, err := BuildPrompt(in)
	require.NoError(t, err)
	require.Empty(t, meta.TruncationPrefix)
	require.Equal(t, plain+groupBlock([]string{"socket hang up", strings.Repeat("y", GroupMemberMsgCap)}), got)
	require.Contains(t, got, "### Related failures in this group")
	require.Contains(t, got, "Explain the cause they share")
	require.Contains(t, got, "- <<<DATA socket hang up DATA>>>")
	require.Len(t, in.GroupMembers[1], GroupMemberMsgCap+50, "BuildPrompt never rewrites the caller's slice")
}

func TestBuildPrompt_RelatedFailuresAreDroppedFirst(t *testing.T) {
	base := PromptInput{Template: DefaultPromptTemplate, TestName: "x", ErrorMessage: "x", FailureType: "x", LogText: "keep this log"}
	small, _, err := BuildPrompt(base)
	require.NoError(t, err)
	base.Steps = []PromptStep{{Order: 1, Action: strings.Repeat("a", PromptCharCap-len(small)-60), Expected: "e"}}
	fits, meta, err := BuildPrompt(base)
	require.NoError(t, err)
	require.LessOrEqual(t, len(fits), PromptCharCap)
	require.Empty(t, meta.TruncationPrefix)

	base.GroupMembers = []string{"socket hang up", "connection reset by peer"}
	got, meta, err := BuildPrompt(base)
	require.NoError(t, err)
	require.Equal(t, "[context: no related failures] ", meta.TruncationPrefix)
	require.Equal(t, fits, got, "only the related failures were dropped")
	require.Contains(t, got, "keep this log")

	// Everything overflows: the related failures still go first.
	huge := PromptInput{Template: DefaultPromptTemplate, TestName: "x", ErrorMessage: "x", FailureType: "x",
		StackTrace: strings.Repeat("s", 3000), LogText: strings.Repeat("L", 30000),
		Steps:        []PromptStep{{Order: 1, Action: strings.Repeat("a", 25000), Expected: "e"}},
		GroupMembers: []string{"socket hang up"}}
	got, meta, err = BuildPrompt(huge)
	require.NoError(t, err)
	require.LessOrEqual(t, len(got), PromptCharCap)
	require.True(t, strings.HasPrefix(meta.TruncationPrefix, "[context: no related failures; no logs"), meta.TruncationPrefix)
}

func TestNarrate_ExplainsTheSharedCause(t *testing.T) {
	prov := &recordingProvider{stubProvider: stubProvider{responses: []string{narrJSON}}}
	in := baseContext()
	in.GroupMembers = []string{"socket hang up", "connection reset by peer"}
	decided := decisionResult(flakyDecision())
	decided.NarrativeStatus = models.NarrativeStatusPending
	_, ok := Narrate(context.Background(), AnalyzeDeps{Narrative: prov}, in, decided)
	require.True(t, ok)
	user := prov.reqs[0].Messages[1].Content
	require.Contains(t, user, "### Related failures in this group")
	require.Contains(t, user, "Explain the cause they share")
	require.Contains(t, user, "<<<DATA socket hang up DATA>>>")
	require.Contains(t, user, "<<<DATA connection reset by peer DATA>>>")
	require.NotContains(t, prov.reqs[0].Messages[0].Content, "socket hang up", "member lines are data, never in the system message")
}

func TestDecide_GenerativeDecisionPromptCarriesRelatedFailures(t *testing.T) {
	unsure := flakyDecision()
	unsure.VerdictConfidence = 0.40
	tsDown := &typesafe.Error{Category: typesafe.CategoryNetwork, Message: "dial"}
	routes := map[string]func(p llm.Provider) AnalyzeDeps{
		"generative-only": func(p llm.Provider) AnalyzeDeps { return AnalyzeDeps{Narrative: p} },
		"takeover": func(p llm.Provider) AnalyzeDeps {
			return AnalyzeDeps{Narrative: p, Decider: fixedDecider{d: unsure}, EscalateBelow: 0.90}
		},
		"fallback": func(p llm.Provider) AnalyzeDeps { return AnalyzeDeps{Narrative: p, Decider: fixedDecider{err: tsDown}} },
	}
	for name, deps := range routes {
		prov := &capturingProvider{}
		in := baseContext()
		in.GroupMembers = []string{"socket hang up"}
		res, err := Decide(context.Background(), deps(prov), in)
		require.NoError(t, err, name)
		require.Equal(t, models.AnalysisEngineGenerative, res.Engine, name)
		require.Contains(t, prov.lastPrompt, "### Related failures in this group", name)
		require.Contains(t, prov.lastPrompt, "<<<DATA socket hang up DATA>>>", name)
	}
}
