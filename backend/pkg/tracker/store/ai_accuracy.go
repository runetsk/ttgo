package store

import "time"

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

// AIFailureAnalysisAccuracy answers "how often did the AI's suggested defect_type match the
// human's triage decision" — overall, per verdict, and per confidence level.
type AIFailureAnalysisAccuracy struct {
	Total         int                          `json:"total"`
	Agreed        int                          `json:"agreed"`
	AgreementRate float64                      `json:"agreement_rate"`
	ByVerdict     []AIAccuracyVerdictBucket    `json:"by_verdict"`
	ByConfidence  []AIAccuracyConfidenceBucket `json:"by_confidence"`
}

// accuracyCalibrationFilter defines the calibration set: the rows where a real human decision
// can be compared against a real AI suggestion. It is declared once and shared verbatim by
// every query below so the breakdowns can never drift apart from each other or from the total.
//
// Both exclusions are load-bearing — a wrong accuracy number is worse than no number:
//
//   - a non-empty suggested_defect_type — the AI actually suggested something at the decision
//     moment. An empty snapshot means there was nothing to agree or disagree with.
//   - defect_type IN ('product_bug', 'automation_bug', 'system_issue') — a REAL human decision.
//     'to_investigate' is the auto-default stamped on every FAIL at ingest, i.e. "nobody has
//     triaged this yet". Counting untriaged rows as disagreements would tank the AI's score and
//     make the whole metric meaningless. Empty is excluded for the same reason.
//
// NULL is not-true under both predicates, so any pre-migration row fails safe (excluded).
//
// The window is on updated_at — the moment the human decided — not created_at, which is when
// the result was first ingested.
const accuracyCalibrationFilter = `
	WHERE updated_at >= ?
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
	}

	if err := s.db.Raw(`
		SELECT suggested_verdict AS verdict,
		       COUNT(*) AS total,
		       SUM(CASE WHEN suggested_defect_type = defect_type THEN 1 ELSE 0 END) AS agreed
		FROM run_results`+accuracyCalibrationFilter+`
		GROUP BY suggested_verdict
		ORDER BY total DESC, verdict ASC`, since).Scan(&out.ByVerdict).Error; err != nil {
		return nil, err
	}

	// Ordered high -> medium -> low so the calibration ladder reads as a descent: a clean
	// drop means confidence is trustworthy, a flat one means it is noise.
	if err := s.db.Raw(`
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
