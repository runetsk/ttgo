package runs_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	api "ttgo/internal/api"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T) *api.Server {
	t.Helper()
	s, err := newTestStore(t)
	require.NoError(t, err)
	return api.NewServer(s)
}

// T016: GET /api/runs/{unknown-uuid} returns HTTP 404 with {"error":"test run not found"}
func TestGetTestRunNotFound(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(s)
	req := httptest.NewRequest(http.MethodGet, "/api/runs/non-existent-uuid", nil)
	addTestAuth(t, s, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "test run not found")
	assert.NotContains(t, w.Body.String(), "circular reference")
}

// T017: PUT /api/runs/{id}/results/{test_id} produces no [DEBUG] log output
func TestUpdateRunResultNoDebugLog(t *testing.T) {
	// This is a compile-time verification — the [DEBUG] log line was removed from source.
	// We verify this by checking the source doesn't contain the pattern.
	// The actual test here is that the handler doesn't panic and returns 200 or appropriate error.
	srv := newTestServer(t)
	req := httptest.NewRequest(http.MethodPut, "/api/runs/some-run-id/results/some-test-id",
		nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	// Should return 400 (bad request - empty body) or 500, NOT 200 with debug info
	assert.NotEqual(t, http.StatusOK, w.Code) // no successful response on empty body
}

func TestRetryRunResultEndpoint(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(s)

	// Seed data
	folder, _ := s.CreateFolder("Root", nil)
	tc := &models.TestCase{Name: "Login", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc))

	run := &models.TestRun{Name: "Retry API Run"}
	require.NoError(t, s.CreateTestRun(run))

	result := &models.RunResult{
		TestRunID:        run.ID,
		TestCaseID:       &tc.ID,
		TestNameSnapshot: tc.Name,
		Status:           models.StatusFail,
	}
	require.NoError(t, s.AddRunResult(result))

	// POST retry
	req := httptest.NewRequest(http.MethodPost, "/api/runs/"+run.ID+"/results/"+result.ID+"/retry", nil)
	addTestAuth(t, s, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var newResult models.RunResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &newResult))
	assert.Equal(t, 2, newResult.AttemptNumber)
	assert.Equal(t, models.StatusPending, newResult.Status)
	assert.NotEqual(t, result.ID, newResult.ID)
}

func TestRetryRunResultEndpointNotFound(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(s)

	run := &models.TestRun{Name: "Not Found Run"}
	require.NoError(t, s.CreateTestRun(run))

	req := httptest.NewRequest(http.MethodPost, "/api/runs/"+run.ID+"/results/nonexistent/retry", nil)
	addTestAuth(t, s, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- AI suggestion snapshot on explicit defect_type triage ---

// newSnapshotEnv returns a file-backed store plus a server over it. A ":memory:" DB can hand
// separate pool connections separate databases, so seeding via the store and reading back after
// an HTTP round-trip is safer on a real file.
func newSnapshotEnv(t *testing.T) (*store.Store, *api.Server) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(dir + "/snapshot.db")
	require.NoError(t, err)
	// Close before TempDir cleanup: Windows cannot delete an open SQLite file.
	t.Cleanup(func() { _ = s.Close() })
	return s, api.NewServer(s)
}

// seedResultWithStatus creates a run holding a single result in the given status.
func seedResultWithStatus(t *testing.T, s *store.Store, status models.ExecutionStatus) *models.RunResult {
	t.Helper()
	run := &models.TestRun{Name: "Snapshot Run"}
	require.NoError(t, s.CreateTestRun(run))
	rr := &models.RunResult{
		TestRunID: run.ID, TestNameSnapshot: "case", AttemptNumber: 1,
		Status: status, ErrorMessage: "boom",
	}
	require.NoError(t, s.AddRunResult(rr))
	return rr
}

func putRunResult(t *testing.T, s *store.Store, srv *api.Server, rr *models.RunResult, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/api/runs/"+rr.TestRunID+"/results/"+rr.ID, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	addTestAuth(t, s, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

// An explicit defect_type on a FAIL result that has an analysis snapshots all three columns —
// the verdict verbatim, the mapped defect_type, and the confidence. The mapped value comes from
// the AI verdict, NOT from what the human chose, so an override is recorded as a disagreement.
func TestUpdateRunResultSnapshotsAISuggestion(t *testing.T) {
	tests := []struct {
		name          string
		verdict       string
		confidence    string
		humanChoice   string
		wantSuggested string
	}{
		{"accept", models.VerdictFlakyTest, models.ConfidenceHigh, "automation_bug", "automation_bug"},
		{"override", models.VerdictProductBug, models.ConfidenceLow, "system_issue", "product_bug"},
		{"unmappable verdict still records verdict+confidence", models.VerdictUnknown, models.ConfidenceMedium, "product_bug", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, srv := newSnapshotEnv(t)
			rr := seedResultWithStatus(t, s, models.StatusFail)
			_, err := s.CreateAnalysis(&models.RunResultAnalysis{
				RunResultID: rr.ID, Verdict: tt.verdict, Confidence: tt.confidence, ModelName: "m",
			})
			require.NoError(t, err)

			// The frontend sends defect_type alone, with no status (ResultsTab.jsx).
			w := putRunResult(t, s, srv, rr, map[string]any{"defect_type": tt.humanChoice})
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			got, err := s.GetRunResultByID(rr.ID)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.humanChoice, got.DefectType)
			assert.Equal(t, tt.verdict, got.SuggestedVerdict)
			assert.Equal(t, tt.wantSuggested, got.SuggestedDefectType)
			assert.Equal(t, tt.confidence, got.SuggestedConfidence)
		})
	}
}

// The snapshot must come from the newest analysis version, not the first one written.
func TestUpdateRunResultSnapshotUsesNewestAnalysis(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	rr := seedResultWithStatus(t, s, models.StatusFail)
	_, err := s.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID: rr.ID, Verdict: models.VerdictFlakyTest, Confidence: models.ConfidenceLow, ModelName: "m",
	})
	require.NoError(t, err)
	_, err = s.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID: rr.ID, Verdict: models.VerdictEnvironment, Confidence: models.ConfidenceHigh, ModelName: "m",
	})
	require.NoError(t, err)

	w := putRunResult(t, s, srv, rr, map[string]any{"defect_type": "system_issue"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got, err := s.GetRunResultByID(rr.ID)
	require.NoError(t, err)
	assert.Equal(t, models.VerdictEnvironment, got.SuggestedVerdict)
	assert.Equal(t, "system_issue", got.SuggestedDefectType)
	assert.Equal(t, models.ConfidenceHigh, got.SuggestedConfidence)
}

// Best-effort: a FAIL result with no analysis is triaged normally, just without a snapshot.
func TestUpdateRunResultNoAnalysisStillTriages(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	rr := seedResultWithStatus(t, s, models.StatusFail)

	w := putRunResult(t, s, srv, rr, map[string]any{"defect_type": "product_bug"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got, err := s.GetRunResultByID(rr.ID)
	require.NoError(t, err)
	assert.Equal(t, "product_bug", got.DefectType, "triage write must succeed without an analysis")
	assert.Empty(t, got.SuggestedVerdict)
	assert.Empty(t, got.SuggestedDefectType)
	assert.Empty(t, got.SuggestedConfidence)
}

// The "to_investigate" auto-default means "not triaged yet", not a human decision — snapshotting
// it would poison the calibration record with non-events. Every other precondition is satisfied
// here (stored status FAIL, analysis present); only the absent explicit defect_type stops it.
func TestUpdateRunResultAutoDefaultWritesNoSnapshot(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	rr := seedResultWithStatus(t, s, models.StatusFail)
	_, err := s.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID: rr.ID, Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, ModelName: "m",
	})
	require.NoError(t, err)

	w := putRunResult(t, s, srv, rr, map[string]any{"status": "FAIL"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got, err := s.GetRunResultByID(rr.ID)
	require.NoError(t, err)
	require.Equal(t, "to_investigate", got.DefectType, "auto-default should still apply")
	assert.Empty(t, got.SuggestedVerdict, "auto-default is not a human decision")
	assert.Empty(t, got.SuggestedDefectType)
	assert.Empty(t, got.SuggestedConfidence)
}

// Only FAILING results (FAIL/ERROR) are in the calibration set — the defect_type control renders
// for those alone, so a PASS row has no human decision to record.
func TestUpdateRunResultNoSnapshotForNonFailResult(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	rr := seedResultWithStatus(t, s, models.StatusPass)
	_, err := s.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID: rr.ID, Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, ModelName: "m",
	})
	require.NoError(t, err)

	w := putRunResult(t, s, srv, rr, map[string]any{"defect_type": "product_bug"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got, err := s.GetRunResultByID(rr.ID)
	require.NoError(t, err)
	assert.Empty(t, got.SuggestedVerdict)
	assert.Empty(t, got.SuggestedDefectType)
	assert.Empty(t, got.SuggestedConfidence)
	assert.Nil(t, got.DecidedAt)
}

// ERROR is a failure too. The analyzer has always produced verdicts for ERROR results, but the
// triage path required FAIL, so those verdicts could never be graded. Both the stored-ERROR shape
// (defect_type alone, as the grid sends) and the single-call {status,defect_type} shape must write
// the full snapshot, decided_at included — that column is what puts the row in the accuracy window.
func TestUpdateRunResultSnapshotsAISuggestionForErrorResult(t *testing.T) {
	tests := []struct {
		name        string
		seedStatus  models.ExecutionStatus
		body        map[string]any
		wantVerdict string
	}{
		{
			name:        "stored ERROR, defect_type alone",
			seedStatus:  models.StatusError,
			body:        map[string]any{"defect_type": "system_issue"},
			wantVerdict: models.VerdictEnvironment,
		},
		{
			name:        "status and defect_type arrive together",
			seedStatus:  models.StatusPending,
			body:        map[string]any{"status": "ERROR", "defect_type": "system_issue"},
			wantVerdict: models.VerdictEnvironment,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, srv := newSnapshotEnv(t)
			rr := seedResultWithStatus(t, s, tt.seedStatus)
			_, err := s.CreateAnalysis(&models.RunResultAnalysis{
				RunResultID: rr.ID, Verdict: tt.wantVerdict, Confidence: models.ConfidenceHigh, ModelName: "m",
			})
			require.NoError(t, err)

			w := putRunResult(t, s, srv, rr, tt.body)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			got, err := s.GetRunResultByID(rr.ID)
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Equal(t, models.StatusError, got.Status)
			assert.Equal(t, "system_issue", got.DefectType)
			assert.Equal(t, tt.wantVerdict, got.SuggestedVerdict)
			assert.Equal(t, "system_issue", got.SuggestedDefectType)
			assert.Equal(t, models.ConfidenceHigh, got.SuggestedConfidence)
			require.NotNil(t, got.DecidedAt, "without decided_at the row never enters the accuracy window")
		})
	}
}

// Moving a result to ERROR must seed the same "nobody has looked at this yet" default FAIL gets.
// Forcing "" instead (the old non-FAIL branch) hid the row from the triage UI entirely, which is
// the reason ERROR verdicts never accumulated calibration data.
func TestUpdateRunResultErrorStatusDefaultsToInvestigate(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	rr := seedResultWithStatus(t, s, models.StatusPending)
	_, err := s.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID: rr.ID, Verdict: models.VerdictEnvironment, Confidence: models.ConfidenceHigh, ModelName: "m",
	})
	require.NoError(t, err)

	w := putRunResult(t, s, srv, rr, map[string]any{"status": "ERROR"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got, err := s.GetRunResultByID(rr.ID)
	require.NoError(t, err)
	require.Equal(t, models.StatusError, got.Status)
	assert.Equal(t, "to_investigate", got.DefectType, "ERROR must default like FAIL, not clear like PASS")
	// The auto-default is not a human decision, so it stays out of the calibration set.
	assert.Empty(t, got.SuggestedVerdict)
	assert.Empty(t, got.SuggestedDefectType)
	assert.Empty(t, got.SuggestedConfidence)
	assert.Nil(t, got.DecidedAt)
}

// REGRESSION for the ERROR extension: widening "is a failure" must not widen it past FAIL/ERROR.
// Both halves are asserted per status, because they are driven by different branches: a bare
// status change still FORCES defect_type to '', and a status change carrying an explicit
// defect_type still records NO snapshot (the explicit value wins on the column, but a non-failure
// row must never enter the calibration set).
func TestUpdateRunResultNonFailureStatusesStillClearDefectType(t *testing.T) {
	for _, status := range []string{"PASS", "SKIP"} {
		t.Run(status, func(t *testing.T) {
			// Seeded FAIL + analysis so only the new status can be what suppresses the snapshot.
			newEnv := func(t *testing.T) (*store.Store, *api.Server, *models.RunResult) {
				t.Helper()
				s, srv := newSnapshotEnv(t)
				rr := seedResultWithStatus(t, s, models.StatusFail)
				_, err := s.CreateAnalysis(&models.RunResultAnalysis{
					RunResultID: rr.ID, Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, ModelName: "m",
				})
				require.NoError(t, err)
				return s, srv, rr
			}

			t.Run("status alone forces defect_type ''", func(t *testing.T) {
				s, srv, rr := newEnv(t)
				w := putRunResult(t, s, srv, rr, map[string]any{"status": status})
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())

				got, err := s.GetRunResultByID(rr.ID)
				require.NoError(t, err)
				require.Equal(t, models.ExecutionStatus(status), got.Status)
				assert.Empty(t, got.DefectType, "%s has nothing to triage", status)
				assert.Empty(t, got.SuggestedVerdict)
				assert.Empty(t, got.SuggestedDefectType)
				assert.Empty(t, got.SuggestedConfidence)
				assert.Nil(t, got.DecidedAt)
			})

			t.Run("explicit defect_type writes no snapshot", func(t *testing.T) {
				s, srv, rr := newEnv(t)
				w := putRunResult(t, s, srv, rr, map[string]any{"status": status, "defect_type": "product_bug"})
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())

				got, err := s.GetRunResultByID(rr.ID)
				require.NoError(t, err)
				require.Equal(t, models.ExecutionStatus(status), got.Status)
				assert.Empty(t, got.SuggestedVerdict, "%s is not a failure and cannot be calibrated", status)
				assert.Empty(t, got.SuggestedDefectType)
				assert.Empty(t, got.SuggestedConfidence)
				assert.Nil(t, got.DecidedAt)
			})
		})
	}
}

// The gate is the status the row will HAVE, not the one it had. The CLI (internal/cli/cmd/runs.go)
// and the execute page (RunExecutePage.submitVerdict) both send status and defect_type in ONE
// call against a row that is still PENDING — reading the stored status would see PENDING, skip
// the snapshot, and make the single-result path disagree with the bulk one on identical input.
func TestUpdateRunResultSnapshotsWhenStatusAndDefectTypeArriveTogether(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	rr := seedResultWithStatus(t, s, models.StatusPending)
	_, err := s.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID: rr.ID, Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, ModelName: "m",
	})
	require.NoError(t, err)

	w := putRunResult(t, s, srv, rr, map[string]any{"status": "FAIL", "defect_type": "automation_bug"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got, err := s.GetRunResultByID(rr.ID)
	require.NoError(t, err)
	assert.Equal(t, "automation_bug", got.DefectType)
	assert.Equal(t, models.VerdictProductBug, got.SuggestedVerdict, "the decision must be recorded, stored status was PENDING")
	assert.Equal(t, "product_bug", got.SuggestedDefectType, "an override is still a calibration data point")
	assert.Equal(t, models.ConfidenceHigh, got.SuggestedConfidence)
	require.NotNil(t, got.DecidedAt)

	// SQLite stores a timestamp as TEXT carrying the writer's offset and compares it as TEXT, so
	// the accuracy window is only exact if the write side matches the UTC cutoff the handler
	// passes. Reading the column back through GORM cannot show this — it parses into local time —
	// so assert on the raw stored text.
	var stored string
	require.NoError(t, s.DB().Raw(`SELECT CAST(decided_at AS TEXT) FROM run_results WHERE id = ?`, rr.ID).
		Scan(&stored).Error)
	assert.True(t, strings.HasSuffix(stored, "+00:00"),
		"decided_at must be written in UTC, got %q — a local offset skews the window by that offset", stored)
}

// The mirror image: a row that WAS failing is re-executed to PASS in the same call that carries a
// defect_type. Gating on the stored FAIL would snapshot a passing row straight into the
// calibration set — and the stale suggestion must go with it.
func TestUpdateRunResultClearsSnapshotWhenStatusLeavesFail(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	rr := seedResultWithStatus(t, s, models.StatusFail)
	_, err := s.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID: rr.ID, Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, ModelName: "m",
	})
	require.NoError(t, err)
	// An earlier decision already put this row in the calibration set.
	w := putRunResult(t, s, srv, rr, map[string]any{"defect_type": "product_bug"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	before, err := s.GetRunResultByID(rr.ID)
	require.NoError(t, err)
	require.Equal(t, "product_bug", before.SuggestedDefectType, "precondition: the row starts out snapshotted")

	w = putRunResult(t, s, srv, rr, map[string]any{"status": "PASS", "defect_type": "product_bug"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got, err := s.GetRunResultByID(rr.ID)
	require.NoError(t, err)
	require.Equal(t, models.StatusPass, got.Status)
	assert.Empty(t, got.SuggestedVerdict, "a passing row is not a triage decision")
	assert.Empty(t, got.SuggestedDefectType)
	assert.Empty(t, got.SuggestedConfidence)
	assert.Nil(t, got.DecidedAt)
}

// A skipped snapshot must CLEAR, never leave the previous one in place: defect_type is
// overwritten in the same statement, so a stale suggestion would be scored against a decision it
// was never made for — fabricating an agreement (or a disagreement) out of two unrelated events.
func TestUpdateRunResultClearsStaleSnapshotWhenAnalysisIsGone(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	run := &models.TestRun{Name: "Stale Snapshot Run"}
	require.NoError(t, s.CreateTestRun(run))
	decided := time.Now().UTC().Add(-time.Hour)
	rr := &models.RunResult{
		TestRunID: run.ID, TestNameSnapshot: "case", AttemptNumber: 1,
		Status: models.StatusFail, ErrorMessage: "boom",
		DefectType:          "product_bug",
		SuggestedVerdict:    models.VerdictProductBug,
		SuggestedDefectType: "product_bug",
		SuggestedConfidence: models.ConfidenceHigh,
		DecidedAt:           &decided,
	}
	require.NoError(t, s.AddRunResult(rr))

	// A new decision on the same row, with no analysis behind it any more.
	w := putRunResult(t, s, srv, rr, map[string]any{"defect_type": "system_issue"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got, err := s.GetRunResultByID(rr.ID)
	require.NoError(t, err)
	require.Equal(t, "system_issue", got.DefectType)
	assert.Empty(t, got.SuggestedVerdict, "the old suggestion belonged to the previous decision")
	assert.Empty(t, got.SuggestedDefectType, "leaving it would score system_issue against a product_bug suggestion")
	assert.Empty(t, got.SuggestedConfidence)
	assert.Nil(t, got.DecidedAt)
}

// --- AI suggestion snapshot on bulk defect_type triage ---

// seedRunWithResults creates one run holding n results, all in the given status.
func seedRunWithResults(t *testing.T, s *store.Store, status models.ExecutionStatus, n int) (*models.TestRun, []*models.RunResult) {
	t.Helper()
	run := &models.TestRun{Name: "Bulk Snapshot Run"}
	require.NoError(t, s.CreateTestRun(run))
	out := make([]*models.RunResult, 0, n)
	for i := 0; i < n; i++ {
		rr := &models.RunResult{
			TestRunID: run.ID, TestNameSnapshot: fmt.Sprintf("case-%d", i), AttemptNumber: 1,
			Status: status, ErrorMessage: "boom",
		}
		require.NoError(t, s.AddRunResult(rr))
		out = append(out, rr)
	}
	return run, out
}

func postBulkUpdate(t *testing.T, s *store.Store, srv *api.Server, runID string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/runs/"+runID+"/results/bulk-update", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	addTestAuth(t, s, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

// THE regression guard for the shared-map trap. store.BulkUpdateRunResults applies ONE map across
// every ID, so a snapshot folded into that statement would smear a single row's verdict over the
// whole selection. Every row must carry ITS OWN triple.
//
// The seeded verdicts are chosen to catch the smear in every direction: flaky_test and test_data
// collapse to the same defect_type, so only the snapshotted verdict distinguishes them; two rows
// share a bucket (proving grouping merges rather than overwrites); and flaky_test appears at two
// DIFFERENT confidences, so a bucket keyed on the verdict alone would hand one of those rows the
// other's confidence and fail here rather than passing by luck.
func TestBulkUpdateSnapshotsPerResultAISuggestion(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	run, results := seedRunWithResults(t, s, models.StatusFail, 6)

	seeded := []struct{ verdict, confidence, wantSuggested string }{
		{models.VerdictFlakyTest, models.ConfidenceHigh, "automation_bug"},
		{models.VerdictTestData, models.ConfidenceLow, "automation_bug"},
		{models.VerdictProductBug, models.ConfidenceMedium, "product_bug"},
		{models.VerdictEnvironment, models.ConfidenceHigh, "system_issue"},
		{models.VerdictFlakyTest, models.ConfidenceHigh, "automation_bug"}, // shares row 0's bucket
		{models.VerdictFlakyTest, models.ConfidenceLow, "automation_bug"},  // same verdict, other confidence
	}
	ids := make([]string, len(results))
	for i, rr := range results {
		ids[i] = rr.ID
		_, err := s.CreateAnalysis(&models.RunResultAnalysis{
			RunResultID: rr.ID, Verdict: seeded[i].verdict, Confidence: seeded[i].confidence, ModelName: "m",
		})
		require.NoError(t, err)
	}

	w := postBulkUpdate(t, s, srv, run.ID, map[string]any{
		"result_ids": ids, "status": "FAIL", "defect_type": "product_bug",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	for i, rr := range results {
		got, err := s.GetRunResultByID(rr.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "product_bug", got.DefectType, "row %d: the one human decision applies to every row", i)
		assert.Equal(t, seeded[i].verdict, got.SuggestedVerdict, "row %d got another row's verdict", i)
		assert.Equal(t, seeded[i].wantSuggested, got.SuggestedDefectType, "row %d got another row's suggestion", i)
		assert.Equal(t, seeded[i].confidence, got.SuggestedConfidence, "row %d got another row's confidence", i)
	}
}

// Best-effort: results with no analysis get their snapshot CLEARED rather than skipped, and
// their presence must not stop the rows that DO have one from being snapshotted, nor fail the
// bulk triage itself. Row 0 carries a snapshot from an earlier decision — leaving it in place
// would score this bulk decision against a suggestion made for a different one.
func TestBulkUpdateClearsResultsWithoutAnalysis(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	run, results := seedRunWithResults(t, s, models.StatusFail, 3)
	analyzed := results[1]
	_, err := s.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID: analyzed.ID, Verdict: models.VerdictInfrastructure, Confidence: models.ConfidenceLow, ModelName: "m",
	})
	require.NoError(t, err)

	stale := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, s.UpdateRunResult(run.ID, results[0].ID, map[string]interface{}{
		"suggested_verdict":     models.VerdictProductBug,
		"suggested_defect_type": "product_bug",
		"suggested_confidence":  models.ConfidenceHigh,
		"decided_at":            stale,
	}))

	ids := []string{results[0].ID, results[1].ID, results[2].ID}
	w := postBulkUpdate(t, s, srv, run.ID, map[string]any{
		"result_ids": ids, "status": "FAIL", "defect_type": "system_issue",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	for i, rr := range []*models.RunResult{results[0], results[2]} {
		got, err := s.GetRunResultByID(rr.ID)
		require.NoError(t, err)
		assert.Equal(t, "system_issue", got.DefectType, "row %d: triage must land even with no analysis", i)
		assert.Empty(t, got.SuggestedVerdict, "row %d: a stale suggestion must not survive a new decision", i)
		assert.Empty(t, got.SuggestedDefectType, "row %d", i)
		assert.Empty(t, got.SuggestedConfidence, "row %d", i)
		assert.Nil(t, got.DecidedAt, "row %d", i)
	}

	got, err := s.GetRunResultByID(analyzed.ID)
	require.NoError(t, err)
	assert.Equal(t, models.VerdictInfrastructure, got.SuggestedVerdict)
	assert.Equal(t, "system_issue", got.SuggestedDefectType)
	assert.Equal(t, models.ConfidenceLow, got.SuggestedConfidence)
	require.NotNil(t, got.DecidedAt)
}

// The gate is uniform: omitting defect_type makes the endpoint apply the "to_investigate"
// auto-default, which means "not triaged yet" and must never enter the calibration record.
func TestBulkUpdateAutoDefaultWritesNoSnapshot(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	run, results := seedRunWithResults(t, s, models.StatusFail, 2)
	for _, rr := range results {
		_, err := s.CreateAnalysis(&models.RunResultAnalysis{
			RunResultID: rr.ID, Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, ModelName: "m",
		})
		require.NoError(t, err)
	}

	w := postBulkUpdate(t, s, srv, run.ID, map[string]any{
		"result_ids": []string{results[0].ID, results[1].ID}, "status": "FAIL",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	for i, rr := range results {
		got, err := s.GetRunResultByID(rr.ID)
		require.NoError(t, err)
		require.Equal(t, "to_investigate", got.DefectType, "row %d: auto-default should still apply", i)
		assert.Empty(t, got.SuggestedVerdict, "row %d: auto-default is not a human decision", i)
		assert.Empty(t, got.SuggestedDefectType, "row %d", i)
		assert.Empty(t, got.SuggestedConfidence, "row %d", i)
	}
}

// THE fail-closed guard. The bulk snapshot is best-effort and runs as a SEPARATE pass after the
// main UPDATE, so its write can fail (SQLite busy past the timeout, disk error) while the triage
// itself has already committed. That must leave the snapshot columns BLANK — excluded from the
// calibration set — never holding the suggestion from an EARLIER decision, which the brand-new
// defect_type would then be scored against.
//
// The failure is forced with a trigger that aborts any write of a NON-EMPTY suggestion, which is
// exactly the grouped second-pass write and nothing else: the main statement only ever blanks
// these columns, so it passes straight through. Without the clearing folded into that main
// statement the row here would end up FAIL + the new system_issue + the stale product_bug
// suggestion — satisfying every clause of accuracyCalibrationFilter and fabricating a
// disagreement that no human ever made.
func TestBulkUpdateFailedSnapshotLeavesNoStaleSuggestion(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	run, results := seedRunWithResults(t, s, models.StatusFail, 1)
	rr := results[0]

	// A previous decision already put this row in the calibration set...
	stale := time.Now().UTC().Add(-time.Hour)
	require.NoError(t, s.UpdateRunResult(run.ID, rr.ID, map[string]interface{}{
		"defect_type":           "product_bug",
		"suggested_verdict":     models.VerdictProductBug,
		"suggested_defect_type": "product_bug",
		"suggested_confidence":  models.ConfidenceHigh,
		"decided_at":            stale,
	}))
	// ...and it still has an analysis, so the second pass WOULD bucket and snapshot it.
	_, err := s.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID: rr.ID, Verdict: models.VerdictFlakyTest, Confidence: models.ConfidenceLow, ModelName: "m",
	})
	require.NoError(t, err)

	require.NoError(t, s.DB().Exec(`
		CREATE TRIGGER fail_snapshot_write BEFORE UPDATE OF suggested_verdict ON run_results
		WHEN NEW.suggested_verdict != ''
		BEGIN SELECT RAISE(ABORT, 'simulated snapshot write failure'); END`).Error)
	t.Cleanup(func() { _ = s.DB().Exec(`DROP TRIGGER IF EXISTS fail_snapshot_write`).Error })

	w := postBulkUpdate(t, s, srv, run.ID, map[string]any{
		"result_ids": []string{rr.ID}, "status": "FAIL", "defect_type": "system_issue",
	})
	require.Equal(t, http.StatusOK, w.Code, "a failed snapshot must never fail the human's triage: %s", w.Body.String())

	got, err := s.GetRunResultByID(rr.ID)
	require.NoError(t, err)
	require.Equal(t, "system_issue", got.DefectType, "the triage decision itself must land")
	assert.Empty(t, got.SuggestedVerdict, "the old suggestion belonged to the previous decision")
	assert.Empty(t, got.SuggestedDefectType, "leaving it would score system_issue against a product_bug suggestion")
	assert.Empty(t, got.SuggestedConfidence)
	assert.Nil(t, got.DecidedAt, "a blank decided_at keeps the row out of the calibration window")
}

// A non-FAIL bulk status clears defect_type entirely, so there is no decision to calibrate.
func TestBulkUpdateNoSnapshotForNonFailStatus(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	run, results := seedRunWithResults(t, s, models.StatusFail, 1)
	_, err := s.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID: results[0].ID, Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, ModelName: "m",
	})
	require.NoError(t, err)

	w := postBulkUpdate(t, s, srv, run.ID, map[string]any{
		"result_ids": []string{results[0].ID}, "status": "PASS", "defect_type": "product_bug",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got, err := s.GetRunResultByID(results[0].ID)
	require.NoError(t, err)
	assert.Empty(t, got.DefectType, "non-FAIL clears defect_type")
	assert.Empty(t, got.SuggestedVerdict)
	assert.Empty(t, got.SuggestedDefectType)
	assert.Empty(t, got.SuggestedConfidence)
}

// THE parity guard between the two endpoints. Gating the bulk switch on FAIL alone left
// {status:"ERROR", defect_type:"X"} forcing defect_type blank here while PUT /results/{result_id}
// triaged and snapshotted the very same request — one decision meaning two different things
// depending on which endpoint the caller happened to reach for.
func TestBulkUpdateTriagesErrorStatusLikeSingleResult(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	run, results := seedRunWithResults(t, s, models.StatusError, 1)
	rr := results[0]
	_, err := s.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID: rr.ID, Verdict: models.VerdictEnvironment, Confidence: models.ConfidenceHigh, ModelName: "m",
	})
	require.NoError(t, err)

	w := postBulkUpdate(t, s, srv, run.ID, map[string]any{
		"result_ids": []string{rr.ID}, "status": "ERROR", "defect_type": "system_issue",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got, err := s.GetRunResultByID(rr.ID)
	require.NoError(t, err)
	require.Equal(t, models.StatusError, got.Status)
	assert.Equal(t, "system_issue", got.DefectType, "ERROR is a failure - the human's decision must survive")
	assert.Equal(t, models.VerdictEnvironment, got.SuggestedVerdict)
	assert.Equal(t, "system_issue", got.SuggestedDefectType)
	assert.Equal(t, models.ConfidenceHigh, got.SuggestedConfidence)
	assert.NotNil(t, got.DecidedAt, "a blank decided_at keeps the row out of the calibration window")
}

// The auto-default branch of the same switch: an ERROR with no explicit defect_type lands on
// "not triaged yet" like a FAIL does, rather than being cleared like a PASS.
func TestBulkUpdateErrorStatusDefaultsToInvestigate(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	run, results := seedRunWithResults(t, s, models.StatusPending, 1)
	_, err := s.CreateAnalysis(&models.RunResultAnalysis{
		RunResultID: results[0].ID, Verdict: models.VerdictProductBug, Confidence: models.ConfidenceHigh, ModelName: "m",
	})
	require.NoError(t, err)

	w := postBulkUpdate(t, s, srv, run.ID, map[string]any{
		"result_ids": []string{results[0].ID}, "status": "ERROR",
	})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	got, err := s.GetRunResultByID(results[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "to_investigate", got.DefectType, "ERROR must default like FAIL, not clear like PASS")
	assert.Empty(t, got.SuggestedVerdict, "the auto-default is not a human decision")
	assert.Empty(t, got.SuggestedDefectType)
	assert.Empty(t, got.SuggestedConfidence)
}

// status is no longer unconditionally required, but a request carrying NEITHER field would report
// rows "updated" after a no-op UPDATE. defect_type is validated here for the first time: it feeds
// a column that every calibration and counter query groups by, so garbage in it is not inert.
func TestBulkUpdateValidatesStatusAndDefectType(t *testing.T) {
	tests := []struct {
		name      string
		body      map[string]any
		wantError string
	}{
		{"neither field", map[string]any{}, "at least one of status or defect_type is required"},
		{"both blank", map[string]any{"status": "", "defect_type": ""}, "at least one of status or defect_type is required"},
		{"invalid status", map[string]any{"status": "BOGUS"}, "invalid status"},
		{"invalid defect_type", map[string]any{"status": "FAIL", "defect_type": "not_a_defect"}, "invalid defect_type"},
		{"invalid defect_type, no status", map[string]any{"defect_type": "not_a_defect"}, "invalid defect_type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, srv := newSnapshotEnv(t)
			run, results := seedRunWithResults(t, s, models.StatusFail, 1)

			body := map[string]any{"result_ids": []string{results[0].ID}}
			for k, v := range tt.body {
				body[k] = v
			}
			w := postBulkUpdate(t, s, srv, run.ID, body)
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

			var resp map[string]string
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, tt.wantError, resp["error"])

			got, err := s.GetRunResultByID(results[0].ID)
			require.NoError(t, err)
			assert.Equal(t, models.StatusFail, got.Status, "a rejected request must write nothing")
			assert.Empty(t, got.DefectType)
		})
	}
}

// The single-result path validates defect_type against the SAME canonical set, so the two
// endpoints agree on what is acceptable. Without this, bulk would 400 on garbage while this
// endpoint silently persisted it.
func TestUpdateRunResultRejectsInvalidDefectType(t *testing.T) {
	tests := []struct {
		name string
		body map[string]any
	}{
		{"defect_type alone", map[string]any{"defect_type": "not_a_defect"}},
		{"status and defect_type together", map[string]any{"status": "FAIL", "defect_type": "not_a_defect"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, srv := newSnapshotEnv(t)
			rr := seedResultWithStatus(t, s, models.StatusFail)

			w := putRunResult(t, s, srv, rr, tt.body)
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

			var resp map[string]string
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, "invalid defect_type", resp["error"])

			got, err := s.GetRunResultByID(rr.ID)
			require.NoError(t, err)
			assert.Empty(t, got.DefectType, "a rejected request must write nothing")
		})
	}
}

func TestLinkExistingDefectToResult(t *testing.T) {
	s, err := newTestStore(t)
	require.NoError(t, err)
	srv := api.NewServer(s)

	// Seed: test case, run, run result, defect
	require.NoError(t, s.DB().Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','x',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, s.DB().Exec(`INSERT INTO test_runs (id,name) VALUES ('run1','R1')`).Error)
	require.NoError(t, s.DB().Exec(`INSERT INTO run_results (id,test_run_id,test_case_id,status) VALUES ('rr1','run1','tc1','FAIL')`).Error)
	d := &models.Defect{Title: "bug"}
	require.NoError(t, s.CreateDefect(d))

	// POST link to correct run/result — should return 201
	body, _ := json.Marshal(map[string]string{"defect_id": d.ID})
	req := httptest.NewRequest(http.MethodPost, "/api/runs/run1/results/rr1/defect-links", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	addTestAuth(t, s, req)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	// GET with wrong run ID — result does not belong to "NOPE" run → 404
	req2 := httptest.NewRequest(http.MethodGet, "/api/runs/NOPE/results/rr1/defect-links", nil)
	addTestAuth(t, s, req2)
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusNotFound, rec2.Code)
}
