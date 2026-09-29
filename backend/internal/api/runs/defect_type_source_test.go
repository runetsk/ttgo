package runs_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	api "ttgo/internal/api"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/require"
)

type resultWrite func(t *testing.T, s *store.Store, srv *api.Server, rr *models.RunResult) *httptest.ResponseRecorder

func singleWrite(body map[string]any) resultWrite {
	return func(t *testing.T, s *store.Store, srv *api.Server, rr *models.RunResult) *httptest.ResponseRecorder {
		return putRunResult(t, s, srv, rr, body)
	}
}

func bulkWrite(body map[string]any) resultWrite {
	return func(t *testing.T, s *store.Store, srv *api.Server, rr *models.RunResult) *httptest.ResponseRecorder {
		b := map[string]any{"result_ids": []string{rr.ID}}
		for k, v := range body {
			b[k] = v
		}
		return postBulkUpdate(t, s, srv, rr.TestRunID, b)
	}
}

// R2 + header amendment 1 + R10: every writer of defect_type other than auto-apply replaces the
// AI source in the same update — with "human" for a person's explicit choice (Confirm included,
// which writes the same value) and "" for the status-change default and the non-failure clear —
// and records that the label it replaced was an AI label.
func TestEveryDefectTypeWriteReplacesTheAISource(t *testing.T) {
	human := models.DefectTypeSourceHuman
	cases := []struct {
		name       string
		write      resultWrite
		wantDefect string
		wantSource string
	}{
		{"single triage", singleWrite(map[string]any{"defect_type": "automation_bug"}), "automation_bug", human},
		{"single confirm of the AI value", singleWrite(map[string]any{"defect_type": "product_bug"}), "product_bug", human},
		{"single status and triage", singleWrite(map[string]any{"status": "FAIL", "defect_type": "system_issue"}), "system_issue", human},
		{"single explicit to investigate", singleWrite(map[string]any{"defect_type": "to_investigate"}), "to_investigate", human},
		{"single status default", singleWrite(map[string]any{"status": "ERROR"}), "to_investigate", ""},
		{"single non-failure clear", singleWrite(map[string]any{"status": "PASS"}), "", ""},
		{"bulk triage", bulkWrite(map[string]any{"defect_type": "automation_bug"}), "automation_bug", human},
		{"bulk status and triage", bulkWrite(map[string]any{"status": "ERROR", "defect_type": "system_issue"}), "system_issue", human},
		{"bulk status default", bulkWrite(map[string]any{"status": "FAIL"}), "to_investigate", ""},
		{"bulk non-failure clear", bulkWrite(map[string]any{"status": "SKIP"}), "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, srv := newSnapshotEnv(t)
			rr := seedResultWithStatus(t, s, models.StatusFail)
			n, err := s.ApplyAutoDefectType([]string{rr.ID}, "product_bug")
			require.NoError(t, err)
			require.EqualValues(t, 1, n)

			w := tc.write(t, s, srv, rr)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())

			got, err := s.GetRunResultByID(rr.ID)
			require.NoError(t, err)
			require.Equal(t, tc.wantDefect, got.DefectType)
			require.Equal(t, tc.wantSource, got.DefectTypeSource, "the AI source is replaced in the same update")
			require.True(t, got.SuggestedAutoApplied, "R10: the write replaced an AI label")
		})
	}
}

// A person's triage of a label nobody had set by AI is an independent decision (R10).
func TestTriageOfAPersonsOwnLabelIsNotAConfirmation(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	rr := seedResultWithStatus(t, s, models.StatusFail)
	w := putRunResult(t, s, srv, rr, map[string]any{"defect_type": "product_bug"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got, err := s.GetRunResultByID(rr.ID)
	require.NoError(t, err)
	require.False(t, got.SuggestedAutoApplied)
	require.Equal(t, models.DefectTypeSourceHuman, got.DefectTypeSource)
}

// Header amendment 1, end to end: "to investigate" picked by a person on a result that has no
// analysis (the handler clears the snapshot, so decided_at stays NULL) is never labelled later.
func TestExplicitToInvestigateWithoutAnAnalysisIsNeverAutoLabelled(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	rr := seedResultWithStatus(t, s, models.StatusFail)
	w := putRunResult(t, s, srv, rr, map[string]any{"defect_type": "to_investigate"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	got, err := s.GetRunResultByID(rr.ID)
	require.NoError(t, err)
	require.Nil(t, got.DecidedAt)

	n, err := s.ApplyAutoDefectType([]string{rr.ID}, "product_bug")
	require.NoError(t, err)
	require.Zero(t, n)
	got, err = s.GetRunResultByID(rr.ID)
	require.NoError(t, err)
	require.Equal(t, "to_investigate", got.DefectType)
	require.Equal(t, models.DefectTypeSourceHuman, got.DefectTypeSource)
}

// A caller cannot claim an AI label: the create request has no source field.
func TestAddRunResultNeverTakesTheSourceFromTheCaller(t *testing.T) {
	s, srv := newSnapshotEnv(t)
	run := &models.TestRun{Name: "ingest"}
	require.NoError(t, s.CreateTestRun(run))
	folder, err := s.CreateFolder("F", nil)
	require.NoError(t, err)
	tc := &models.TestCase{FolderID: folder.ID, Name: "c"}
	require.NoError(t, s.CreateTestCase(tc))

	raw, err := json.Marshal(map[string]any{"test_case_id": tc.ID, "status": "FAIL", "defect_type": "product_bug",
		"defect_type_source": "ai", "suggested_auto_applied": true})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/api/runs/"+run.ID+"/results", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	addTestAuth(t, s, req)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var created models.RunResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	got, err := s.GetRunResultByID(created.ID)
	require.NoError(t, err)
	require.Equal(t, "product_bug", got.DefectType)
	require.Equal(t, "", got.DefectTypeSource)
	require.False(t, got.SuggestedAutoApplied)
}
