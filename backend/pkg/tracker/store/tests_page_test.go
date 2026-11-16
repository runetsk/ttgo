package store

import (
	"testing"
	"time"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// names returns the test-case names in order, for terse assertions.
func names(rows []*models.TestCase) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Name
	}
	return out
}

func TestListTestCasesPage_PaginationAndTotal(t *testing.T) {
	s := newTestStore(t)
	root, _ := s.CreateFolder("Root", nil)
	for _, n := range []string{"a", "b", "c", "d", "e"} {
		require.NoError(t, s.CreateTestCase(&models.TestCase{Name: n, FolderID: root.ID}))
	}
	rows, total, err := s.ListTestCasesPage(TestPageQuery{
		FolderIDs: []string{root.ID}, Sort: "name", Direction: "asc", Limit: 2, Offset: 2,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Equal(t, []string{"c", "d"}, names(rows))

	// Offset beyond the last row: empty page, true total.
	rows, total, err = s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, Limit: 2, Offset: 99})
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Empty(t, rows)
}

func TestListTestCasesPage_FolderScoping(t *testing.T) {
	s := newTestStore(t)
	a, _ := s.CreateFolder("A", nil)
	child, _ := s.CreateFolder("Child", &a.ID)
	b, _ := s.CreateFolder("B", nil)
	require.NoError(t, s.CreateTestCase(&models.TestCase{Name: "inA", FolderID: a.ID}))
	require.NoError(t, s.CreateTestCase(&models.TestCase{Name: "inChild", FolderID: child.ID}))
	require.NoError(t, s.CreateTestCase(&models.TestCase{Name: "inB", FolderID: b.ID}))

	// Selecting A includes its descendants but not B.
	rows, total, err := s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{a.ID}, Sort: "name", Direction: "asc", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Equal(t, []string{"inA", "inChild"}, names(rows))

	// Selected folders that resolve to nothing must NOT broaden to every case.
	rows, total, err = s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{"ghost"}, Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, rows)
}

func TestListTestCasesPage_GlobalSearchUnicodeAndFields(t *testing.T) {
	s := newTestStore(t)
	root, _ := s.CreateFolder("Root", nil)
	cat, _ := s.CreateCategory("Смоук", "") // Cyrillic category name
	tcName := &models.TestCase{Name: "ПРИВЕТ login", FolderID: root.ID}
	require.NoError(t, s.CreateTestCase(tcName))
	byCat := &models.TestCase{Name: "other", FolderID: root.ID, Categories: []*models.Category{cat}}
	require.NoError(t, s.CreateTestCase(byCat))
	require.NoError(t, s.CreateTestCase(&models.TestCase{Name: "unrelated", FolderID: root.ID}))

	// Non-Latin, case-insensitive match on name.
	rows, total, err := s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, Search: "привет", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, []string{"ПРИВЕТ login"}, names(rows))

	// Global search also matches a category name (Cyrillic, case-insensitive).
	rows, _, err = s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, Search: "смоук", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, []string{"other"}, names(rows))

	// Global search matches the id.
	rows, _, err = s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, Search: tcName.ID, Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, []string{"ПРИВЕТ login"}, names(rows))
}

func TestListTestCasesPage_ColumnFilters(t *testing.T) {
	s := newTestStore(t)
	root, _ := s.CreateFolder("Root", nil)
	cat, _ := s.CreateCategory("Smoke", "")
	withDesc := &models.TestCase{Name: "desc-case", FolderID: root.ID, Description: "<p>Hello &amp; welcome</p>"}
	require.NoError(t, s.CreateTestCase(withDesc))
	withCat := &models.TestCase{Name: "cat-case", FolderID: root.ID, Categories: []*models.Category{cat}}
	require.NoError(t, s.CreateTestCase(withCat))
	withSteps := &models.TestCase{Name: "steps-case", FolderID: root.ID, Steps: []*models.TestStep{
		{Action: "a", OrderIndex: 0}, {Action: "b", OrderIndex: 1}, {Action: "c", OrderIndex: 2},
	}}
	require.NoError(t, s.CreateTestCase(withSteps))

	// Description filter matches the HTML-stripped, entity-decoded text.
	rows, _, err := s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, Description: "hello & welcome", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, []string{"desc-case"}, names(rows))

	// Category ANY.
	rows, _, err = s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, CategoryIDs: []string{cat.ID}, Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, []string{"cat-case"}, names(rows))

	// Steps-count decimal substring ("3" matches the 3-step case).
	rows, _, err = s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, StepsCount: "3", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, []string{"steps-case"}, names(rows))

	// Name column filter.
	rows, _, err = s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, Name: "cat-", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, []string{"cat-case"}, names(rows))
}

func TestListTestCasesPage_ReverificationAndRequirementAndDates(t *testing.T) {
	s := newTestStore(t)
	root, _ := s.CreateFolder("Root", nil)

	flagged := &models.TestCase{Name: "flagged", FolderID: root.ID, ReverificationFlagged: true}
	require.NoError(t, s.CreateTestCase(flagged))
	require.NoError(t, s.CreateTestCase(&models.TestCase{Name: "plain", FolderID: root.ID}))

	rows, _, err := s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, Reverification: "yes", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, []string{"flagged"}, names(rows))
	rows, _, err = s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, Reverification: "no", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, []string{"plain"}, names(rows))

	// Linked requirement identifier filter.
	req := &models.Requirement{Identifier: "PROJ-42", Title: "T"}
	require.NoError(t, s.CreateRequirement(req))
	linked := &models.TestCase{Name: "linked", FolderID: root.ID}
	require.NoError(t, s.CreateTestCase(linked))
	_, err = s.CreateLink(req.ID, linked.ID)
	require.NoError(t, err)
	rows, _, err = s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, LinkedRequirement: "proj-42", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, []string{"linked"}, names(rows))

	// Date range is lower-inclusive, upper-exclusive on created_at.
	from := time.Now().Add(-time.Hour)
	to := time.Now().Add(time.Hour)
	_, total, err := s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, CreatedFrom: &from, CreatedTo: &to, Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total, "all three were created within the window")
	past := time.Now().Add(-48 * time.Hour)
	_, total, err = s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, CreatedTo: &past, Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total, "nothing created before 48h ago")
}

func TestListTestCasesPage_SortStableTieBreak(t *testing.T) {
	s := newTestStore(t)
	root, _ := s.CreateFolder("Root", nil)
	// Same name, so the id tie-break decides a stable order.
	t1 := &models.TestCase{ID: "id-b", Name: "same", FolderID: root.ID}
	t2 := &models.TestCase{ID: "id-a", Name: "same", FolderID: root.ID}
	require.NoError(t, s.CreateTestCase(t1))
	require.NoError(t, s.CreateTestCase(t2))

	rows, _, err := s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, Sort: "name", Direction: "asc", Limit: 50})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "id-a", rows[0].ID, "equal sort keys break ties by id ascending")
	assert.Equal(t, "id-b", rows[1].ID)

	// Descending primary sort still tie-breaks by id ascending (stable paging).
	rows, _, err = s.ListTestCasesPage(TestPageQuery{FolderIDs: []string{root.ID}, Sort: "id", Direction: "desc", Limit: 50})
	require.NoError(t, err)
	assert.Equal(t, []string{"id-b", "id-a"}, []string{rows[0].ID, rows[1].ID})
}
