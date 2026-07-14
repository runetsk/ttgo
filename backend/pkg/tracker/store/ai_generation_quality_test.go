package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/aigen"
	"ttgo/pkg/tracker/models"
)

func seedQualityFixture(t *testing.T) (*Store, *models.Requirement, *models.Folder) {
	t.Helper()
	s := newTestStore(t)
	req := &models.Requirement{Identifier: "REQ-DUP-1", Title: "Login"}
	require.NoError(t, s.CreateRequirement(req))
	folder, err := s.CreateFolder("Regression", nil)
	require.NoError(t, err)
	return s, req, folder
}

func createNamedTestCase(t *testing.T, s *Store, folderID, name string) *models.TestCase {
	t.Helper()
	tc := &models.TestCase{Name: name, FolderID: folderID}
	require.NoError(t, s.CreateTestCase(tc))
	return tc
}

func TestSearchDuplicateCandidates_RanksLinkedFirst(t *testing.T) {
	s, req, folder := seedQualityFixture(t)
	linked := createNamedTestCase(t, s, folder.ID, "Sign in with valid credentials")
	_, err := s.CreateLink(req.ID, linked.ID)
	require.NoError(t, err)
	createNamedTestCase(t, s, folder.ID, "Sign in with valid credentials happy path")
	createNamedTestCase(t, s, folder.ID, "Export report as CSV")

	cands, err := s.SearchDuplicateCandidates("[Functional] Sign in with valid credentials", req.ID, 5)
	require.NoError(t, err)
	require.Len(t, cands, 2, "the CSV case is below the similarity threshold")

	assert.Equal(t, linked.ID, cands[0].TestCaseID, "requirement-linked candidate ranks first")
	assert.Equal(t, aigen.DupKindExisting, cands[0].Kind)
	assert.Equal(t, folder.ID, cands[0].FolderID)
	assert.GreaterOrEqual(t, cands[0].Similarity, aigen.DupHighConfidence)
	assert.Contains(t, cands[0].Reason, "linked to this requirement")
	assert.Nil(t, cands[0].DraftPosition)
}

func TestSearchDuplicateCandidates_EmptyAndNoMatch(t *testing.T) {
	s, req, folder := seedQualityFixture(t)
	createNamedTestCase(t, s, folder.ID, "Export report as CSV")

	cands, err := s.SearchDuplicateCandidates("", req.ID, 5)
	require.NoError(t, err)
	assert.Empty(t, cands)

	cands, err = s.SearchDuplicateCandidates("Completely different topic entirely", req.ID, 5)
	require.NoError(t, err)
	assert.Empty(t, cands)
}

func TestUpdateGenerationRunCoverage(t *testing.T) {
	s, req, _ := seedQualityFixture(t)
	run, _, err := s.CreateGenerationRun(&models.AIGenerationRun{RequirementID: req.ID})
	require.NoError(t, err)

	require.NoError(t, s.UpdateGenerationRunCoverage(run.ID, `{"targets":[]}`))
	got, err := s.GetGenerationRun(run.ID)
	require.NoError(t, err)
	assert.Equal(t, `{"targets":[]}`, got.CoverageJSON)
}
