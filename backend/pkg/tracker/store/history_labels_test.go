package store

import (
	"testing"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// Enrichment can only ignore AI labels if the history query returns the source column.
func TestListRecentFailuresByTestCase_CarriesTheLabelSource(t *testing.T) {
	s := newTestStore(t)
	folder, err := s.CreateFolder("F", nil)
	require.NoError(t, err)
	tc := &models.TestCase{FolderID: folder.ID, Name: "c"}
	require.NoError(t, s.CreateTestCase(tc))
	tcID := tc.ID
	prior := seedRun(t, s)
	ran := time.Now().UTC().Add(-48 * time.Hour)
	rr := &models.RunResult{TestRunID: prior, TestCaseID: &tcID, TestNameSnapshot: "c", AttemptNumber: 1,
		Status: models.StatusFail, ErrorMessage: "boom", StartTime: ran}
	require.NoError(t, s.AddRunResult(rr))
	n, err := s.ApplyAutoDefectType([]string{rr.ID}, "product_bug")
	require.NoError(t, err)
	require.EqualValues(t, 1, n)

	hist, err := s.ListRecentFailuresByTestCase(tcID, ran.Add(-time.Hour), time.Now().UTC(), 10, "current-run")
	require.NoError(t, err)
	require.Len(t, hist, 1)
	require.Equal(t, "product_bug", hist[0].DefectType)
	require.Equal(t, models.DefectTypeSourceAI, hist[0].DefectTypeSource)
}
