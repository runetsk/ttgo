package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func TestCountDefectLinksByRunResults(t *testing.T) {
	s := newTestStore(t)
	require.NoError(t, s.db.Exec(`INSERT INTO test_cases (id,name,created_at,updated_at) VALUES ('tc1','x',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO test_runs (id,name) VALUES ('run1','R1')`).Error)
	require.NoError(t, s.db.Exec(`INSERT INTO run_results (id,test_run_id,test_case_id,status) VALUES ('rr1','run1','tc1','FAIL')`).Error)

	o := &models.Defect{Title: "open", Status: "open"}
	c := &models.Defect{Title: "closed", Status: "closed"}
	require.NoError(t, s.CreateDefect(o))
	require.NoError(t, s.CreateDefect(c))
	_, _ = s.LinkDefectToResult(o.ID, "rr1", "tc1")
	_, _ = s.LinkDefectToResult(c.ID, "rr1", "tc1")

	open, closed, err := s.CountDefectLinksByRunResults([]string{"rr1"})
	require.NoError(t, err)
	assert.Equal(t, 1, open["rr1"])
	assert.Equal(t, 1, closed["rr1"])
}

func TestDefectCRUD(t *testing.T) {
	s := newTestStore(t)

	d := &models.Defect{Title: "Checkout 500", Severity: "major"}
	require.NoError(t, s.CreateDefect(d))
	assert.NotEmpty(t, d.ID)
	assert.Equal(t, "open", d.Status)

	got, err := s.GetDefect(d.ID)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Checkout 500", got.Title)

	closed := "closed"
	upd, err := s.UpdateDefect(d.ID, models.UpdateDefectRequest{Status: &closed})
	require.NoError(t, err)
	assert.Equal(t, "closed", upd.Status)

	list, err := s.ListDefects("closed", "", "checkout")
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, s.DeleteDefect(d.ID))
	gone, err := s.GetDefect(d.ID)
	require.NoError(t, err)
	assert.Nil(t, gone)
}
