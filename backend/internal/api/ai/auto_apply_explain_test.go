package ai_test

import (
	"net/http"
	"testing"

	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/require"
)

// R10: a group Explain keeps the semantic clone's AI label in step with the fit it just computed —
// labelled when the explanation fits and the stored representative qualifies, reset otherwise. It
// never labels the representative or the signature clone.
func TestExplainAnalysis_KeepsSemanticCloneLabelsInStepWithTheFit(t *testing.T) {
	for _, tc := range []struct {
		name       string
		fit        float64
		wantDefect string
		wantSource string
	}{
		{"the explanation fits", 0.9, "automation_bug", models.DefectTypeSourceAI},
		{"the explanation does not fit", 0.3, "to_investigate", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prov := &promptProvider{reply: explainReply}
			base := transferDeps(prov, &fitsClient{fit: tc.fit})
			e := newQuickEnv(t, func(trigger string) (failureanalysis.JobDeps, error) {
				d, err := base(trigger)
				d.AutoApply, d.AutoApplyState = &failureanalysis.AutoApplyDeps{MinConfidence: 0.95}, models.AutoApplyStateOn
				return d, err
			})
			g := seedTransferGroup(t, e, models.NarrativeStatusSkipped, nil)
			// The stored representative is a direct TypeSafe answer at 0.97.
			require.NoError(t, e.s.DB().Model(&models.RunResultAnalysis{}).Where("id = ?", g.rep.ID).
				Update("suggested_defect_type_confidence", 0.97).Error)
			// An earlier analysis had labelled the semantic clone.
			n, err := e.s.ApplyAutoDefectType([]string{g.semRR.ID}, "system_issue")
			require.NoError(t, err)
			require.EqualValues(t, 1, n)

			rec := serve(e.h.ExplainAnalysis, "POST", map[string]string{"id": g.repRR.ID, "analysisId": g.rep.ID})
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

			sem, err := e.s.GetRunResultByID(g.semRR.ID)
			require.NoError(t, err)
			require.Equal(t, tc.wantDefect, sem.DefectType)
			require.Equal(t, tc.wantSource, sem.DefectTypeSource)
			for _, id := range []string{g.repRR.ID, g.sigRR.ID} {
				other, err := e.s.GetRunResultByID(id)
				require.NoError(t, err)
				require.Equal(t, "", other.DefectType, "Explain never labels the representative or a signature clone")
			}
		})
	}
}
