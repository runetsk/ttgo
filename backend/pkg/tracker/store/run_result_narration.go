package store

import (
	"sort"
	"strings"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"

	"gorm.io/gorm"
)

// SweptNarrativeSummary is what a row whose explanation was never written says after a sweep.
const SweptNarrativeSummary = "AI narrative unavailable: the analysis ended before the explanation was written"

// groupMemberScanLimit bounds how many clone results an on-demand explanation loads to pick its
// member excerpts; the prompt keeps only a handful of distinct ones.
const groupMemberScanLimit = 200

// claimableNarratives are the explanation states an on-demand Explain may start from.
var claimableNarratives = []string{models.NarrativeStatusSkipped, models.NarrativeStatusUnavailable, models.NarrativeStatusUnparseable}

// ClaimExplanation atomically moves a representative's decided, unexplained analysis to
// pending (bumping narrative_revision) so exactly one explanation is written per group at a
// time. false = someone else holds it, it is already explained, or it is not a claimable
// representative (a clone, a failed attempt).
func (s *Store) ClaimExplanation(repID string) (bool, error) {
	res := s.db.Model(&models.RunResultAnalysis{}).
		Where("id = ? AND source_analysis_id IS NULL AND decision_status = ? AND narrative_status IN ?",
			repID, models.DecisionStatusOK, claimableNarratives).
		Updates(map[string]interface{}{
			"narrative_status":   models.NarrativeStatusPending,
			"narrative_revision": gorm.Expr("narrative_revision + 1"),
		})
	return res.RowsAffected > 0, res.Error
}

// ApplyNarration writes an explanation onto a whole group in one transaction, and only while the
// representative is still pending (from the worker's decided phase or an Explain claim). The
// representative gets the text, raw response, finish reason and the call's usage/timing added;
// each clone (source_analysis_id = repID) gets the text and its rationale rebuilt as its own
// "[Grouped …] " marker + the new rationale, and no usage. Both bump narrative_revision.
// It returns the changed rows, representative first; none when the representative was no
// longer pending (a lost apply: a sweep settled the group while the call ran).
func (s *Store) ApplyNarration(repID string, d failureanalysis.NarrationDelta) ([]*models.RunResultAnalysis, error) {
	var ids []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		rep := map[string]interface{}{
			"summary":                d.Summary,
			"next_action":            d.NextAction,
			"rationale":              d.Rationale,
			"narrative_status":       d.NarrativeStatus,
			"narrative_revision":     gorm.Expr("narrative_revision + 1"),
			"token_usage_prompt":     gorm.Expr("token_usage_prompt + ?", d.PromptTokens),
			"token_usage_completion": gorm.Expr("token_usage_completion + ?", d.CompletionTokens),
			"llm_ms":                 gorm.Expr("llm_ms + ?", d.LLMMs),
			"llm_calls":              gorm.Expr("llm_calls + ?", d.LLMCalls),
		}
		// A narration that made no call (cancelled before it started) keeps what the row had.
		if d.RawResponse != "" {
			rep["raw_response"] = d.RawResponse
		}
		if d.FinishReason != "" {
			rep["finish_reason"] = d.FinishReason
		}
		res := tx.Model(&models.RunResultAnalysis{}).
			Where("id = ? AND narrative_status = ?", repID, models.NarrativeStatusPending).
			Updates(rep)
		if res.Error != nil || res.RowsAffected == 0 {
			return res.Error
		}
		ids = append(ids, repID)

		var clones []struct {
			ID        string
			Rationale string
		}
		if err := tx.Model(&models.RunResultAnalysis{}).Select("id, rationale").
			Where("source_analysis_id = ?", repID).Scan(&clones).Error; err != nil {
			return err
		}
		for _, c := range clones {
			if err := tx.Model(&models.RunResultAnalysis{}).Where("id = ?", c.ID).Updates(map[string]interface{}{
				"summary":            d.Summary,
				"next_action":        d.NextAction,
				"rationale":          groupedPrefix(c.Rationale) + d.Rationale,
				"narrative_status":   d.NarrativeStatus,
				"narrative_revision": gorm.Expr("narrative_revision + 1"),
			}).Error; err != nil {
				return err
			}
			ids = append(ids, c.ID)
		}
		return nil
	})
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	var out []*models.RunResultAnalysis
	if err := s.db.Where("id IN ?", ids).Find(&out).Error; err != nil {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID == repID && out[j].ID != repID })
	return out, nil
}

// groupedPrefix is the "[Grouped …] " marker a clone's rationale starts with, or "".
func groupedPrefix(rationale string) string {
	if !strings.HasPrefix(rationale, "[Grouped") {
		return ""
	}
	if i := strings.Index(rationale, "] "); i >= 0 {
		return rationale[:i+2]
	}
	return ""
}

func (s *Store) pendingNarratives(jobID string) *gorm.DB {
	q := s.db.Model(&models.RunResultAnalysis{}).Where("narrative_status = ?", models.NarrativeStatusPending)
	if jobID != "" {
		q = q.Where("job_id = ?", jobID)
	}
	return q
}

// PendingNarrativeIDs lists the rows SweepPendingNarratives(jobID) would settle.
func (s *Store) PendingNarrativeIDs(jobID string) ([]string, error) {
	var ids []string
	err := s.pendingNarratives(jobID).Pluck("id", &ids).Error
	return ids, err
}

// SweepPendingNarratives marks explanations that will never be written as unavailable: a job's
// leftovers when it ends (jobID), or every pending row at worker startup (jobID ""), which the
// single-process deployment makes safe — nothing can be narrating then.
func (s *Store) SweepPendingNarratives(jobID string) (int64, error) {
	res := s.pendingNarratives(jobID).Updates(map[string]interface{}{
		"narrative_status":   models.NarrativeStatusUnavailable,
		"summary":            SweptNarrativeSummary,
		"narrative_revision": gorm.Expr("narrative_revision + 1"),
	})
	return res.RowsAffected, res.Error
}

// ListGroupMemberResults returns the results of a representative's clones (bounded), so an
// on-demand explanation can quote the group's other failures.
func (s *Store) ListGroupMemberResults(repAnalysisID string) ([]*models.RunResult, error) {
	var out []*models.RunResult
	err := s.db.Raw(`
		SELECT rr.* FROM run_results rr
		JOIN run_result_analyses a ON a.run_result_id = rr.id
		WHERE a.source_analysis_id = ?
		ORDER BY rr.start_time, rr.id
		LIMIT ?`, repAnalysisID, groupMemberScanLimit).Scan(&out).Error
	return out, err
}
