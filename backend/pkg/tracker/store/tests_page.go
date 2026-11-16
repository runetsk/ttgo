package store

import (
	"fmt"
	"strings"
	"time"
	"ttgo/pkg/tracker/models"

	"gorm.io/gorm"
)

// TestPageQuery is the server-side library page request: folder scope, the same
// filters the grid applied in the browser, a sort key from a fixed allowlist, and
// limit/offset. Text filters are Unicode case-insensitive substring matches
// (ttgo_icontains); counts match as decimal substrings; dates are
// lower-inclusive / upper-exclusive instants.
type TestPageQuery struct {
	FolderIDs         []string
	Search            string // global: name OR id OR category name
	ID                string
	Name              string
	Description       string // matched against HTML-stripped text
	CategoryIDs       []string
	StepsCount        string
	OpenDefects       string
	Reverification    string // "yes" | "no" | ""
	LinkedRequirement string // requirement identifier substring
	CreatedFrom       *time.Time
	CreatedTo         *time.Time
	UpdatedFrom       *time.Time
	UpdatedTo         *time.Time
	Sort              string
	Direction         string
	Limit             int
	Offset            int
}

// testPageSortExpr allowlists the sortable columns and maps each to a SQL
// expression. Values are fixed here (never user input), so interpolating them into
// ORDER BY is safe.
var testPageSortExpr = map[string]string{
	"name":                   "test_cases.name",
	"id":                     "test_cases.id",
	"created_at":             "test_cases.created_at",
	"updated_at":             "test_cases.updated_at",
	"reverification_flagged": "test_cases.reverification_flagged",
	"description":            "ttgo_lower(ttgo_striphtml(test_cases.description))",
	"steps_count":            "(SELECT COUNT(*) FROM test_steps ts WHERE ts.test_case_id = test_cases.id)",
	"open_defects":           "(SELECT COUNT(DISTINCT d.id) FROM defect_links dl JOIN defects d ON d.id = dl.defect_id WHERE dl.test_case_id = test_cases.id AND d.status != 'closed')",
	"categories":             "(SELECT group_concat(su.name, ', ') FROM suite_test_cases stc JOIN suites su ON su.id = stc.suite_id WHERE stc.test_case_id = test_cases.id)",
	"linked_requirements":    "(SELECT group_concat(r.identifier, ', ') FROM requirement_test_case_links l JOIN requirements r ON r.id = l.requirement_id WHERE l.test_case_id = test_cases.id)",
}

// IsValidTestPageSort reports whether key is an allowed sort column.
func IsValidTestPageSort(key string) bool {
	_, ok := testPageSortExpr[key]
	return ok
}

func testPageOrder(sortKey, direction string) string {
	expr, ok := testPageSortExpr[sortKey]
	if !ok {
		expr = testPageSortExpr["name"]
	}
	dir := "ASC"
	if strings.EqualFold(direction, "desc") {
		dir = "DESC"
	}
	// Stable id tie-break so paging is deterministic across equal sort keys.
	return fmt.Sprintf("%s %s, test_cases.id ASC", expr, dir)
}

// ListTestCasesPage returns one page of test cases matching the query plus the
// total match count. Filtering, sorting and paging all run in SQLite; associations
// are loaded only for the returned page. Folder ids that resolve to nothing yield
// an empty result rather than the whole library.
func (s *Store) ListTestCasesPage(q TestPageQuery) ([]*models.TestCase, int64, error) {
	var folderIDs []string
	if len(q.FolderIDs) > 0 {
		uDesc, err := s.folderDescendantClosure(q.FolderIDs)
		if err != nil {
			return nil, 0, err
		}
		if len(uDesc) == 0 {
			return []*models.TestCase{}, 0, nil
		}
		folderIDs = uDesc
	}

	var rows []*models.TestCase
	var total int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		build := func() *gorm.DB {
			t := tx.Model(&models.TestCase{})
			if len(folderIDs) > 0 {
				t = t.Where("test_cases.folder_id IN ?", folderIDs)
			}
			return applyTestPageFilters(t, q)
		}
		if err := build().Count(&total).Error; err != nil {
			return err
		}
		return build().
			Order(testPageOrder(q.Sort, q.Direction)).
			Limit(q.Limit).Offset(q.Offset).
			Find(&rows).Error
	})
	if err != nil {
		return nil, 0, err
	}

	if err := s.attachTestCaseAssociations(rows, false); err != nil {
		return nil, 0, err
	}
	s.enrichListRows(rows, true)
	return rows, total, nil
}

// ListTestCaseIDs returns the IDs of every test case matching the query's filters
// (ignoring limit/offset/sort), capped at maxIDs, plus the true match total. It
// backs the grid's "select all matching" action so a bulk operation can act on the
// whole filtered set rather than only the visible page.
func (s *Store) ListTestCaseIDs(q TestPageQuery, maxIDs int) ([]string, int64, error) {
	var folderIDs []string
	if len(q.FolderIDs) > 0 {
		uDesc, err := s.folderDescendantClosure(q.FolderIDs)
		if err != nil {
			return nil, 0, err
		}
		if len(uDesc) == 0 {
			return []string{}, 0, nil
		}
		folderIDs = uDesc
	}

	var ids []string
	var total int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		build := func() *gorm.DB {
			t := tx.Model(&models.TestCase{})
			if len(folderIDs) > 0 {
				t = t.Where("test_cases.folder_id IN ?", folderIDs)
			}
			return applyTestPageFilters(t, q)
		}
		if err := build().Count(&total).Error; err != nil {
			return err
		}
		sel := build().Order("test_cases.id ASC")
		if maxIDs > 0 {
			sel = sel.Limit(maxIDs)
		}
		return sel.Pluck("test_cases.id", &ids).Error
	})
	if err != nil {
		return nil, 0, err
	}
	return ids, total, nil
}

// folderDescendantClosure returns the de-duplicated descendant closure (including
// the folders themselves) of the given folder ids.
func (s *Store) folderDescendantClosure(folderIDs []string) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, fid := range folderIDs {
		if fid == "" {
			continue
		}
		descendants, err := s.GetFolderDescendants(fid)
		if err != nil {
			return nil, err
		}
		for _, d := range descendants {
			if !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
		}
	}
	return out, nil
}

func applyTestPageFilters(t *gorm.DB, q TestPageQuery) *gorm.DB {
	if q.Search != "" {
		n := strings.ToLower(q.Search)
		t = t.Where(
			"ttgo_icontains(test_cases.name, ?) OR ttgo_icontains(test_cases.id, ?) OR "+
				"EXISTS (SELECT 1 FROM suite_test_cases stc JOIN suites su ON su.id = stc.suite_id "+
				"WHERE stc.test_case_id = test_cases.id AND ttgo_icontains(su.name, ?))",
			n, n, n,
		)
	}
	if q.ID != "" {
		t = t.Where("ttgo_icontains(test_cases.id, ?)", strings.ToLower(q.ID))
	}
	if q.Name != "" {
		t = t.Where("ttgo_icontains(test_cases.name, ?)", strings.ToLower(q.Name))
	}
	if q.Description != "" {
		t = t.Where("ttgo_icontains(ttgo_striphtml(test_cases.description), ?)", strings.ToLower(q.Description))
	}
	if len(q.CategoryIDs) > 0 {
		t = t.Where("EXISTS (SELECT 1 FROM suite_test_cases stc WHERE stc.test_case_id = test_cases.id AND stc.suite_id IN ?)", q.CategoryIDs)
	}
	if q.StepsCount != "" {
		t = t.Where("instr(CAST((SELECT COUNT(*) FROM test_steps ts WHERE ts.test_case_id = test_cases.id) AS TEXT), ?) > 0", q.StepsCount)
	}
	if q.OpenDefects != "" {
		t = t.Where("instr(CAST((SELECT COUNT(DISTINCT d.id) FROM defect_links dl JOIN defects d ON d.id = dl.defect_id WHERE dl.test_case_id = test_cases.id AND d.status != 'closed') AS TEXT), ?) > 0", q.OpenDefects)
	}
	switch q.Reverification {
	case "yes":
		t = t.Where("test_cases.reverification_flagged = ?", true)
	case "no":
		t = t.Where("test_cases.reverification_flagged = ?", false)
	}
	if q.LinkedRequirement != "" {
		t = t.Where("EXISTS (SELECT 1 FROM requirement_test_case_links l JOIN requirements r ON r.id = l.requirement_id WHERE l.test_case_id = test_cases.id AND ttgo_icontains(r.identifier, ?))", strings.ToLower(q.LinkedRequirement))
	}
	if q.CreatedFrom != nil {
		t = t.Where("test_cases.created_at >= ?", *q.CreatedFrom)
	}
	if q.CreatedTo != nil {
		t = t.Where("test_cases.created_at < ?", *q.CreatedTo)
	}
	if q.UpdatedFrom != nil {
		t = t.Where("test_cases.updated_at >= ?", *q.UpdatedFrom)
	}
	if q.UpdatedTo != nil {
		t = t.Where("test_cases.updated_at < ?", *q.UpdatedTo)
	}
	return t
}
