package ai

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"ttgo/internal/importparser"
	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"
)

// typesafeImportStructure recovers test cases from content the deterministic parsers could not
// read (spec Wave 5 §1): TypeSafe.ai names each line's role and the cases are assembled from the
// lines themselves. It returns nothing when the use is unavailable, a request fails, or no case
// comes out; the caller then runs the LLM fallback.
func (h *Handler) typesafeImportStructure(ctx context.Context, content string) ([]models.GeneratedTestCase, map[string]interface{}) {
	use, why := h.TypeSafeFor(TypeSafeUseImportStructure)
	if use == nil {
		slog.DebugContext(ctx, "ai_import: TypeSafe structure recovery skipped", "reason", why)
		return nil, nil
	}
	lines, truncated := importparser.ImportLines(content)
	if len(lines) == 0 {
		return nil, nil
	}
	sent := make([]map[string]any, len(lines))
	for i, l := range lines {
		if use.Redact {
			l = failureanalysis.Redact(l)
		}
		sent[i] = map[string]any{"i": i, "text": l}
	}
	start := time.Now()
	roles := make([]string, len(lines))
	tokens, requests, model := 0, 0, ""
	for from := 0; from < len(lines); from += failureanalysis.ImportLinesPerRequest {
		to := min(from+failureanalysis.ImportLinesPerRequest, len(lines))
		qs := make(map[string]typesafe.Question, to-from)
		for i := from; i < to; i++ {
			qs[fmt.Sprintf("line_%d", i)] = failureanalysis.ImportLineQuestion(i)
		}
		// Every request carries all the lines, so each line is read in its full context.
		resp, err := use.Client.Evaluate(ctx, typesafe.Request{State: map[string]any{"lines": sent}, Model: use.Model, Questions: qs})
		requests++
		if err != nil {
			slog.WarnContext(ctx, "ai_import: TypeSafe structure recovery failed; using the LLM fallback", "err", err)
			return nil, nil
		}
		h.recordTypeSafeUse(models.AnalysisCostKindImport, use, resp)
		tokens += resp.Usage.InputTokens
		model = resp.Model
		for i := from; i < to; i++ {
			if a, ok := resp.Answers[fmt.Sprintf("line_%d", i)]; ok {
				roles[i] = a.Choice
			}
		}
	}
	cases := importparser.AssembleClassified(lines, roles)
	return cases, map[string]interface{}{
		"engine": "typesafe", "model": model, "policy_version": failureanalysis.ImportPolicyVersion,
		"lines": len(lines), "lines_truncated": truncated, "requests": requests, "input_tokens": tokens,
		"duration_ms": time.Since(start).Milliseconds(),
	}
}
