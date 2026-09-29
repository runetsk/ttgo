package ai_test

import (
	"context"
	"testing"
	"time"

	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"

	"github.com/stretchr/testify/require"
)

// confDecider is a TypeSafe decision whose defect-type confidence the test moves.
type confDecider struct{ conf float64 }

func (d *confDecider) Decide(context.Context, failureanalysis.Evidence) (*failureanalysis.Decision, error) {
	return &failureanalysis.Decision{Verdict: models.VerdictProductBug, VerdictConfidence: 0.95,
		SuggestedDefectType: "product_bug", DefectTypeConfidence: d.conf, Model: "jev-1.13.0",
		PolicyVersion: failureanalysis.PolicyVersionNoExamples}, nil
}

// R3: single-result Analyze follows and resets an AI label like the worker, and never touches a
// person's label.
func TestAnalyzeRunResult_AutoApplyFollowsTheNewAnalysis(t *testing.T) {
	dec := &confDecider{conf: 0.97}
	e := newQuickEnv(t, func(string) (failureanalysis.JobDeps, error) {
		return failureanalysis.JobDeps{Decider: dec, DeciderModel: "jev-1.13.0", NarrativeSkipped: true,
			AutoApply: &failureanalysis.AutoApplyDeps{MinConfidence: 0.95}, AutoApplyState: models.AutoApplyStateOn}, nil
	})
	rr := e.result
	label := func() (string, string) {
		t.Helper()
		got, err := e.s.GetRunResultByID(rr.ID)
		require.NoError(t, err)
		return got.DefectType, got.DefectTypeSource
	}

	var out map[string]interface{}
	callAnalyze(t, e.h, rr.ID, &out)
	d, src := label()
	require.Equal(t, "product_bug", d)
	require.Equal(t, models.DefectTypeSourceAI, src)

	dec.conf = 0.90 // the newer analysis no longer qualifies: the AI label resets
	callAnalyze(t, e.h, rr.ID, &out)
	d, src = label()
	require.Equal(t, "to_investigate", d)
	require.Equal(t, "", src)

	upd := store.HumanDefectTypeFields("system_issue", true)
	upd["decided_at"] = time.Now().UTC()
	require.NoError(t, e.s.UpdateRunResult(rr.TestRunID, rr.ID, upd))
	dec.conf = 0.99
	callAnalyze(t, e.h, rr.ID, &out)
	d, src = label()
	require.Equal(t, "system_issue", d, "a person's label is never overwritten")
	require.Equal(t, models.DefectTypeSourceHuman, src)
}
