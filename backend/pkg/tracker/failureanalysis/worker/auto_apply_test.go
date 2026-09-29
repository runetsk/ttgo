package worker

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

// resultsBC records the result rows the worker republishes, on top of the analysis events.
type resultsBC struct {
	*recordingBC
	mu   sync.Mutex
	rows map[string]models.RunResult
}

func newResultsBC() *resultsBC {
	return &resultsBC{recordingBC: &recordingBC{}, rows: map[string]models.RunResult{}}
}

func (b *resultsBC) BroadcastRunResultsUpdated(_ *models.TestRun, rows []*models.RunResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, r := range rows {
		b.rows[r.ID] = *r
	}
}

func (b *resultsBC) source(id string) (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	r, ok := b.rows[id]
	return r.DefectTypeSource, ok
}

func autoApplyJobSettings(t *testing.T, s *store.Store) {
	t.Helper()
	_, err := s.UpdateFailureAnalysisSettings(&models.AIFailureAnalysisSettings{
		MaxAnalysesPerRun: 10, DedupEnabled: true, RedactionEnabled: false,
		PromptTemplate: failureanalysis.DefaultPromptTemplate, ParallelGroups: 1,
	})
	require.NoError(t, err)
}

func failingIDs(t *testing.T, s *store.Store, runID string) []string {
	t.Helper()
	rows, err := s.ListLatestFailingResults(runID)
	require.NoError(t, err)
	ids := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return ids
}

func resultLabel(t *testing.T, s *store.Store, id string) *models.RunResult {
	t.Helper()
	got, err := s.GetRunResultByID(id)
	require.NoError(t, err)
	require.NotNil(t, got)
	return got
}

// tsVerdict answers the defect-type question "automation_bug" at 0.87 (and the verdict flaky_test
// at 0.92, which maps to the same type), so a 0.85 threshold qualifies it.
func autoApplyDeps(ts *fakeTS) failureanalysis.JobDeps {
	return failureanalysis.JobDeps{
		Decider: failureanalysis.NewTypeSafeDecider(ts, "jev-1.13.0"), DeciderModel: "jev-1.13.0",
		AutoApply: &failureanalysis.AutoApplyDeps{MinConfidence: 0.85}, AutoApplyState: models.AutoApplyStateOn,
	}
}

func TestWorker_AutoApplyLabelsTheGroupButNeverAPersonsLabel(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
		{"timeout", "Timeout waiting for #checkout button after 5000ms"},
	})
	autoApplyJobSettings(t, s)
	ids := failingIDs(t, s, run.ID)
	require.Len(t, ids, 3)
	// A person already labelled one of them (as the triage handler writes it).
	upd := store.HumanDefectTypeFields("product_bug", true)
	upd["decided_at"] = time.Now().UTC()
	require.NoError(t, s.UpdateRunResult(run.ID, ids[2], upd))
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)

	deps := autoApplyDeps(&fakeTS{fn: tsVerdict("flaky_test", "automation_bug", 0.95)})
	deps.NarrativeSkipped = true
	bc := newResultsBC()
	w := NewWorker(s, staticResolver(deps), bc, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	for _, id := range ids[:2] {
		got := resultLabel(t, s, id)
		require.Equal(t, "automation_bug", got.DefectType, id)
		require.Equal(t, models.DefectTypeSourceAI, got.DefectTypeSource, id)
		require.Nil(t, got.DecidedAt, "an AI label is not a decision")
		require.Empty(t, got.SuggestedDefectType, "and writes no snapshot")
		src, published := bc.source(id)
		require.True(t, published, "the grid is told")
		require.Equal(t, models.DefectTypeSourceAI, src)
	}
	human := resultLabel(t, s, ids[2])
	require.Equal(t, "product_bug", human.DefectType, "a person's label is never overwritten")
	require.Equal(t, models.DefectTypeSourceHuman, human.DefectTypeSource)

	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Equal(t, 2, o.AutoApplied)
	require.Equal(t, models.AutoApplyStateOn, o.AutoApplyState)
}

// Spec §3.5 + Decision 6: a new analysis that does not qualify — here because the job found the
// gate closed — resets an AI label an earlier analysis wrote.
func TestWorker_AutoApplyPausedResetsEarlierAILabels(t *testing.T) {
	s := newStore(t)
	run := seedRunWithFailures(t, s, [][2]string{{"timeout", "Timeout waiting for #pay after 5000ms"}})
	autoApplyJobSettings(t, s)
	ids := failingIDs(t, s, run.ID)
	n, err := s.ApplyAutoDefectType(ids, "product_bug")
	require.NoError(t, err)
	require.EqualValues(t, 1, n)
	job, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
	require.NoError(t, err)

	deps := autoApplyDeps(&fakeTS{fn: tsVerdict("flaky_test", "automation_bug", 0.95)})
	deps.AutoApply, deps.AutoApplyState = nil, models.AutoApplyStatePaused
	deps.NarrativeSkipped = true
	bc := newResultsBC()
	w := NewWorker(s, staticResolver(deps), bc, 10*time.Millisecond)
	require.NoError(t, w.processOnce(context.Background()))

	got := resultLabel(t, s, ids[0])
	require.Equal(t, "to_investigate", got.DefectType)
	require.Equal(t, "", got.DefectTypeSource)
	src, published := bc.source(ids[0])
	require.True(t, published)
	require.Equal(t, "", src)
	o, err := s.AnalysisJobOutcomes(job.ID)
	require.NoError(t, err)
	require.Zero(t, o.AutoApplied)
	require.Equal(t, models.AutoApplyStatePaused, o.AutoApplyState)
}

// Spec §3.3: a semantic clone is labelled only when the transfer check says the explanation fits
// it (narrative_fit ≥ TransferFitMin), after the narration lands.
func TestWorker_AutoApplySemanticCloneNeedsATransferFit(t *testing.T) {
	for _, tc := range []struct {
		name       string
		fit        float64
		wantDefect string
		wantSource string
	}{
		{"the explanation fits", 0.9, "automation_bug", models.DefectTypeSourceAI},
		{"the explanation does not fit", 0.2, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := newStore(t)
			run := seedRunWithFailures(t, s, [][2]string{
				{"timeout", "Timeout waiting for #checkout button after 5000ms"},
				{"timeout", "Timeout waiting for #checkout button after 5000ms"}, // signature clone
				{"timeout", "Timeout waiting for #checkout button after 7000ms"}, // semantic clone
			})
			autoApplyJobSettings(t, s)
			_, _, err := s.MaybeEnqueueForRun(run.ID, models.RunAnalysisJobTriggerManual, "")
			require.NoError(t, err)
			base := tsVerdict("flaky_test", "automation_bug", 0.95)
			ts := &fakeTS{fn: func(req typesafe.Request) (*typesafe.Response, error) {
				resp, err := base(req)
				if err != nil {
					return nil, err
				}
				for id := range req.Questions {
					if strings.HasPrefix(id, "fits_") {
						resp.Answers[id] = typesafe.Answer{Type: "noul", Noul: tc.fit}
					}
				}
				return resp, nil
			}}
			deps := autoApplyDeps(ts)
			deps.Narrative, deps.NarrativeModel = &verdictProvider{verdict: "product_bug"}, "mock"
			deps.Semantic = &failureanalysis.SemanticDeps{Client: ts, Model: "jev-1.13.0"}
			deps.Transfer = &failureanalysis.TransferDeps{Client: ts, Model: "jev-1.13.0"}
			w := NewWorker(s, staticResolver(deps), newResultsBC(), 10*time.Millisecond)
			require.NoError(t, w.processOnce(context.Background()))

			analyses, err := s.GetCurrentAnalysesByRun(run.ID)
			require.NoError(t, err)
			require.Len(t, analyses, 3)
			for resultID, a := range analyses {
				got := resultLabel(t, s, resultID)
				if a.DedupMethod == models.DedupMethodSemantic {
					require.NotNil(t, a.NarrativeFit, "the transfer check ran")
					require.Equal(t, tc.wantDefect, got.DefectType)
					require.Equal(t, tc.wantSource, got.DefectTypeSource)
					continue
				}
				require.Equal(t, "automation_bug", got.DefectType, "representative and signature clone are labelled in the decided phase")
				require.Equal(t, models.DefectTypeSourceAI, got.DefectTypeSource)
			}
		})
	}
}
