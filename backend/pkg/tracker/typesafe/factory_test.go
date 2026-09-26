package typesafe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientFactory_SharesItsLimiter(t *testing.T) {
	lim := &countingLimiter{}
	f := NewClientFactory(lim)
	require.Same(t, lim, f.New("k1", Options{}).limiter)
	require.Same(t, lim, f.New("k2", Options{}).limiter, "every client draws from the one limiter")

	own := &countingLimiter{}
	require.Same(t, own, f.New("k3", Options{Limiter: own}).limiter, "an explicit limiter wins")

	var none *ClientFactory
	require.Nil(t, none.New("k4", Options{}).limiter, "a nil factory builds unlimited clients")
}
