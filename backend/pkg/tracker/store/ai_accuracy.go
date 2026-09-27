package store

import (
	"time"

	"gorm.io/gorm"
)

// AIAccuracyVerdictBucket is the agreement breakdown for one snapshotted AI verdict.
type AIAccuracyVerdictBucket struct {
	Verdict string  `json:"verdict"`
	Total   int     `json:"total"`
	Agreed  int     `json:"agreed"`
	Rate    float64 `json:"rate"`
}

// AIAccuracyConfidenceBucket is the agreement breakdown for one snapshotted confidence level.
type AIAccuracyConfidenceBucket struct {
	Confidence string  `json:"confidence"`
	Total      int     `json:"total"`
	Agreed     int     `json:"agreed"`
	Rate       float64 `json:"rate"`
}

// AIAccuracySplit separates direct predictions from dedup clones (spec B2). A clone carries its
// group representative's answer, so counting it as an independent prediction would weight one
// decision by the size of its group. Rows whose provenance was never recorded are in neither half.
type AIAccuracySplit struct {
	DirectTotal  int     `json:"direct_total"`
	DirectAgreed int     `json:"direct_agreed"`
	DirectRate   float64 `json:"direct_rate"`
	CloneTotal   int     `json:"clone_total"`
	CloneAgreed  int     `json:"clone_agreed"`
}

// AIAccuracyEngineBucket is one engine's agreement with its own confidence ladder. Ladders are
// per engine because generative confidence is self-reported while TypeSafe's is calibrated on
// the defect-type question; mixing them in one ladder would make both unreadable.
type AIAccuracyEngineBucket struct {
	Engine string  `json:"engine"`
	Total  int     `json:"total"`
	Agreed int     `json:"agreed"`
	Rate   float64 `json:"rate"`
	AIAccuracySplit
	ByConfidence []AIAccuracyConfidenceBucket `json:"by_confidence"`
}

// AIAccuracyCoverage is how often one engine decided at all (spec B3), over the representative
// analyses created in the window: a suggestion nobody triaged still counts here.
type AIAccuracyCoverage struct {
	Engine        string `json:"engine"`
	Analyses      int    `json:"analyses"`
	Decided       int    `json:"decided"`        // a decision with a suggestion
	Abstained     int    `json:"abstained"`      // a decision of verdict unknown or with the suggestion withheld
	Failed        int    `json:"failed"`         // no decision: the attempt failed
	NoExplanation int    `json:"no_explanation"` // a decision whose explanation is unavailable, unparseable or skipped
}

// AIFailureAnalysisAccuracy answers "how often did the AI's suggested defect_type match the
// human's triage decision": overall, per verdict, per confidence level and per engine, with the
// direct-prediction/clone split and per-engine coverage.
type AIFailureAnalysisAccuracy struct {
	Total         int     `json:"total"`
	Agreed        int     `json:"agreed"`
	AgreementRate float64 `json:"agreement_rate"`
	AIAccuracySplit
	// UnknownProvenance counts calibration rows decided before the split was recorded: in Total,
	// in neither half of the split, and never in a specific policy version.
	UnknownProvenance int `json:"unknown_provenance"`
	// PolicyVersion echoes the filter ("" = all versions). PolicyVersions lists every version
	// present in the window, ignoring the filter, so the UI's dropdown stays stable.
	PolicyVersion  string                       `json:"policy_version"`
	PolicyVersions []string                     `json:"policy_versions"`
	ByVerdict      []AIAccuracyVerdictBucket    `json:"by_verdict"`
	ByConfidence   []AIAccuracyConfidenceBucket `json:"by_confidence"`
	ByEngine       []AIAccuracyEngineBucket     `json:"by_engine"`
	Coverage       []AIAccuracyCoverage         `json:"coverage"`
}

// AccuracyFilter selects the calibration set: decisions at or after Since (UTC) and, when
// PolicyVersion is set, only suggestions snapshotted under that TypeSafe policy version.
type AccuracyFilter struct {
	Since         time.Time
	PolicyVersion string
}

// accuracyWhere is accuracyCalibrationFilter plus the optional policy-version predicate, with
// its arguments. Every calibration query uses it, so the breakdowns cannot drift apart.
func accuracyWhere(f AccuracyFilter) (string, []interface{}) {
	where, args := accuracyCalibrationFilter, []interface{}{f.Since}
	if f.PolicyVersion != "" {
		where += `
	  AND suggested_policy_version = ?`
		args = append(args, f.PolicyVersion)
	}
	return where, args
}

// accuracyCalibrationFilter defines the calibration set: the rows where a real human decision
// can be compared against a real AI suggestion. It is declared once and shared verbatim by
// every query below so the breakdowns can never drift apart from each other or from the total.
//
// Every exclusion is load-bearing — a wrong accuracy number is worse than no number:
//
//   - decided_at >= ? — the window is measured on the moment the human DECIDED, a column
//     written only by the triage snapshot. updated_at would be wrong here: it means "row last
//     touched" and is re-stamped by writes that are not decisions (an artifact or log edit, a
//     plain status change, the test-case delete cascade), any of which would drag an old
//     decision into a recent window. decided_at is written in UTC, and the handler's cutoff is
//     UTC, so the TEXT comparison SQLite performs is exact rather than offset-skewed.
//   - status IN ('FAIL','ERROR') — only failing results carry a triage decision, and a result can
//     be re-executed to PASS after being triaged. Filtering here makes the calibration set
//     self-enforcing instead of trusting every writer to have cleared the snapshot. Both failure
//     statuses are listed because both are triageable; this must stay in step with
//     models.IsFailureStatus, which the triage handlers gate on.
//   - a non-empty suggested_defect_type — the AI actually suggested something at the decision
//     moment. An empty snapshot means there was nothing to agree or disagree with.
//   - defect_type IN ('product_bug', 'automation_bug', 'system_issue') — a REAL human decision.
//     'to_investigate' is the auto-default stamped on every failure at ingest, i.e. "nobody has
//     triaged this yet". Counting untriaged rows as disagreements would tank the AI's score and
//     make the whole metric meaningless. Empty is excluded for the same reason.
//
// NULL is not-true under every predicate, so any pre-migration row fails safe (excluded).
//
// ERROR results belong to the set on the same terms as FAIL: the analyzer produces verdicts for
// both, and both expose the defect_type control that records a human decision. Pre-existing ERROR
// rows contribute nothing until genuinely triaged — they carry defect_type = "" (or the untriaged
// 'to_investigate'), which the last predicate excludes — so no backfill is needed.
const accuracyCalibrationFilter = `
	WHERE decided_at >= ?
	  AND status IN ('FAIL','ERROR')
	  AND suggested_defect_type != ''
	  AND defect_type IN ('product_bug', 'automation_bug', 'system_issue')`

// GetFailureAnalysisAccuracy is GetFailureAnalysisAccuracyFor over every policy version.
func (s *Store) GetFailureAnalysisAccuracy(since time.Time) (*AIFailureAnalysisAccuracy, error) {
	return s.GetFailureAnalysisAccuracyFor(AccuracyFilter{Since: since})
}

// GetFailureAnalysisAccuracyFor compares the AI's snapshotted suggestion against the human's
// triage decision for every calibration-set row the filter selects.
//
// Agreement is a pure comparison of two stored columns (suggested_defect_type vs defect_type),
// which is why the suggestion is snapshotted onto run_results at decision time: no join, and
// the record is immune to a later re-analysis changing the verdict.
//
// by_verdict groups on the snapshotted suggested_verdict rather than the mapped defect type,
// because the verdict -> defect_type mapping is lossy (flaky_test and test_data both map to
// automation_bug; environment and infrastructure both map to system_issue). Grouping on the
// mapped value would silently merge distinct verdicts and hide which one is actually wrong.
//
// Coverage reads run_result_analyses, whose created_at is written in server-local time, so its
// cutoff is Since converted to local time; comparing a UTC cutoff as TEXT would be skewed by the
// offset (the same trap decided_at avoids by being written in UTC).
func (s *Store) GetFailureAnalysisAccuracyFor(f AccuracyFilter) (*AIFailureAnalysisAccuracy, error) {
	out := &AIFailureAnalysisAccuracy{
		PolicyVersion:  f.PolicyVersion,
		PolicyVersions: []string{},
		ByVerdict:      []AIAccuracyVerdictBucket{},
		ByConfidence:   []AIAccuracyConfidenceBucket{},
		ByEngine:       []AIAccuracyEngineBucket{},
		Coverage:       []AIAccuracyCoverage{},
	}
	where, args := accuracyWhere(f)

	// One transaction for every breakdown: they are rendered side by side and the headline is
	// derived from them, so a triage write landing between two independent queries would show a
	// total that disagrees with the ladders next to it.
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(`
			SELECT suggested_verdict AS verdict,
			       COUNT(*) AS total,
			       SUM(CASE WHEN suggested_defect_type = defect_type THEN 1 ELSE 0 END) AS agreed
			FROM run_results`+where+`
			GROUP BY suggested_verdict
			ORDER BY total DESC, verdict ASC`, args...).Scan(&out.ByVerdict).Error; err != nil {
			return err
		}

		// Ordered high -> medium -> low so the calibration ladder reads as a descent: a clean
		// drop means confidence is trustworthy, a flat one means it is noise.
		if err := tx.Raw(`
			SELECT suggested_confidence AS confidence,
			       COUNT(*) AS total,
			       SUM(CASE WHEN suggested_defect_type = defect_type THEN 1 ELSE 0 END) AS agreed
			FROM run_results`+where+`
			GROUP BY suggested_confidence
			ORDER BY CASE suggested_confidence
			           WHEN 'high' THEN 0
			           WHEN 'medium' THEN 1
			           WHEN 'low' THEN 2
			           ELSE 3
			         END, confidence ASC`, args...).Scan(&out.ByConfidence).Error; err != nil {
			return err
		}

		// Per-engine ladders (spec §5): generative confidence is self-reported, TypeSafe's is
		// calibrated on the defect-type question, so mixing them into one ladder would be
		// meaningless. suggested_engine is blank on rows decided before engines existed; those
		// are grouped with 'generative' rather than forming their own silent bucket. The
		// direct/clone split rides along: suggested_is_clone is 0 for a direct prediction, 1 for a
		// clone and NULL when unknown, and NULL matches neither CASE.
		type engineRow struct {
			Engine, Confidence                                 string
			Total, Agreed                                      int
			DirectTotal, DirectAgreed, CloneTotal, CloneAgreed int
		}
		var rows []engineRow
		if err := tx.Raw(`
			SELECT COALESCE(NULLIF(suggested_engine, ''), 'generative') AS engine,
			       suggested_confidence AS confidence,
			       COUNT(*) AS total,
			       SUM(CASE WHEN suggested_defect_type = defect_type THEN 1 ELSE 0 END) AS agreed,
			       SUM(CASE WHEN suggested_is_clone = 0 THEN 1 ELSE 0 END) AS direct_total,
			       SUM(CASE WHEN suggested_is_clone = 0 AND suggested_defect_type = defect_type THEN 1 ELSE 0 END) AS direct_agreed,
			       SUM(CASE WHEN suggested_is_clone = 1 THEN 1 ELSE 0 END) AS clone_total,
			       SUM(CASE WHEN suggested_is_clone = 1 AND suggested_defect_type = defect_type THEN 1 ELSE 0 END) AS clone_agreed
			FROM run_results`+where+`
			GROUP BY engine, suggested_confidence
			ORDER BY engine ASC, CASE suggested_confidence WHEN 'high' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 ELSE 3 END`, args...).Scan(&rows).Error; err != nil {
			return err
		}
		byEngine := map[string]*AIAccuracyEngineBucket{}
		var order []string
		for _, r := range rows {
			b := byEngine[r.Engine]
			if b == nil {
				b = &AIAccuracyEngineBucket{Engine: r.Engine, ByConfidence: []AIAccuracyConfidenceBucket{}}
				byEngine[r.Engine] = b
				order = append(order, r.Engine)
			}
			b.Total += r.Total
			b.Agreed += r.Agreed
			b.DirectTotal += r.DirectTotal
			b.DirectAgreed += r.DirectAgreed
			b.CloneTotal += r.CloneTotal
			b.CloneAgreed += r.CloneAgreed
			b.ByConfidence = append(b.ByConfidence, AIAccuracyConfidenceBucket{Confidence: r.Confidence, Total: r.Total, Agreed: r.Agreed, Rate: accuracyRate(r.Agreed, r.Total)})
		}
		for _, e := range order {
			b := byEngine[e]
			b.Rate = accuracyRate(b.Agreed, b.Total)
			b.DirectRate = accuracyRate(b.DirectAgreed, b.DirectTotal)
			out.ByEngine = append(out.ByEngine, *b)
		}

		// Coverage (spec B3): representatives only; a clone repeats its representative's decision.
		covWhere := ` WHERE source_analysis_id IS NULL AND created_at >= ?`
		covArgs := []interface{}{f.Since.Local()}
		if f.PolicyVersion != "" {
			covWhere += ` AND policy_version = ?`
			covArgs = append(covArgs, f.PolicyVersion)
		}
		if err := tx.Raw(`
			SELECT COALESCE(NULLIF(engine, ''), 'generative') AS engine,
			       COUNT(*) AS analyses,
			       SUM(CASE WHEN decision_status = 'failed' THEN 1 ELSE 0 END) AS failed,
			       SUM(CASE WHEN decision_status != 'failed' AND (verdict = 'unknown' OR suggested_defect_type = '') THEN 1 ELSE 0 END) AS abstained,
			       SUM(CASE WHEN decision_status != 'failed' AND narrative_status IN ('unavailable', 'unparseable', 'skipped') THEN 1 ELSE 0 END) AS no_explanation
			FROM run_result_analyses`+covWhere+`
			GROUP BY 1
			ORDER BY 1`, covArgs...).Scan(&out.Coverage).Error; err != nil {
			return err
		}

		// Every version present in the window, ignoring the filter, newest first.
		allWhere, allArgs := accuracyWhere(AccuracyFilter{Since: f.Since})
		return tx.Raw(`
			SELECT v FROM (
			  SELECT suggested_policy_version AS v FROM run_results`+allWhere+` AND suggested_policy_version != ''
			  UNION
			  SELECT policy_version AS v FROM run_result_analyses
			   WHERE source_analysis_id IS NULL AND created_at >= ? AND policy_version != ''
			) ORDER BY v DESC`, append(allArgs, f.Since.Local())...).Scan(&out.PolicyVersions).Error
	})
	if err != nil {
		return nil, err
	}
	if out.PolicyVersions == nil {
		out.PolicyVersions = []string{}
	}
	if out.Coverage == nil {
		out.Coverage = []AIAccuracyCoverage{}
	}

	for i := range out.ByVerdict {
		b := &out.ByVerdict[i]
		b.Rate = accuracyRate(b.Agreed, b.Total)
		// Derive the headline from the buckets so the total can never disagree with the
		// breakdown that is displayed next to it.
		out.Total += b.Total
		out.Agreed += b.Agreed
	}
	for i := range out.ByConfidence {
		b := &out.ByConfidence[i]
		b.Rate = accuracyRate(b.Agreed, b.Total)
	}
	for _, b := range out.ByEngine {
		out.DirectTotal += b.DirectTotal
		out.DirectAgreed += b.DirectAgreed
		out.CloneTotal += b.CloneTotal
		out.CloneAgreed += b.CloneAgreed
	}
	out.AgreementRate = accuracyRate(out.Agreed, out.Total)
	out.DirectRate = accuracyRate(out.DirectAgreed, out.DirectTotal)
	out.UnknownProvenance = out.Total - out.DirectTotal - out.CloneTotal
	for i := range out.Coverage {
		c := &out.Coverage[i]
		c.Decided = c.Analyses - c.Failed - c.Abstained
	}
	return out, nil
}

// accuracyRate returns agreed/total, or 0 for an empty bucket (the day-one case).
func accuracyRate(agreed, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(agreed) / float64(total)
}
