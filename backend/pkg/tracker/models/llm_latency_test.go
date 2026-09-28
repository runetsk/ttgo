package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateLLMLatency(t *testing.T) {
	require.NoError(t, ValidateLLMLatency(45, 0))
	require.NoError(t, ValidateLLMLatency(10, 3))
	require.NoError(t, ValidateLLMLatency(120, 119))
	require.EqualError(t, ValidateLLMLatency(9, 0), "llm_call_timeout_seconds must be between 10 and 120")
	require.EqualError(t, ValidateLLMLatency(121, 0), "llm_call_timeout_seconds must be between 10 and 120")
	require.EqualError(t, ValidateLLMLatency(45, 2), "hedge_after_seconds must be 0 (off) or at least 3")
	require.EqualError(t, ValidateLLMLatency(45, -1), "hedge_after_seconds must be 0 (off) or at least 3")
	require.EqualError(t, ValidateLLMLatency(45, 45), "hedge_after_seconds must be less than llm_call_timeout_seconds (45)")
}
