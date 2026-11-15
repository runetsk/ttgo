package store

import (
	"encoding/json"
	"testing"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

func TestUpdateTestCasePersistsEditsAndHistory(t *testing.T) {
	s := newTestStore(t)
	folder, err := s.CreateFolder("Edits", nil)
	require.NoError(t, err)
	tc := &models.TestCase{Name: "Case", FolderID: folder.ID, Description: "old description", Steps: []*models.TestStep{
		{Action: "old action", ExpectedResult: "old expected", OrderIndex: 0},
		{Action: "second", ExpectedResult: "second expected", OrderIndex: 1},
	}}
	require.NoError(t, s.CreateTestCase(tc))
	tc.Description = ""
	tc.Steps[0].Action = "new action"
	tc.Steps[0].ExpectedResult = ""
	tc.Steps[0].OrderIndex = 1
	tc.Steps[1].OrderIndex = 0
	require.NoError(t, s.UpdateTestCase(tc))
	fresh, err := s.GetTestCase(tc.ID)
	require.NoError(t, err)
	require.Empty(t, fresh.Description)
	byID := map[string]*models.TestStep{}
	for _, step := range fresh.Steps {
		byID[step.ID] = step
	}
	require.Equal(t, "new action", byID[tc.Steps[0].ID].Action)
	require.Empty(t, byID[tc.Steps[0].ID].ExpectedResult)
	require.Equal(t, 1, byID[tc.Steps[0].ID].OrderIndex)
	require.Equal(t, 0, byID[tc.Steps[1].ID].OrderIndex)
	versions, err := s.ListVersions(tc.ID)
	require.NoError(t, err)
	actual, err := buildSnapshot(fresh)
	require.NoError(t, err)
	require.JSONEq(t, actual, versions[0].Snapshot)
}

func TestUpdateTestCasePartialHistoryReflectsSavedFields(t *testing.T) {
	s := newTestStore(t)
	folder, err := s.CreateFolder("Edits", nil)
	require.NoError(t, err)
	tc := &models.TestCase{Name: "Keep name", FolderID: folder.ID}
	require.NoError(t, s.CreateTestCase(tc))
	require.NoError(t, s.UpdateTestCase(&models.TestCase{ID: tc.ID, Description: "changed"}))
	versions, err := s.ListVersions(tc.ID)
	require.NoError(t, err)
	var snap models.VersionSnapshot
	require.NoError(t, json.Unmarshal([]byte(versions[0].Snapshot), &snap))
	require.Equal(t, tc.Name, snap.Name)
}

func TestPasswordUpdateRevokesSessionsAtomically(t *testing.T) {
	for _, failDelete := range []bool{false, true} {
		t.Run(map[bool]string{false: "revoke", true: "rollback"}[failDelete], func(t *testing.T) {
			s := newTestStore(t)
			u, err := s.CreateUser("edit@example.test", "Editor", "old-hash", "member")
			require.NoError(t, err)
			sess, err := s.CreateSession(u.ID)
			require.NoError(t, err)
			if failDelete {
				require.NoError(t, s.DB().Exec(`CREATE TRIGGER reject_session_delete BEFORE DELETE ON user_sessions BEGIN SELECT RAISE(ABORT, 'cannot revoke'); END`).Error)
			}
			_, err = s.UpdateUser(u.ID, map[string]interface{}{"hashed_password": "new-hash"})
			if failDelete {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			fresh, err := s.GetUser(u.ID)
			require.NoError(t, err)
			valid, err := s.ValidateSession(sess.ID)
			require.NoError(t, err)
			if failDelete {
				require.Equal(t, "old-hash", fresh.HashedPassword)
				require.NotNil(t, valid)
			} else {
				require.Equal(t, "new-hash", fresh.HashedPassword)
				require.Nil(t, valid)
			}
		})
	}
}
