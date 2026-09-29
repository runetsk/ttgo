package failureanalysis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
)

// Evidence blocks the injection guard reasons about (spec R8). A block is all of the evidence
// the prompt renders in one place; the TypeSafe state never carries requirements, so they are
// never checked.
const (
	BlockTest            = "test"          // name, categories, environment, browser, os, app_version
	BlockSteps           = "steps"         // test steps
	BlockFailureError    = "failure.error" // failure_type and error_message
	BlockFailureStack    = "failure.stack" // stack trace head
	BlockFailureLog      = "failure.log"   // log tail
	BlockHistory         = "history"       // similar failures, their labels and keys, the rollup, recent outcomes
	BlockExamples        = "examples"      // few-shot triage examples
	BlockDefects         = "defects"       // linked defects
	BlockRequirements    = "requirements"  // linked requirements (never in the TypeSafe state)
	BlockRelatedFailures = "related_failures"
)

// checkedBlocks names, in a fixed order, the blocks the sent state carried exactly as built:
// not dropped by the ladder, not partly dropped (examples go one at a time), not trimmed by the
// hard cap. A block that is empty in both counts: its hash still catches later additions.
func checkedBlocks(built, sent Evidence) []string {
	var out []string
	add := func(name string, same bool) {
		if same {
			out = append(out, name)
		}
	}
	add(BlockTest, true) // the ladder never touches it
	add(BlockSteps, len(sent.Steps) == len(built.Steps))
	add(BlockFailureError, sent.ErrorMessage == built.ErrorMessage)
	add(BlockFailureStack, sent.StackTrace == built.StackTrace)
	add(BlockFailureLog, sent.LogText == built.LogText)
	add(BlockHistory, len(sent.SimilarFailures) == len(built.SimilarFailures) &&
		sent.SimilarFailuresRollup == built.SimilarFailuresRollup && sent.RecentOutcomes == built.RecentOutcomes)
	add(BlockExamples, len(sent.Examples) == len(built.Examples))
	add(BlockDefects, len(sent.LinkedDefects) == len(built.LinkedDefects))
	add(BlockRelatedFailures, len(sent.GroupMembers) == len(built.GroupMembers))
	return out
}

// blockHashes is the sha256 (hex) of each block's part of the rendered state, so a hash
// compares exactly what TypeSafe read.
func blockHashes(ev Evidence) map[string]string {
	st := stateObject(ev)
	test := st["test"].(map[string]any)
	failure := st["failure"].(map[string]any)
	parts := map[string]any{
		BlockTest: map[string]any{"name": test["name"], "categories": test["categories"], "environment": test["environment"],
			"browser": test["browser"], "os": test["os"], "app_version": test["app_version"]},
		BlockSteps:           test["steps"],
		BlockFailureError:    map[string]any{"failure_type": failure["failure_type"], "error_message": failure["error_message"]},
		BlockFailureStack:    failure["stack_trace_head"],
		BlockFailureLog:      failure["log_tail"],
		BlockHistory:         st["history"],
		BlockExamples:        st["examples"], // nil when absent
		BlockDefects:         st["linked_defects"],
		BlockRelatedFailures: st["group"], // nil when absent
	}
	out := make(map[string]string, len(parts))
	for name, v := range parts {
		b, _ := json.Marshal(v)
		sum := sha256.Sum256(b)
		out[name] = hex.EncodeToString(sum[:])
	}
	return out
}

// evidenceHash is the sha256 (hex) of the checked blocks' hashes, one "name=hash" line each.
func evidenceHash(blocks []string, hashes map[string]string) string {
	var b strings.Builder
	for _, name := range blocks {
		b.WriteString(name + "=" + hashes[name] + "\n")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// RecordChecked records which blocks the injection question covered: the ones the sent state
// carried as built, with their hashes over the sent (redacted, capped) text.
func (s *Signals) RecordChecked(built, sent Evidence) {
	s.CheckedBlocks = checkedBlocks(built, sent)
	all := blockHashes(sent)
	s.BlockHashes = make(map[string]string, len(s.CheckedBlocks))
	for _, name := range s.CheckedBlocks {
		s.BlockHashes[name] = all[name]
	}
	s.EvidenceHash = evidenceHash(s.CheckedBlocks, s.BlockHashes)
	s.MembersChecked = slices.Contains(s.CheckedBlocks, BlockRelatedFailures)
}

// VerifiedAgainst narrows a guarded decision's checked blocks to those whose text in ev (the
// TypeSafe-budget evidence rebuilt later, for Explain) is unchanged since the decision. The
// receiver is not modified; unguarded signals come back as they are.
func (s Signals) VerifiedAgainst(ev Evidence) Signals {
	if !s.Guarded() {
		return s
	}
	cur := blockHashes(ev)
	if evidenceHash(s.CheckedBlocks, cur) == s.EvidenceHash {
		return s
	}
	var kept []string
	for _, name := range s.CheckedBlocks {
		if h, ok := s.BlockHashes[name]; ok && cur[name] == h {
			kept = append(kept, name)
		}
	}
	out := s
	out.CheckedBlocks = kept
	out.MembersChecked = slices.Contains(kept, BlockRelatedFailures)
	return out
}

// OnlyBlocks returns ev with every block not in keep emptied, and NotChecked naming the ones
// that had content (BuildPrompt lists them in the truncation prefix). ev itself is unchanged.
func (ev Evidence) OnlyBlocks(keep []string) Evidence {
	var notChecked []string
	drop := func(name string, present bool, clear func()) {
		if slices.Contains(keep, name) {
			return
		}
		if present {
			notChecked = append(notChecked, name)
		}
		clear()
	}
	drop(BlockTest, ev.TestName != "" || ev.Categories != "" || ev.Env != "" || ev.Browser != "" || ev.OS != "" || ev.AppVersion != "",
		func() { ev.TestName, ev.Categories, ev.Env, ev.Browser, ev.OS, ev.AppVersion = "", "", "", "", "", "" })
	drop(BlockSteps, len(ev.Steps) > 0, func() { ev.Steps = nil })
	drop(BlockFailureError, ev.FailureType != "" || ev.ErrorMessage != "", func() { ev.FailureType, ev.ErrorMessage = "", "" })
	drop(BlockFailureStack, ev.StackTrace != "", func() { ev.StackTrace = "" })
	drop(BlockFailureLog, ev.LogText != "", func() { ev.LogText = "" })
	drop(BlockHistory, len(ev.SimilarFailures) > 0 || ev.SimilarFailuresRollup != "" || ev.RecentOutcomes != "",
		func() { ev.SimilarFailures, ev.SimilarFailuresRollup, ev.RecentOutcomes = nil, "", "" })
	drop(BlockExamples, len(ev.Examples) > 0, func() { ev.Examples = nil })
	drop(BlockDefects, len(ev.LinkedDefects) > 0, func() { ev.LinkedDefects = nil })
	drop(BlockRequirements, len(ev.LinkedRequirements) > 0, func() { ev.LinkedRequirements = nil })
	drop(BlockRelatedFailures, len(ev.GroupMembers) > 0, func() { ev.GroupMembers = nil })
	ev.NotChecked = notChecked
	return ev
}
