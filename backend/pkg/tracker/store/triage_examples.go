package store

import (
	"ttgo/pkg/tracker/failureanalysis"
)

// TriageExample and TriageExampleFilter are declared in failureanalysis (which must not import
// store) and aliased here, so *Store satisfies failureanalysis.EnrichmentSource.
type (
	TriageExample       = failureanalysis.TriageExample
	TriageExampleFilter = failureanalysis.TriageExampleFilter
)

// triageExamplePoolFactor sizes the ranked candidate pool the correction/agreement balance picks
// from: limit x this many rows, so a limit of 8 considers the 32 best-ranked decisions.
const triageExamplePoolFactor = 4

type triageExampleRow struct {
	ID                  string
	ErrorMessage        string
	FailureType         string
	SuggestedDefectType string
	DefectType          string
}

// ListTriageExamples returns past human triage decisions to show the engines as few-shot examples.
//
// The candidate set is the accuracy calibration set (accuracyCalibrationFilter's predicates:
// failing status, a non-empty suggestion, a conclusive human defect type, a decision instant),
// narrowed to known direct predictions (suggested_is_clone = 0; NULL is unknown provenance and is
// excluded, like a clone, whose suggestion was copied from another result). The analyzed run is
// excluded. Decisions count only if made before the analyzed failure happened and within the
// lookback. Both bounds compare instants through julianday: decided_at is written in UTC by the
// triage snapshot, but a TEXT comparison against any other offset would be skewed.
//
// Ranking: the same test case first, then the same failure type, then the newest decision. Up to
// f.Limit are returned, at most ceil(limit/2) of one kind (corrections or agreements) while the
// other kind has candidates left; see balanceExamples.
func (s *Store) ListTriageExamples(f failureanalysis.TriageExampleFilter) ([]failureanalysis.TriageExample, error) {
	if f.Limit <= 0 {
		return nil, nil
	}
	var rows []triageExampleRow
	err := s.db.Raw(`
		SELECT id, error_message, failure_type, suggested_defect_type, defect_type
		  FROM run_results
		 WHERE status IN ('FAIL','ERROR')
		   AND suggested_defect_type != ''
		   AND defect_type IN ('product_bug','automation_bug','system_issue')
		   AND decided_at IS NOT NULL
		   AND suggested_is_clone = 0
		   AND test_run_id != ?
		   AND julianday(decided_at) < julianday(?)
		   AND julianday(decided_at) >= julianday(?)
		 ORDER BY CASE WHEN test_case_id = ? THEN 0 ELSE 1 END,
		          CASE WHEN ? != '' AND failure_type = ? THEN 0 ELSE 1 END,
		          julianday(decided_at) DESC,
		          id
		 LIMIT ?`,
		f.ExcludeRunID, f.Before.UTC(), f.Since.UTC(), f.TestCaseID, f.FailureType, f.FailureType,
		f.Limit*triageExamplePoolFactor).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	ranked := make([]TriageExample, 0, len(rows))
	for _, r := range rows {
		ranked = append(ranked, TriageExample{ResultID: r.ID, ErrorMessage: r.ErrorMessage, FailureType: r.FailureType,
			SuggestedDefectType: r.SuggestedDefectType, HumanDefectType: r.DefectType,
			Corrected: r.SuggestedDefectType != r.DefectType})
	}
	return balanceExamples(ranked, f.Limit), nil
}

// balanceExamples picks up to limit examples in rank order, at most ceil(limit/2) corrections and
// ceil(limit/2) agreements, then fills any remaining room from the skipped ones in rank order, so
// a pool with only one kind still fills the limit. The result keeps rank order.
func balanceExamples(ranked []TriageExample, limit int) []TriageExample {
	if len(ranked) <= limit {
		return ranked
	}
	maxEach := (limit + 1) / 2
	picked := make([]bool, len(ranked))
	n, corrections, agreements := 0, 0, 0
	for i, e := range ranked {
		if n == limit {
			break
		}
		switch {
		case e.Corrected && corrections < maxEach:
			corrections++
		case !e.Corrected && agreements < maxEach:
			agreements++
		default:
			continue
		}
		picked[i] = true
		n++
	}
	for i := range ranked {
		if n == limit {
			break
		}
		if !picked[i] {
			picked[i] = true
			n++
		}
	}
	out := make([]TriageExample, 0, limit)
	for i, e := range ranked {
		if picked[i] {
			out = append(out, e)
		}
	}
	return out
}

// ListCategoryNamesByTestCase returns the names of the categories (suites) a test case belongs
// to, alphabetically.
func (s *Store) ListCategoryNamesByTestCase(tcID string) ([]string, error) {
	var names []string
	err := s.db.Raw(`
		SELECT s.name FROM suites s
		  JOIN suite_test_cases stc ON stc.suite_id = s.id
		 WHERE stc.test_case_id = ?
		 ORDER BY s.name`, tcID).Scan(&names).Error
	return names, err
}
