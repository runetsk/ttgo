package api

import (
	"testing"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

// The automatic job's estimate follows the automatic route: resolverStore's LLM is not approved
// for automatic analysis, so only TypeSafe is priced, per failing group.
func TestAutoAnalysisEstimate_UsesTheAutomaticRoute(t *testing.T) {
	s := resolverStore(t)
	enableTypeSafe(t, s, true)
	a := newAutoAnalysis(s, newAnalyzeDepsResolver(s, typesafe.NewClientFactory(nil)))
	require.NotNil(t, a.Gate)

	failures := []*models.RunResult{
		{ID: "a", FailureType: "assertion", ErrorMessage: "boom"},
		{ID: "b", FailureType: "assertion", ErrorMessage: "boom"},
		{ID: "c", FailureType: "assertion", ErrorMessage: "other"},
	}
	est := a.Estimate(failures)
	require.NotNil(t, est)
	per := float64(failureanalysis.TypeSafeStateCharCap/4) * models.TypeSafeDefaultPricePerMTok / 1e6
	require.InDelta(t, 2*per, *est, 1e-12, "two signature groups")
}
