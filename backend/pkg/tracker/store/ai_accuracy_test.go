package store

import (
	"fmt"
	"testing"
	"time"

	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// calibrationRow describes one run_result to seed for the accuracy tests.
type calibrationRow struct {
	verdict    string        // snapshotted suggested_verdict ("" = the AI made no suggestion)
	suggested  string        // snapshotted suggested_defect_type
	confidence string        // snapshotted suggested_confidence
	defectType string        // the human's triage decision (or the untriaged auto-default)
	decidedAgo time.Duration // how long ago decided_at says the decision was made
}

func seedCalibrationRows(t *testing.T, s *Store, rows []calibrationRow) {
	t.Helper()
	runID := seedRun(t, s)
	for i, r := range rows {
		// decided_at is written in UTC, exactly as the triage handlers write it: SQLite stores
		// time as TEXT carrying the writer's offset and compares it as TEXT, so seeding a local
		// timestamp here would only prove that a local-vs-UTC comparison is off by the offset.
		decided := time.Now().UTC().Add(-r.decidedAgo)
		rr := &models.RunResult{
			TestRunID: runID, TestNameSnapshot: fmt.Sprintf("t%d", i),
			AttemptNumber: i + 1, Status: models.StatusFail, ErrorMessage: "boom",
			DefectType:          r.defectType,
			SuggestedVerdict:    r.verdict,
			SuggestedDefectType: r.suggested,
			SuggestedConfidence: r.confidence,
			DecidedAt:           &decided,
		}
		require.NoError(t, s.AddRunResult(rr))
	}
}

// last30d matches what the handler passes: a UTC cutoff against UTC-written decided_at values,
// so the TEXT comparison SQLite performs is exact and the boundary means what it says.
func last30d() time.Time { return time.Now().UTC().AddDate(0, 0, -30) }

// TestAccuracyExcludesUntriagedToInvestigate is the keystone guard. 'to_investigate' is the
// auto-default stamped on every FAIL at ingest — it means "nobody has looked at this yet", not
// "the human disagreed". Counting it would tank the rate and make the metric worse than useless.
func TestAccuracyExcludesUntriagedToInvestigate(t *testing.T) {
	s := newTestStore(t)
	rows := []calibrationRow{
		// Three real decisions that agreed with the AI.
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", time.Hour},
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", time.Hour},
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", time.Hour},
		// One real decision that overrode the AI.
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "automation_bug", time.Hour},
		// Five untriaged rows sitting at the auto-default — MUST NOT count either way.
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "to_investigate", time.Hour},
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "to_investigate", time.Hour},
		{models.VerdictFlakyTest, "automation_bug", models.ConfidenceLow, "to_investigate", time.Hour},
		{models.VerdictEnvironment, "system_issue", models.ConfidenceMedium, "to_investigate", time.Hour},
		{models.VerdictTestData, "automation_bug", models.ConfidenceLow, "to_investigate", time.Hour},
	}
	seedCalibrationRows(t, s, rows)

	got, err := s.GetFailureAnalysisAccuracy(last30d())
	require.NoError(t, err)

	// Counting the five untriaged rows as disagreements would give 3/9 = 0.333.
	require.Equal(t, 4, got.Total, "only the four genuinely triaged rows belong to the calibration set")
	require.Equal(t, 3, got.Agreed)
	require.InDelta(t, 0.75, got.AgreementRate, 1e-9)

	// The untriaged rows must not leak into the breakdowns either.
	var bucketed int
	for _, b := range got.ByVerdict {
		bucketed += b.Total
	}
	require.Equal(t, 4, bucketed, "by_verdict must cover exactly the calibration set")
	require.Len(t, got.ByVerdict, 1, "flaky_test/environment/test_data rows were all untriaged")
	require.Equal(t, models.VerdictProductBug, got.ByVerdict[0].Verdict)
}

// TestAccuracyExcludesRowsWithoutSuggestion covers the other half of the rule: a human decision
// with no AI suggestion behind it is not a disagreement, there was simply nothing to compare.
func TestAccuracyExcludesRowsWithoutSuggestion(t *testing.T) {
	s := newTestStore(t)
	seedCalibrationRows(t, s, []calibrationRow{
		// Real decisions with a real suggestion: one agreed, one overridden.
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", time.Hour},
		{models.VerdictFlakyTest, "automation_bug", models.ConfidenceHigh, "system_issue", time.Hour},
		// Triaged, but the AI suggested nothing (unknown verdict maps to "").
		{models.VerdictUnknown, "", models.ConfidenceLow, "product_bug", time.Hour},
		{models.VerdictUnknown, "", models.ConfidenceLow, "system_issue", time.Hour},
		// No suggestion and no decision — excluded twice over.
		{"", "", "", "", time.Hour},
	})

	got, err := s.GetFailureAnalysisAccuracy(last30d())
	require.NoError(t, err)
	require.Equal(t, 2, got.Total)
	require.Equal(t, 1, got.Agreed)
	require.InDelta(t, 0.5, got.AgreementRate, 1e-9)
}

// TestAccuracyByVerdictSeparatesVerdictsSharingADefectType proves the third snapshot column is
// earning its keep: flaky_test and test_data both map to automation_bug, so grouping on the
// mapped value would silently merge two distinct verdicts into one meaningless bucket.
func TestAccuracyByVerdictSeparatesVerdictsSharingADefectType(t *testing.T) {
	s := newTestStore(t)
	seedCalibrationRows(t, s, []calibrationRow{
		// flaky_test: the AI was right both times.
		{models.VerdictFlakyTest, "automation_bug", models.ConfidenceHigh, "automation_bug", time.Hour},
		{models.VerdictFlakyTest, "automation_bug", models.ConfidenceHigh, "automation_bug", time.Hour},
		// test_data: same suggested defect type, but the AI was wrong both times.
		{models.VerdictTestData, "automation_bug", models.ConfidenceHigh, "product_bug", time.Hour},
		{models.VerdictTestData, "automation_bug", models.ConfidenceHigh, "product_bug", time.Hour},
	})

	got, err := s.GetFailureAnalysisAccuracy(last30d())
	require.NoError(t, err)
	require.Equal(t, 4, got.Total)
	require.Equal(t, 2, got.Agreed)

	byVerdict := map[string]AIAccuracyVerdictBucket{}
	for _, b := range got.ByVerdict {
		byVerdict[b.Verdict] = b
	}
	// Grouping on suggested_defect_type would collapse these into one 4-row, 50% bucket.
	require.Len(t, got.ByVerdict, 2, "flaky_test and test_data must stay distinct")

	flaky := byVerdict[models.VerdictFlakyTest]
	require.Equal(t, 2, flaky.Total)
	require.Equal(t, 2, flaky.Agreed)
	require.InDelta(t, 1.0, flaky.Rate, 1e-9)

	testData := byVerdict[models.VerdictTestData]
	require.Equal(t, 2, testData.Total)
	require.Equal(t, 0, testData.Agreed)
	require.InDelta(t, 0.0, testData.Rate, 1e-9)
}

// TestAccuracyByConfidenceBuckets checks the calibration ladder: exact per-bucket rates, ordered
// high -> medium -> low so a descent (or a damning flat line) reads directly off the response.
func TestAccuracyByConfidenceBuckets(t *testing.T) {
	s := newTestStore(t)
	rows := []calibrationRow{}
	// high: 3 of 4 agreed = 0.75
	for i := 0; i < 3; i++ {
		rows = append(rows, calibrationRow{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", time.Hour})
	}
	rows = append(rows, calibrationRow{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "system_issue", time.Hour})
	// medium: 1 of 2 agreed = 0.5
	rows = append(rows,
		calibrationRow{models.VerdictFlakyTest, "automation_bug", models.ConfidenceMedium, "automation_bug", time.Hour},
		calibrationRow{models.VerdictFlakyTest, "automation_bug", models.ConfidenceMedium, "product_bug", time.Hour},
	)
	// low: 0 of 2 agreed = 0.0
	rows = append(rows,
		calibrationRow{models.VerdictEnvironment, "system_issue", models.ConfidenceLow, "product_bug", time.Hour},
		calibrationRow{models.VerdictEnvironment, "system_issue", models.ConfidenceLow, "automation_bug", time.Hour},
	)
	// An untriaged high-confidence row must not inflate the top of the ladder.
	rows = append(rows, calibrationRow{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "to_investigate", time.Hour})
	seedCalibrationRows(t, s, rows)

	got, err := s.GetFailureAnalysisAccuracy(last30d())
	require.NoError(t, err)
	require.Equal(t, 8, got.Total)
	require.Equal(t, 4, got.Agreed)
	require.InDelta(t, 0.5, got.AgreementRate, 1e-9)

	require.Len(t, got.ByConfidence, 3)
	require.Equal(t, models.ConfidenceHigh, got.ByConfidence[0].Confidence, "ladder runs high -> medium -> low")
	require.Equal(t, 4, got.ByConfidence[0].Total)
	require.InDelta(t, 0.75, got.ByConfidence[0].Rate, 1e-9)

	require.Equal(t, models.ConfidenceMedium, got.ByConfidence[1].Confidence)
	require.Equal(t, 2, got.ByConfidence[1].Total)
	require.InDelta(t, 0.5, got.ByConfidence[1].Rate, 1e-9)

	require.Equal(t, models.ConfidenceLow, got.ByConfidence[2].Confidence)
	require.Equal(t, 2, got.ByConfidence[2].Total)
	require.Equal(t, 0, got.ByConfidence[2].Agreed)
	require.InDelta(t, 0.0, got.ByConfidence[2].Rate, 1e-9)
}

// TestAccuracyWindowBoundaryIsExact pins the edge to the hour rather than the day. Every other
// window test uses day-sized margins, which would still pass if the comparison were skewed by a
// timezone offset — this one fails if it is off by more than an hour in either direction.
func TestAccuracyWindowBoundaryIsExact(t *testing.T) {
	s := newTestStore(t)
	day := 24 * time.Hour
	seedCalibrationRows(t, s, []calibrationRow{
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", 30*day - time.Hour}, // just inside
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", 30*day + time.Hour}, // just outside
	})

	got, err := s.GetFailureAnalysisAccuracy(last30d())
	require.NoError(t, err)
	require.Equal(t, 1, got.Total, "a decision an hour past the cutoff is outside the window")
}

// TestAccuracyIgnoresLaterUnrelatedWrites is why decided_at exists at all. updated_at means "row
// last touched" and is re-stamped by writes that are not decisions — an artifact edit, a plain
// status change, or the test-case delete cascade NULLing test_case_id across every historical
// result. Windowing on it would drag a years-old decision into the last-30-days figure.
func TestAccuracyIgnoresLaterUnrelatedWrites(t *testing.T) {
	s := newTestStore(t)
	day := 24 * time.Hour
	seedCalibrationRows(t, s, []calibrationRow{
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", 400 * day},
	})
	// Touch every row the way an unrelated write would: updated_at moves to now, decided_at does not.
	require.NoError(t, s.db.Exec(`UPDATE run_results SET updated_at = ?, log_text = 'attached later'`,
		time.Now()).Error)

	got, err := s.GetFailureAnalysisAccuracy(last30d())
	require.NoError(t, err)
	require.Equal(t, 0, got.Total, "touching a row is not deciding it")
}

// TestAccuracyExcludesRowsNoLongerFailing makes the calibration set self-enforcing: a triaged
// failure that is later re-executed to PASS is no longer a failure anyone decided on, and the
// filter must drop it without depending on every writer having cleared the snapshot first.
func TestAccuracyExcludesRowsNoLongerFailing(t *testing.T) {
	s := newTestStore(t)
	seedCalibrationRows(t, s, []calibrationRow{
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", time.Hour},
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", time.Hour},
	})
	require.NoError(t, s.db.Exec(`UPDATE run_results SET status = ? WHERE test_name_snapshot = 't1'`,
		models.StatusPass).Error)

	got, err := s.GetFailureAnalysisAccuracy(last30d())
	require.NoError(t, err)
	require.Equal(t, 1, got.Total, "only rows that are still FAIL carry a failure triage decision")
}

// TestAccuracyCountsTriagedErrorRows is the other side of that self-enforcing filter. ERROR is a
// failure: the analyzer has always produced verdicts for ERROR results, and a human triaging one
// makes exactly the decision they would on a FAIL. Filtering on status = 'FAIL' silently discarded
// every one of those decisions, which is why calibration volume never grew.
//
// The seeded set deliberately mixes in the two rows that must still be excluded, so a filter that
// simply dropped the status predicate would fail here rather than pass by luck.
func TestAccuracyCountsTriagedErrorRows(t *testing.T) {
	s := newTestStore(t)
	seedCalibrationRows(t, s, []calibrationRow{
		// t0 stays FAIL — a real decision that agreed.
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", time.Hour},
		// t1, t2 become ERROR — real decisions, one agreed and one overridden.
		{models.VerdictEnvironment, "system_issue", models.ConfidenceHigh, "system_issue", time.Hour},
		{models.VerdictFlakyTest, "automation_bug", models.ConfidenceLow, "product_bug", time.Hour},
		// t3 becomes ERROR but sits at the untriaged auto-default — still excluded.
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "to_investigate", time.Hour},
		// t4 becomes SKIP — a real decision, but not on a failure, so still excluded.
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", time.Hour},
	})
	require.NoError(t, s.db.Exec(`UPDATE run_results SET status = ? WHERE test_name_snapshot IN ('t1','t2','t3')`,
		models.StatusError).Error)
	require.NoError(t, s.db.Exec(`UPDATE run_results SET status = ? WHERE test_name_snapshot = 't4'`,
		models.StatusSkip).Error)

	got, err := s.GetFailureAnalysisAccuracy(last30d())
	require.NoError(t, err)
	require.Equal(t, 3, got.Total, "one FAIL plus the two genuinely triaged ERROR rows")
	require.Equal(t, 2, got.Agreed)

	// The ERROR rows must reach the breakdowns too, not just the headline.
	byVerdict := map[string]int{}
	var bucketed int
	for _, b := range got.ByVerdict {
		byVerdict[b.Verdict] = b.Total
		bucketed += b.Total
	}
	require.Equal(t, 3, bucketed, "by_verdict must cover exactly the calibration set")
	require.Equal(t, 1, byVerdict[models.VerdictEnvironment], "a triaged ERROR row belongs in its verdict bucket")
	require.Equal(t, 1, byVerdict[models.VerdictFlakyTest])
}

// TestAccuracyWindowsOnDecisionTime pins the window to decided_at (when the human decided).
func TestAccuracyWindowsOnDecisionTime(t *testing.T) {
	s := newTestStore(t)
	day := 24 * time.Hour
	seedCalibrationRows(t, s, []calibrationRow{
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", 2 * day},
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "system_issue", 29 * day},
		// Decided outside the 30-day window.
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", 31 * day},
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "product_bug", 400 * day},
	})

	got, err := s.GetFailureAnalysisAccuracy(last30d())
	require.NoError(t, err)
	require.Equal(t, 2, got.Total, "only decisions inside the window count")
	require.Equal(t, 1, got.Agreed)

	// Widening the window pulls the older decisions back in.
	all, err := s.GetFailureAnalysisAccuracy(time.Now().UTC().AddDate(0, 0, -500))
	require.NoError(t, err)
	require.Equal(t, 4, all.Total)
	require.Equal(t, 3, all.Agreed)
}

// TestAccuracyEmptySet is the day-one case: no crash, no divide-by-zero, and empty (not nil)
// slices so the JSON carries [] rather than null.
func TestAccuracyEmptySet(t *testing.T) {
	s := newTestStore(t)
	seedCalibrationRows(t, s, []calibrationRow{
		{models.VerdictProductBug, "product_bug", models.ConfidenceHigh, "to_investigate", time.Hour},
	})

	got, err := s.GetFailureAnalysisAccuracy(last30d())
	require.NoError(t, err)
	require.Equal(t, 0, got.Total)
	require.Equal(t, 0, got.Agreed)
	require.Equal(t, 0.0, got.AgreementRate)
	require.NotNil(t, got.ByVerdict)
	require.NotNil(t, got.ByConfidence)
	require.Empty(t, got.ByVerdict)
	require.Empty(t, got.ByConfidence)
}
