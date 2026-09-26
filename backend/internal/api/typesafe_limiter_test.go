package api

import (
	"context"
	"errors"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

// refusingLimiter counts every wait and refuses it, so no request ever leaves the process.
type refusingLimiter struct{ calls atomic.Int32 }

func (l *refusingLimiter) Wait(context.Context) error {
	l.calls.Add(1)
	return errors.New("limiter closed for the test")
}

// The failure-analysis resolver and the settings connection test build their TypeSafe clients
// through the Server's one factory, so they draw from one process-wide rate limiter.
func TestTypeSafeClients_ResolverAndConnectionTestShareOneLimiter(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, false)
	lim := &refusingLimiter{}
	srv := NewServer(s, WithTypeSafeClientFactory(typesafe.NewClientFactory(lim)))
	defer srv.Shutdown()

	deps, err := srv.analyzeDepsResolver()(models.RunAnalysisJobTriggerManual)
	require.NoError(t, err)
	require.NotNil(t, deps.Semantic)
	_, err = deps.Semantic.Client.Evaluate(context.Background(), typesafe.Request{Model: "m"})
	var te *typesafe.Error
	require.ErrorAs(t, err, &te)
	require.Equal(t, typesafe.CategoryNetwork, te.Category)
	require.Equal(t, int32(1), lim.calls.Load(), "the resolver's client waits on the shared limiter")

	rr := httptest.NewRecorder()
	srv.aiHandler.TestTypeSafeConnection(rr, httptest.NewRequest("POST", "/api/settings/typesafe/test", nil))
	require.Equal(t, 200, rr.Code)
	require.Contains(t, rr.Body.String(), `"ok":false`)
	require.Equal(t, int32(2), lim.calls.Load(), "the connection test waits on the same limiter")
}
