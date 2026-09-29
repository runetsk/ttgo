package failureanalysis

import (
	"encoding/json"
	"errors"
)

// InjectionSummary is the stored summary of a decision whose evidence the injection guard
// flagged (spec Wave 3 §1.4): the decision stands, and nothing of the failure went to the LLM.
const InjectionSummary = "AI narrative unavailable: possible prompt injection — review the raw failure"

// ErrInjectionOverrideRequired: Explain was asked for a flagged decision without
// override_injection=true. Its text is the 409 body's error.
var ErrInjectionOverrideRequired = errors.New("possible prompt injection: confirm to send this failure to the LLM")

// KnownDefectSignal is the linked defect TypeSafe matched to the failure: the redacted, capped
// key that was in the state (never an option id, never the raw key) and the choice's confidence.
type KnownDefectSignal struct {
	Key        string  `json:"key"`
	Confidence float64 `json:"confidence"`
}

// Signals are TypeSafe's answers to the companion questions of one verdict request, stored as
// run_result_analyses.signals. An answer field is set only when its question was asked and
// answered. CheckedBlocks, BlockHashes and EvidenceHash record which evidence blocks the
// injection question covered, as sent (spec R8; see checked.go); every LLM call that follows
// the decision sees only those blocks. MembersChecked = related_failures ∈ CheckedBlocks.
type Signals struct {
	Injection      *float64           `json:"injection,omitempty"`
	FlakyHistory   *float64           `json:"flaky_history,omitempty"`
	Recurring      *float64           `json:"recurring,omitempty"`
	OutsideApp     *float64           `json:"outside_app,omitempty"`
	KnownDefect    *KnownDefectSignal `json:"known_defect,omitempty"`
	MembersChecked bool               `json:"members_checked,omitempty"`
	CheckedBlocks  []string           `json:"checked_blocks,omitempty"`
	EvidenceHash   string             `json:"evidence_hash,omitempty"`
	BlockHashes    map[string]string  `json:"block_hashes,omitempty"`
}

// InjectionFlagged reports whether the guard trips: P(injection) at or above InjectionMin.
func (s Signals) InjectionFlagged() bool {
	return s.Injection != nil && *s.Injection >= InjectionMin
}

// Guarded reports whether TypeSafe's injection question was answered for this decision, so
// CheckedBlocks is authoritative. Unguarded: rows before policy v7 (and test fakes).
func (s Signals) Guarded() bool { return s.Injection != nil }

func (s Signals) isZero() bool {
	return s.Injection == nil && s.FlakyHistory == nil && s.Recurring == nil && s.OutsideApp == nil &&
		s.KnownDefect == nil && !s.MembersChecked && len(s.CheckedBlocks) == 0 && s.EvidenceHash == "" &&
		len(s.BlockHashes) == 0
}

// SignalsJSON is the stored form: "" when nothing was asked (rows before Wave 3, LLM routes).
func SignalsJSON(s Signals) string {
	if s.isZero() {
		return ""
	}
	b, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(b)
}

// ParseSignals reads a stored value; "" or unreadable JSON is no signals.
func ParseSignals(raw string) Signals {
	var s Signals
	if raw == "" {
		return s
	}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return Signals{}
	}
	return s
}
