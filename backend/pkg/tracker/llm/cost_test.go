package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fp(v float64) *float64 { return &v }

func TestEstimateCostUSD(t *testing.T) {
	assert.Nil(t, EstimateCostUSD(1000, 1000, nil, nil), "no prices -> no estimate")

	got := EstimateCostUSD(1_000_000, 2_000_000, fp(2.5), fp(10))
	require.NotNil(t, got)
	assert.InDelta(t, 2.5+20.0, *got, 1e-9)

	got = EstimateCostUSD(500_000, 0, fp(2.0), nil)
	require.NotNil(t, got)
	assert.InDelta(t, 1.0, *got, 1e-9, "a missing side contributes zero")

	got = EstimateCostUSD(0, 0, fp(2.0), fp(2.0))
	require.NotNil(t, got)
	assert.Zero(t, *got)
}
