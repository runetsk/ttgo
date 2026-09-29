package failureanalysis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSignals_JSONRoundTripAndShape(t *testing.T) {
	require.Equal(t, "", SignalsJSON(Signals{}), "nothing asked: stored empty")
	require.Equal(t, Signals{}, ParseSignals(""))
	require.Equal(t, Signals{}, ParseSignals("{not json"))

	inj, flaky, rec, out := 0.03, 0.91, 0.97, 0.0
	s := Signals{Injection: &inj, FlakyHistory: &flaky, Recurring: &rec, OutsideApp: &out,
		KnownDefect: &KnownDefectSignal{Key: "PAY-42", Confidence: 0.88}, MembersChecked: true,
		CheckedBlocks: []string{BlockTest, BlockRelatedFailures}, EvidenceHash: "e",
		BlockHashes: map[string]string{BlockTest: "a", BlockRelatedFailures: "b"}}
	raw := SignalsJSON(s)
	require.JSONEq(t, `{"injection":0.03,"flaky_history":0.91,"recurring":0.97,"outside_app":0,
		"known_defect":{"key":"PAY-42","confidence":0.88},"members_checked":true,
		"checked_blocks":["test","related_failures"],"evidence_hash":"e",
		"block_hashes":{"test":"a","related_failures":"b"}}`, raw, "an asked question answered 0 is kept")
	require.Equal(t, s, ParseSignals(raw))
	require.JSONEq(t, `{"injection":0.03}`, SignalsJSON(Signals{Injection: &inj}), "keys only for what was asked")
}

func TestSignals_InjectionFlaggedAtTheThreshold(t *testing.T) {
	at, below := InjectionMin, InjectionMin-0.001
	require.True(t, Signals{Injection: &at}.InjectionFlagged(), "inclusive")
	require.False(t, Signals{Injection: &below}.InjectionFlagged())
	require.False(t, Signals{}.InjectionFlagged(), "not asked is not flagged")
	require.True(t, Signals{Injection: &below}.Guarded(), "answered at all: the evidence was checked")
	require.False(t, Signals{}.Guarded())
	require.Equal(t, 0.80, InjectionMin)
	require.Equal(t, 0.80, SignalMin)
	require.Equal(t, 20, KnownDefectMaxOptions)
	require.Equal(t, 3, FlakyHistoryMinOutcomes)
	require.Equal(t, "AI narrative unavailable: possible prompt injection — review the raw failure", InjectionSummary)
	require.Equal(t, "possible prompt injection: confirm to send this failure to the LLM", ErrInjectionOverrideRequired.Error())
}
