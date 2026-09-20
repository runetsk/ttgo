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

// AIAccuracyEngineBucket is one engine's agreement with its own confidence ladder. Ladders are
// per engine because generative confidence is self-reported while TypeSafe's is calibrated on
// the defect-type question; mixing them in one ladder would make both unreadable.
type AIAccuracyEngineBucket struct {
	Engine       string                       `json:"engine"`
	Total        int                          `json:"total"`
	Agreed       int                          `json:"agreed"`
	Rate         float64                      `json:"rate"`
	ByConfidence []AIAccuracyConfidenceBucket `json:"by_confidence"`
}

// AIFailureAnalysisAccuracy answers "how often did the AI's suggested defect_type match the
// human's triage decision" — overall, per verdict, per confidence level, and per engine.
type AIFailureAnalysisAccuracy struct {
	Total         int                          `json:"total"`
	Agreed        int                          `json:"agreed"`
	AgreementRate float64                      `json:"agreement_rate"`
	ByVerdict     []AIAccuracyVerdictBucket    `json:"by_verdict"`
	ByConfidence  []AIAccuracyConfidenceBucket `json:"by_confidence"`
	ByEngine      []AIAccuracyEngineBucket     `json:"by_engine"`
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

// GetFailureAnalysisAccuracy compares the AI's snapshotted suggestion against the human's
// triage decision for every calibration-set row decided at or after `since`.
//
// Agreement is a pure comparison of two stored columns (suggested_defect_type vs defect_type),
// which is why the suggestion is snapshotted onto run_results at decision time: no join, and
// the record is immune to a later re-analysis changing the verdict.
//
// by_verdict groups on the snapshotted suggested_verdict rather than the mapped defect type,
// because the verdict -> defect_type mapping is lossy (flaky_test and test_data both map to
// automation_bug; environment and infrastructure both map to system_issue). Grouping on the
// mapped value would silently merge distinct verdicts and hide which one is actually wrong.
func (s *Store) GetFailureAnalysisAccuracy(since time.Time) (*AIFailureAnalysisAccuracy, error) {
	out := &AIFailureAnalysisAccuracy{
		ByVerdict:    []AIAccuracyVerdictBucket{},
		ByConfidence: []AIAccuracyConfidenceBucket{},
		ByEngine:     []AIAccuracyEngineBucket{},
	}

	// One transaction for both breakdowns: they are rendered side by side and the headline is
	// derived from by_verdict, so a triage write landing between two independent queries would
	// show a total that disagrees with the confidence ladder next to it.
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Raw(`
			SELECT suggested_verdict AS verdict,
			       COUNT(*) AS total,
			       SUM(CASE WHEN suggested_defect_type = defect_type THEN 1 ELSE 0 END) AS agreed
			FROM run_results`+accuracyCalibrationFilter+`
			GROUP BY suggested_verdict
			ORDER BY total DESC, verdict ASC`, since).Scan(&out.ByVerdict).Error; err != nil {
			return err
		}

		// Ordered high -> medium -> low so the calibration ladder reads as a descent: a clean
		// drop means confidence is trustworthy, a flat one means it is noise.
		if err := tx.Raw(`
			SELECT suggested_confidence AS confidence,
			       COUNT(*) AS total,
			       SUM(CASE WHEN suggested_defect_type = defect_type THEN 1 ELSE 0 END) AS agreed
			FROM run_results`+accuracyCalibrationFilter+`
			GROUP BY suggested_confidence
			ORDER BY CASE suggested_confidence
			           WHEN 'high' THEN 0
			           WHEN 'medium' THEN 1
			           WHEN 'low' THEN 2
			           ELSE 3
			         END, confidence ASC`, since).Scan(&out.ByConfidence).Error; err != nil {
			return err
		}

		// Per-engine ladders (spec §5): generative confidence is self-reported, TypeSafe's is
		// calibrated on the defect-type question, so mixing them into one ladder would be
		// meaningless. suggested_engine is blank on rows decided before engines existed; those
		// are grouped with 'generative' rather than forming their own silent bucket.
		type engineRow struct {
			Engine, Confidence string
			Total, Agreed      int
		}
		var rows []engineRow
		if err := tx.Raw(`
			SELECT COALESCE(NULLIF(suggested_engine, ''), 'generative') AS engine,
			       suggested_confidence AS confidence,
			       COUNT(*) AS total,
			       SUM(CASE WHEN suggested_defect_type = defect_type THEN 1 ELSE 0 END) AS agreed
			FROM run_results`+accuracyCalibrationFilter+`
			GROUP BY engine, suggested_confidence
			ORDER BY engine ASC, CASE suggested_confidence WHEN 'high' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 ELSE 3 END`, since).Scan(&rows).Error; err != nil {
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
			b.ByConfidence = append(b.ByConfidence, AIAccuracyConfidenceBucket{Confidence: r.Confidence, Total: r.Total, Agreed: r.Agreed, Rate: accuracyRate(r.Agreed, r.Total)})
		}
		for _, e := range order {
			b := byEngine[e]
			b.Rate = accuracyRate(b.Agreed, b.Total)
			out.ByEngine = append(out.ByEngine, *b)
		}
		return nil
	})
	if err != nil {
		return nil, err
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
	out.AgreementRate = accuracyRate(out.Agreed, out.Total)
	return out, nil
}

// accuracyRate returns agreed/total, or 0 for an empty bucket (the day-one case).
func accuracyRate(agreed, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(agreed) / float64(total)
}
