package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"ttgo/pkg/tracker/aigen"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"
)

// reviewText is a draft or candidate as TypeSafe reads it.
func reviewText(use *TypeSafeUse, name, description string, steps []models.GeneratedStep) map[string]any {
	red := func(s string) string {
		if use.Redact {
			return failureanalysis.Redact(s)
		}
		return s
	}
	ss := make([]map[string]string, 0, len(steps))
	for _, s := range steps {
		ss = append(ss, map[string]string{"action": red(s.Action), "expected": red(s.ExpectedResult)})
	}
	return map[string]any{"name": red(name), "description": red(description), "steps": ss}
}

// runTypeSafeDraftReview rates a generation's drafts and judges their duplicate candidates with
// TypeSafe.ai (spec Wave 5 §2), updating each draft's quality and duplicates in place. It is
// best effort: a failure leaves the drafts as the rubric left them and returns a warning.
func (h *Handler) runTypeSafeDraftReview(ctx context.Context, rows []*models.AIGeneratedDraft, requirementID string) string {
	if len(rows) == 0 {
		return ""
	}
	use, _ := h.TypeSafeFor(TypeSafeUseDraftReview)
	if use == nil {
		return ""
	}
	contents := make([]models.DraftContent, len(rows))
	byPosition := map[int]models.DraftContent{}
	for i, r := range rows {
		c, err := r.Content()
		if err != nil {
			return ""
		}
		contents[i] = c
		byPosition[r.Position] = c
	}
	for from := 0; from < len(rows); from += failureanalysis.DraftReviewPerRequest {
		to := min(from+failureanalysis.DraftReviewPerRequest, len(rows))
		drafts := make([]map[string]any, 0, to-from)
		var candidates []map[string]any
		asked := make([][]aigen.DuplicateCandidate, to-from)
		shown := make([][]aigen.DuplicateCandidate, to-from)
		qs := map[string]typesafe.Question{}
		for d := 0; d < to-from; d++ {
			r, c := rows[from+d], contents[from+d]
			drafts = append(drafts, reviewText(use, c.Name, c.Description, c.Steps))
			qs[fmt.Sprintf("clarity_%d", d)] = failureanalysis.DraftClarityQuestion(d)
			qs[fmt.Sprintf("observable_%d", d)] = failureanalysis.DraftObservableQuestion(d)
			qs[fmt.Sprintf("specific_%d", d)] = failureanalysis.DraftSpecificQuestion(d)
			if r.DuplicatesJSON != "" {
				_ = json.Unmarshal([]byte(r.DuplicatesJSON), &shown[d])
			}
			wider, err := h.store.SearchDuplicateCandidatesAt(c.Name, requirementID, failureanalysis.DraftDupPerDraft*2, failureanalysis.DraftDupAskFloor)
			if err != nil {
				wider = nil
			}
			for _, cand := range aigen.AskCandidates(shown[d], wider) {
				var text map[string]any
				switch {
				case cand.Kind == aigen.DupKindExisting:
					tc, err := h.store.GetTestCase(cand.TestCaseID)
					if err != nil || tc == nil {
						continue
					}
					steps := make([]models.GeneratedStep, 0, len(tc.Steps))
					for _, s := range tc.Steps {
						steps = append(steps, models.GeneratedStep{Action: s.Action, ExpectedResult: s.ExpectedResult})
					}
					text = reviewText(use, tc.Name, tc.Description, steps)
				case cand.DraftPosition != nil:
					bc, ok := byPosition[*cand.DraftPosition]
					if !ok {
						continue
					}
					text = reviewText(use, bc.Name, bc.Description, bc.Steps)
				default:
					continue
				}
				k := len(candidates)
				candidates = append(candidates, text)
				asked[d] = append(asked[d], cand)
				qs[fmt.Sprintf("dup_%d_%d", d, k)] = failureanalysis.DraftDuplicateQuestion(d, k)
			}
		}
		resp, err := use.Client.Evaluate(ctx, typesafe.Request{
			State: map[string]any{"drafts": drafts, "candidates": candidates}, Model: use.Model, Questions: qs})
		if err != nil {
			slog.Warn("ai_generation: TypeSafe draft review failed", "err", err)
			return "TypeSafe.ai draft review failed: " + err.Error()
		}
		h.recordTypeSafeUse(models.AnalysisCostKindDraftReview, use, resp)
		k := 0
		for d := 0; d < to-from; d++ {
			r := rows[from+d]
			ratings := map[string]aigen.Rating{}
			for _, aspect := range []string{"clarity", "observable", "specific"} {
				if a, ok := resp.Answers[fmt.Sprintf("%s_%d", aspect, d)]; ok {
					ratings[aspect] = aigen.Rating{Value: a.Choice, Confidence: a.Confidence}
				}
			}
			q := aigen.WithReviewDimension(r.QualityJSON, aigen.ReviewFindings(ratings))
			if q != r.QualityJSON {
				r.QualityJSON = q
				if err := h.store.UpdateDraftQuality(r.ID, q); err != nil {
					slog.Warn("ai_generation: review quality persist failed", "draft_id", r.ID, "err", err)
				}
			}
			p := map[string]float64{}
			for _, cand := range asked[d] {
				if a, ok := resp.Answers[fmt.Sprintf("dup_%d_%d", d, k)]; ok {
					p[aigen.DupKey(cand)] = a.Noul
				}
				k++
			}
			merged := aigen.MergeDuplicateVerdicts(shown[d], asked[d], p)
			b, _ := json.Marshal(merged)
			if string(b) != r.DuplicatesJSON {
				r.DuplicatesJSON = string(b)
				if err := h.store.UpdateDraftDuplicates(r.ID, string(b)); err != nil {
					slog.Warn("ai_generation: review duplicates persist failed", "draft_id", r.ID, "err", err)
				}
			}
		}
	}
	return ""
}
