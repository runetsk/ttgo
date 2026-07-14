package store

import (
	"fmt"
	"strings"

	"ttgo/pkg/tracker/aigen"
	"ttgo/pkg/tracker/models"
)

// duplicate-candidate FTS query: OR of quoted significant tokens.
var dupStopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true,
	"that": true, "this": true, "into": true, "when": true, "then": true,
}

func buildDupFTSQuery(name string) string {
	var toks []string
	for _, tok := range strings.Fields(aigen.NormalizeTestText(name)) {
		if len(tok) < 3 || dupStopwords[tok] {
			continue
		}
		toks = append(toks, fmt.Sprintf(`"%s"`, tok))
		if len(toks) == 8 {
			break
		}
	}
	return strings.Join(toks, " OR ")
}

// dupScored pairs a candidate with its linked-to-requirement flag for sorting.
type dupScored struct {
	cand   aigen.DuplicateCandidate
	linked bool
}

// SearchDuplicateCandidates finds existing test cases lexically similar to a
// draft name using the test_cases_fts index, ranking requirement-linked
// matches first (spec: "Duplicate detection" — FTS5 candidates, ranked).
func (s *Store) SearchDuplicateCandidates(draftName, requirementID string, limit int) ([]aigen.DuplicateCandidate, error) {
	if limit <= 0 {
		limit = aigen.MaxDuplicateCandidates
	}
	ftsQuery := buildDupFTSQuery(draftName)
	if ftsQuery == "" {
		return nil, nil
	}

	rows, err := s.db.Raw(`
		SELECT tc.id, tc.name, tc.folder_id,
		       CASE WHEN l.id IS NOT NULL THEN 1 ELSE 0 END AS linked
		FROM test_cases tc
		JOIN test_cases_fts fts ON tc.rowid = fts.rowid
		LEFT JOIN requirement_test_case_links l
		       ON l.test_case_id = tc.id AND l.requirement_id = ?
		WHERE test_cases_fts MATCH ?
		LIMIT 50`, requirementID, ftsQuery).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var all []dupScored
	for rows.Next() {
		var id, name, folderID string
		var linked int
		if err := rows.Scan(&id, &name, &folderID, &linked); err != nil {
			return nil, err
		}
		sim := aigen.TokenSimilarity(draftName, name)
		if sim < aigen.DupSimilarityThreshold {
			continue
		}
		reason := fmt.Sprintf("existing test %q shares %d%% of its name", name, int(sim*100+0.5))
		if linked == 1 {
			reason += "; linked to this requirement"
		}
		all = append(all, dupScored{
			cand: aigen.DuplicateCandidate{
				Kind: aigen.DupKindExisting, TestCaseID: id, FolderID: folderID,
				Name: name, Similarity: sim, Reason: reason,
			},
			linked: linked == 1,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// linked first, then similarity descending (insertion sort — lists are tiny)
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && lessDupCandidate(all[j-1], all[j]); j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
	out := make([]aigen.DuplicateCandidate, 0, limit)
	for _, sc := range all {
		out = append(out, sc.cand)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func lessDupCandidate(a, b dupScored) bool {
	if a.linked != b.linked {
		return !a.linked && b.linked
	}
	return a.cand.Similarity < b.cand.Similarity
}

// UpdateGenerationRunCoverage stores the run-level coverage report without
// touching any other run column.
func (s *Store) UpdateGenerationRunCoverage(runID, coverageJSON string) error {
	return s.db.Model(&models.AIGenerationRun{}).
		Where("id = ?", runID).
		Update("coverage_json", coverageJSON).Error
}
