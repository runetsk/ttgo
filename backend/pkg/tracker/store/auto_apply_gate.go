package store

import (
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
)

// AutoApplyGate grades TypeSafe's direct defect-type suggestions at or above minConfidence (0..1)
// against the people who triaged them (spec §3.4, R4): decisions of the last
// failureanalysis.AutoApplyGateWindowDays (decided_at, compared as instants in UTC), a conclusive
// human label on a failing result, a suggestion snapshotted from engine 'typesafe' — not
// 'typesafe-derived', whose confidence is the verdict's — on a direct prediction
// (suggested_is_clone = 0; NULL provenance is excluded), under the current question policies
// only, so a policy bump closes the gate until enough new decisions are graded.
// suggested_confidence_score is the defect-type question's confidence on typesafe rows.
// R10: the label must be a person's (defect_type_source != 'ai': 'human' from explicit triage,
// ” for decisions recorded before Wave 3) and the write that made it must not have replaced an
// AI label (suggested_auto_applied = 0) — a Confirm, or a correction of a label auto-apply showed
// first, never grades auto-apply.
func (s *Store) AutoApplyGate(minConfidence float64) (failureanalysis.GateStatus, error) {
	policies := []string{failureanalysis.PolicyVersionNoExamples, failureanalysis.PolicyVersionWithExamples}
	since := time.Now().UTC().AddDate(0, 0, -failureanalysis.AutoApplyGateWindowDays)
	var row struct {
		Graded int
		Agreed int
	}
	err := s.db.Raw(`
		SELECT COUNT(*) AS graded,
		       COALESCE(SUM(CASE WHEN suggested_defect_type = defect_type THEN 1 ELSE 0 END), 0) AS agreed
		  FROM run_results
		 WHERE decided_at IS NOT NULL
		   AND julianday(decided_at) >= julianday(?)
		   AND status IN ('FAIL','ERROR')
		   AND defect_type IN ('product_bug','automation_bug','system_issue')
		   AND defect_type_source != 'ai'
		   AND suggested_auto_applied = 0
		   AND suggested_defect_type != ''
		   AND suggested_engine = ?
		   AND suggested_is_clone = 0
		   AND suggested_policy_version IN ?
		   AND suggested_confidence_score >= ?`,
		since, models.AnalysisEngineTypeSafe, policies, minConfidence).Scan(&row).Error
	if err != nil {
		return failureanalysis.GateStatus{}, err
	}
	return failureanalysis.NewGateStatus(row.Graded, row.Agreed, minConfidence, policies), nil
}
