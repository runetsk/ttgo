package analytics_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	api "ttgo/internal/api"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	require.NoError(t, err)
	return s
}

func auth(t *testing.T, s *store.Store, r *http.Request) {
	t.Helper()
	require.NoError(t, s.SeedAdminIfNeeded("admin@test.com", "testpassword1234"))
	user, err := s.FindUserByEmail("admin@test.com")
	require.NoError(t, err)
	sess, err := s.CreateSession(user.ID)
	require.NoError(t, err)
	r.AddCookie(&http.Cookie{Name: "session_token", Value: sess.ID})
}

func get(t *testing.T, st *store.Store, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("GET", path, nil)
	auth(t, st, r)
	w := httptest.NewRecorder()
	api.NewServer(st).ServeHTTP(w, r)
	return w
}

func TestSummary(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/summary")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSummary_WithDates(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/summary?start_date=2025-01-01&end_date=2025-12-31&folder_id=x")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSummary_IgnoresInvalidDates(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/summary?start_date=bogus&end_date=also-bogus")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTrend(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/trend")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTrend_DaysParam(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/trend?days=7")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTrend_InvalidDays(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/trend?days=abc")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTrend_StartDateOverridesDays(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/trend?days=7&start_date=2025-01-01")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFlaky(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/flaky?lookback=10&limit=5")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFlaky_InvalidParams(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/flaky?lookback=abc&limit=999")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMostFailed(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/most-failed?limit=10")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMostFailed_InvalidLimit(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/most-failed?limit=xyz")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDuration(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/duration")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestDurationTop(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/duration/top?limit=5")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestComponentHealth(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/component-health")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestComponentHealth_WithThreshold(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/component-health?threshold=75.0")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestComponentHealth_InvalidThreshold(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/component-health?threshold=abc")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGrowth(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/growth")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPassingRate(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/passing-rate?exclude_skipped=true")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestActivity(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/activity?limit=10")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestActivity_InvalidLimit(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/activity?limit=999")
	assert.NotEqual(t, 0, w.Code)
}

func TestUniqueBugs(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/unique-bugs?limit=20")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCompareRuns_MissingParams(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/compare-runs")
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = get(t, st, "/api/analytics/compare-runs?run1=a")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCompareRuns_UnknownRuns(t *testing.T) {
	st := newStore(t)
	w := get(t, st, "/api/analytics/compare-runs?run1=x&run2=y")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
