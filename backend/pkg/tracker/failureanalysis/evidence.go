package failureanalysis

import (
	"encoding/json"
	"log/slog"
	"strings"
	"unicode/utf8"
)

// Per-field caps (spec §6). Their sum is below StateCharCap by construction, which is what
// makes StateCharCap a guarantee rather than a target; TestRenderState_BoundedByConstruction
// asserts the arithmetic.
const (
	TestNameCap       = 200
	CategoriesCap     = 200
	EnvFieldCap       = 100 // environment, browser, os, app_version, failure_type
	MaxSteps          = 30
	StepTextCap       = 300 // action and expected, each
	SimilarLabelCap   = 50  // status and human label
	RollupCap         = 300
	MaxLinkedDefects  = 10
	DefectKeyCap      = 50
	DefectStatusCap   = 50
	DefectSummaryCap  = 300
	StateCharCap      = 40000 // hard bound on the JSON state; the ladder below targets it
	HardCapStackChars = 1000  // hard-cap fallback if the ladder is exhausted
	HardCapErrorChars = 1000
)

// Evidence is the redacted, capped material both engines see.
type Evidence struct {
	TestName, Categories, Env, Browser, OS, AppVersion string
	Steps                                              []PromptStep
	FailureType                                        string
	ErrorMessage                                       string // head ErrorMessageHeadCap
	StackTrace                                         string // head StackTraceHeadCap
	LogText                                            string // tail LogTextTailCap
	SimilarFailures                                    []SimilarFailure
	SimilarFailuresRollup                              string
	LinkedDefects                                      []LinkedDefect
	LinkedRequirements                                 []LinkedRequirement
	GroupMembers                                       []string // other members' error lines, redacted and capped; LLM prompts only
	StateCap                                           int      // bound on the rendered JSON state; 0 = StateCharCap
}

// Budget is an engine's allowance for the three large text fields (runes) and
// for the rendered state. Every other cap is shared.
type Budget struct {
	ErrorHead, StackHead, LogTail int
	StateChars                    int // 0 = StateCharCap
}

// TypeSafe budget (spec §6 revised 2026-09-22): Jev is asked to read the failure
// as recorded, its window is 32k tokens of state, and a filled window costs a
// tenth of a cent, so it gets the whole log tail up to 48,000 characters and
// 16,000 characters each of error and stack inside a 64,000-character state
// (up to about 32k tokens: logs full of ids and timestamps tokenize near two
// characters per token). The ladder still trims a larger state, and the
// decider retries an oversized rejection once at half the bound. The LLM
// prompt keeps its own, smaller caps.
const (
	TypeSafeErrorHeadCap = 16000
	TypeSafeStackHeadCap = 16000
	TypeSafeLogTailCap   = 48000
	TypeSafeStateCharCap = 64000
)

// GenerativeBudget is the allowance the LLM prompt has always had.
func GenerativeBudget() Budget {
	return Budget{ErrorHead: ErrorMessageHeadCap, StackHead: StackTraceHeadCap, LogTail: LogTextTailCap}
}

// TypeSafeBudget is the allowance for the Jev state.
func TypeSafeBudget() Budget {
	return Budget{ErrorHead: TypeSafeErrorHeadCap, StackHead: TypeSafeStackHeadCap, LogTail: TypeSafeLogTailCap, StateChars: TypeSafeStateCharCap}
}

// headRunes / tailRunes are rune-safe: they never cut inside a multi-byte sequence.
func headRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	i, count := 0, 0
	for i < len(s) && count < n {
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
		count++
	}
	return s[:i]
}

func tailRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	i, count := len(s), 0
	for i > 0 && count < n {
		_, size := utf8.DecodeLastRuneInString(s[:i])
		i -= size
		count++
	}
	return s[i:]
}

// BuildEvidence applies redaction (when enabled) and every per-field cap. It never mutates
// the caller's AnalyzeContext or RunResult.
func BuildEvidence(in AnalyzeContext) Evidence {
	return BuildEvidenceWithBudget(in, GenerativeBudget())
}

// BuildEvidenceWithBudget builds the evidence with an engine's allowance for the
// error message, stack head, log tail and state bound.
func BuildEvidenceWithBudget(in AnalyzeContext, b Budget) Evidence {
	r := in.Result
	red := func(s string) string {
		if in.RedactionEnabled {
			return Redact(s)
		}
		return s
	}
	ev := Evidence{
		// Every field that leaves the server is redacted, not only the failure text: test
		// names, environment fields, steps and linked records can carry secrets and emails too.
		TestName:              headRunes(red(r.TestNameSnapshot), TestNameCap),
		Categories:            headRunes(red(in.Categories), CategoriesCap),
		Env:                   headRunes(red(in.Env), EnvFieldCap),
		Browser:               headRunes(red(in.Browser), EnvFieldCap),
		OS:                    headRunes(red(in.OS), EnvFieldCap),
		AppVersion:            headRunes(red(in.AppVersion), EnvFieldCap),
		FailureType:           headRunes(red(r.FailureType), EnvFieldCap),
		ErrorMessage:          headRunes(red(r.ErrorMessage), b.ErrorHead),
		StackTrace:            headRunes(red(r.StackTrace), b.StackHead),
		LogText:               tailRunes(red(r.LogText), b.LogTail),
		StateCap:              b.StateChars,
		SimilarFailuresRollup: headRunes(in.SimilarFailuresRollup, RollupCap),
	}
	steps := in.Steps
	if len(steps) > MaxSteps {
		steps = steps[:MaxSteps]
	}
	ev.Steps = make([]PromptStep, len(steps))
	for i, s := range steps {
		ev.Steps[i] = PromptStep{Order: s.Order, Action: headRunes(red(s.Action), StepTextCap), Expected: headRunes(red(s.Expected), StepTextCap)}
	}
	sims := in.SimilarFailures
	if len(sims) > SimilarFailuresMax {
		sims = sims[:SimilarFailuresMax]
	}
	ev.SimilarFailures = make([]SimilarFailure, len(sims)) // fresh slice: never write into the caller's array
	for i, s := range sims {
		ev.SimilarFailures[i] = SimilarFailure{RunStartedAt: s.RunStartedAt,
			Status: headRunes(s.Status, SimilarLabelCap), ErrorMessage: oneline(headRunes(red(s.ErrorMessage), SimilarMsgCap)),
			DefectType: headRunes(s.DefectType, SimilarLabelCap), DefectKey: headRunes(red(s.DefectKey), DefectKeyCap)}
	}
	defs := in.LinkedDefects
	if len(defs) > MaxLinkedDefects {
		defs = defs[:MaxLinkedDefects]
	}
	ev.LinkedDefects = make([]LinkedDefect, len(defs))
	for i, d := range defs {
		ev.LinkedDefects[i] = LinkedDefect{Key: headRunes(red(d.Key), DefectKeyCap), Status: headRunes(d.Status, DefectStatusCap), Summary: headRunes(red(d.Summary), DefectSummaryCap)}
	}
	ev.LinkedRequirements = make([]LinkedRequirement, len(in.LinkedRequirements))
	for i, lr := range in.LinkedRequirements {
		ev.LinkedRequirements[i] = LinkedRequirement{Key: red(lr.Key), Title: red(lr.Title)}
	}
	ev.GroupMembers = groupMemberLines(in, red)
	return ev
}

// groupMemberLines picks up to GroupMembersMax error lines from the group's other members
// (spec §A4). A blank line, the representative's own error and repeats are skipped, compared
// after the signature normalizer, so a signature group (identical normalized errors) adds
// nothing. Each line is redacted when redaction is on, capped at GroupMemberMsgCap runes and
// flattened to one line. nil when nothing is left.
func groupMemberLines(in AnalyzeContext, red func(string) string) []string {
	if len(in.GroupMembers) == 0 {
		return nil
	}
	seen := map[string]bool{normalize(oneline(in.Result.ErrorMessage)): true}
	var out []string
	for _, m := range in.GroupMembers {
		if len(out) == GroupMembersMax {
			break
		}
		if strings.TrimSpace(m) == "" {
			continue
		}
		key := normalize(oneline(m))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, oneline(headRunes(red(m), GroupMemberMsgCap)))
	}
	return out
}

// PromptInput converts evidence into the template's data struct.
func (ev Evidence) PromptInput(template string) PromptInput {
	return PromptInput{
		Template: template, TestName: ev.TestName, Categories: ev.Categories, Env: ev.Env, Browser: ev.Browser,
		OS: ev.OS, AppVersion: ev.AppVersion, Steps: ev.Steps, FailureType: ev.FailureType,
		ErrorMessage: ev.ErrorMessage, StackTrace: ev.StackTrace, LogText: ev.LogText,
		SimilarFailures: ev.SimilarFailures, SimilarFailuresRollup: ev.SimilarFailuresRollup,
		LinkedDefects: ev.LinkedDefects, LinkedRequirements: ev.LinkedRequirements,
		GroupMembers: ev.GroupMembers,
	}
}

// dropNext removes the next optional block in the shared drop order and names it.
func dropNext(ev *Evidence) (string, bool) {
	switch {
	case ev.LogText != "":
		ev.LogText = ""
		return "no logs", true
	case len(ev.SimilarFailures) > 0:
		ev.SimilarFailures, ev.SimilarFailuresRollup = nil, ""
		return "no similar failures", true
	case len(ev.Steps) > 0:
		ev.Steps = nil
		return "no steps", true
	case len(ev.LinkedDefects) > 0:
		ev.LinkedDefects = nil
		return "no defects", true
	case len(ev.LinkedRequirements) > 0:
		ev.LinkedRequirements = nil
		return "no requirements", true
	}
	return "", false
}

func applyHardCap(ev *Evidence) {
	ev.StackTrace = headRunes(ev.StackTrace, HardCapStackChars)
	ev.ErrorMessage = headRunes(ev.ErrorMessage, HardCapErrorChars)
}

// RenderState builds the JSON state for TypeSafe (spec §6 shape). Linked requirements are
// never included. The drop ladder runs against StateCharCap (unreachable by construction
// with the per-field caps, kept as the safety net); if it is exhausted the hard cap truncates
// stack then error, so the result is always <= StateCharCap.
func RenderState(ev Evidence) (map[string]any, PromptMeta) {
	bound := ev.StateCap
	if bound <= 0 {
		bound = StateCharCap
	}
	var dropped []string
	for {
		state := stateObject(ev)
		b, _ := json.Marshal(state)
		if len(b) <= bound {
			return state, PromptMeta{TruncationPrefix: makePrefix(dropped)}
		}
		name, ok := dropNext(&ev)
		if !ok {
			applyHardCap(&ev)
			state = stateObject(ev)
			b, _ = json.Marshal(state)
			slog.Debug("failure-analysis: state hard-capped", "bytes", len(b))
			return state, PromptMeta{TruncationPrefix: makePrefix(append(dropped, "hard cap"))}
		}
		slog.Debug("failure-analysis: TypeSafe state field dropped", "field", name, "bytes", len(b))
		dropped = append(dropped, name)
	}
}

func stateObject(ev Evidence) map[string]any {
	steps := make([]map[string]any, 0, len(ev.Steps))
	for _, s := range ev.Steps {
		steps = append(steps, map[string]any{"order": s.Order, "action": s.Action, "expected": s.Expected})
	}
	sims := make([]map[string]any, 0, len(ev.SimilarFailures))
	for _, s := range ev.SimilarFailures {
		sims = append(sims, map[string]any{"run_started_at": s.RunStartedAt.UTC().Format("2006-01-02T15:04:05Z"),
			"status": s.Status, "error_message": s.ErrorMessage, "human_defect_type": s.DefectType})
	}
	defects := make([]map[string]any, 0, len(ev.LinkedDefects))
	for _, d := range ev.LinkedDefects {
		defects = append(defects, map[string]any{"key": d.Key, "status": d.Status, "summary": d.Summary})
	}
	return map[string]any{
		"test": map[string]any{"name": ev.TestName, "categories": ev.Categories, "environment": ev.Env,
			"browser": ev.Browser, "os": ev.OS, "app_version": ev.AppVersion, "steps": steps},
		"failure": map[string]any{"failure_type": ev.FailureType, "error_message": ev.ErrorMessage,
			"stack_trace_head": ev.StackTrace, "log_tail": ev.LogText},
		"history":        map[string]any{"note": HistoryNote, "human_label_rollup": ev.SimilarFailuresRollup, "similar_failures": sims},
		"linked_defects": defects,
	}
}
