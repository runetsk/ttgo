package failureanalysis

import (
	"encoding/json"
	"html"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"ttgo/pkg/tracker/models"

	"github.com/microcosm-cc/bluemonday"
)

// EnrichmentSource is the subset of *store.Store that BuildContext needs.
// It lives in this package (rather than importing store) so failureanalysis
// never depends on store — no import cycle — and the builder stays unit-testable
// with a mock. *store.Store satisfies it structurally, so the signatures MUST
// match the store methods exactly (value vs pointer in params and returns).
type EnrichmentSource interface {
	ListDefectsByTestCase(string) ([]models.Defect, error)
	ListRequirementsByTestCase(string) ([]*models.Requirement, error)
	ListRecentFailuresByTestCase(string, time.Time, int, string) ([]*models.RunResult, error)
}

// Enrichment window/limits for the historical-failure lookup.
const (
	enrichHistoryDays  = 30
	enrichHistoryLimit = SimilarFailuresMax
)

// timesGlyph is the multiplication sign used in the rollup ("automation_bug ×2").
const timesGlyph = "×"

// stepDTO is the tagged intermediate for RunResult.Steps. The stored JSON keys
// (action / expected_result / order_index) differ from PromptStep's field names,
// so unmarshalling straight into PromptStep would silently drop Expected/Order.
type stepDTO struct {
	Action         string `json:"action"`
	ExpectedResult string `json:"expected_result"`
	OrderIndex     int    `json:"order_index"`
}

// BuildContext assembles a fully-populated AnalyzeContext for result using src.
//
// It is best-effort: each source query is guarded independently, so a failing
// query logs a warning and leaves that slot empty but never fails the build —
// BuildContext never returns an error. Env fields and steps come straight off
// the RunResult (no query); a result with a nil/empty TestCaseID (deleted test
// case) still gets those, and the three cross-entity queries are skipped.
//
// Callers layer PromptTemplate/RedactionEnabled/ProviderModel onto the result.
func BuildContext(src EnrichmentSource, result *models.RunResult, now time.Time) AnalyzeContext {
	ctx := AnalyzeContext{
		Result:     result,
		Env:        result.Environment,
		Browser:    result.Browser,
		OS:         result.OS,
		AppVersion: result.AppVersion,
		Steps:      buildSteps(result.Steps),
	}

	if result.TestCaseID == nil || *result.TestCaseID == "" {
		return ctx
	}
	tcID := *result.TestCaseID

	if defects, err := src.ListDefectsByTestCase(tcID); err != nil {
		slog.Warn("failure-analysis: enrich defects failed", "err", err, "test_case_id", tcID)
	} else {
		ctx.LinkedDefects = mapDefects(defects)
	}

	if reqs, err := src.ListRequirementsByTestCase(tcID); err != nil {
		slog.Warn("failure-analysis: enrich requirements failed", "err", err, "test_case_id", tcID)
	} else {
		ctx.LinkedRequirements = mapRequirements(reqs)
	}

	since := now.AddDate(0, 0, -enrichHistoryDays)
	if hist, err := src.ListRecentFailuresByTestCase(tcID, since, enrichHistoryLimit, result.TestRunID); err != nil {
		slog.Warn("failure-analysis: enrich history failed", "err", err, "test_case_id", tcID)
	} else {
		ctx.SimilarFailures = mapSimilarFailures(hist)
		ctx.SimilarFailuresRollup = rollupDefectTypes(hist)
	}

	return ctx
}

// buildSteps decodes RunResult.Steps into prompt steps, scrubbing HTML from the
// text fields. Malformed or empty JSON yields nil (never a panic).
func buildSteps(raw json.RawMessage) []PromptStep {
	if len(raw) == 0 {
		return nil
	}
	var dtos []stepDTO
	if err := json.Unmarshal(raw, &dtos); err != nil {
		return nil
	}
	if len(dtos) == 0 {
		return nil
	}
	steps := make([]PromptStep, 0, len(dtos))
	for i, d := range dtos {
		order := d.OrderIndex
		if order == 0 {
			order = i + 1 // synthesize from position when order_index is 0/absent
		}
		steps = append(steps, PromptStep{
			Order:    order,
			Action:   plain(d.Action),
			Expected: plain(d.ExpectedResult),
		})
	}
	return steps
}

func mapDefects(defects []models.Defect) []LinkedDefect {
	if len(defects) == 0 {
		return nil
	}
	out := make([]LinkedDefect, 0, len(defects))
	for _, d := range defects {
		out = append(out, LinkedDefect{
			Key:     firstNonEmpty(d.ExternalKey, shortID(d.ID)),
			Status:  d.Status,
			Summary: d.Title,
		})
	}
	return out
}

func mapRequirements(reqs []*models.Requirement) []LinkedRequirement {
	if len(reqs) == 0 {
		return nil
	}
	out := make([]LinkedRequirement, 0, len(reqs))
	for _, r := range reqs {
		if r == nil {
			continue
		}
		out = append(out, LinkedRequirement{
			Key:   r.Identifier,
			Title: r.Title,
		})
	}
	return out
}

func mapSimilarFailures(hist []*models.RunResult) []SimilarFailure {
	if len(hist) == 0 {
		return nil
	}
	out := make([]SimilarFailure, 0, len(hist))
	for _, r := range hist {
		if r == nil {
			continue
		}
		out = append(out, SimilarFailure{
			RunStartedAt: r.StartTime,
			Status:       string(r.Status),
			ErrorMessage: r.ErrorMessage,
			DefectType:   r.DefectType,
			// DefectKey: left empty in v1 — see enrich.go [decision]. The 3-method
			// EnrichmentSource has no per-result defect lookup, and attributing a
			// test-case-scoped defect to a specific historical row would be wrong.
		})
	}
	return out
}

// rollupDefectTypes renders a one-line distribution of the human triage labels
// across the historical rows, e.g. "automation_bug ×2, product_bug ×1". Rows
// without a label are ignored; returns "" when no row carries one. Ordering is
// deterministic: highest count first, ties broken alphabetically.
func rollupDefectTypes(hist []*models.RunResult) string {
	counts := map[string]int{}
	for _, r := range hist {
		if r == nil || r.DefectType == "" {
			continue
		}
		counts[r.DefectType]++
	}
	if len(counts) == 0 {
		return ""
	}
	type kv struct {
		label string
		n     int
	}
	pairs := make([]kv, 0, len(counts))
	for label, n := range counts {
		pairs = append(pairs, kv{label, n})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].n != pairs[j].n {
			return pairs[i].n > pairs[j].n
		}
		return pairs[i].label < pairs[j].label
	})
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, p.label+" "+timesGlyph+strconv.Itoa(p.n))
	}
	return strings.Join(parts, ", ")
}

// plain strips HTML tags and unescapes entities so stored step markup
// (e.g. "<p>Click &amp; wait</p>") becomes bare prompt text.
// StrictPolicy removes all tags; mirrors internal/api/httpx/html.go:12.
func plain(s string) string {
	return html.UnescapeString(bluemonday.StrictPolicy().Sanitize(s))
}

// shortID returns a log-safe short prefix of an internal ID. Length-safe:
// a bare id[:8] evaluates eagerly and panics on IDs shorter than 8 bytes.
func shortID(id string) string {
	if len(id) < 8 {
		return id
	}
	return id[:8]
}
