package ai

import (
	"log/slog"
	"net/http"
	"time"

	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"
)

// TypeSafe used beyond failure analysis (spec Wave 5 §0): each use has its own switch in the
// TypeSafe settings.
const (
	TypeSafeUseImportStructure = "import_structure"
	TypeSafeUseDraftReview     = "draft_review"
	TypeSafeUseDefectAssist    = "defect_assist"
	TypeSafeUseSearchRerank    = "search_rerank"
)

// TypeSafeUse is what one TypeSafe use needs: the client, the model to ask, whether to redact the
// text it sends (the failure-analysis redaction setting) and the price for the cost ledger.
type TypeSafeUse struct {
	Client       typesafe.Client
	Model        string
	Redact       bool
	PricePerMTok float64
}

// useSwitch reads a use's switch from the settings.
func useSwitch(ts *models.TypeSafeSettings, use string) bool {
	switch use {
	case TypeSafeUseImportStructure:
		return ts.ImportStructureEnabled
	case TypeSafeUseDraftReview:
		return ts.DraftReviewEnabled
	case TypeSafeUseDefectAssist:
		return ts.DefectAssistEnabled
	case TypeSafeUseSearchRerank:
		return ts.SearchRerankEnabled
	}
	return false
}

// TypeSafeFor returns the client for a use of TypeSafe outside failure analysis, or nil and why
// it is unavailable: the AI master switch, TypeSafe itself, the use's switch, then a usable key.
func (h *Handler) TypeSafeFor(use string) (*TypeSafeUse, string) {
	on, err := h.aiEnabled()
	if err != nil {
		return nil, "AI settings could not be read"
	}
	if !on {
		return nil, aiOffMessage
	}
	ts, err := h.store.GetTypeSafeSettings()
	if err != nil || ts == nil {
		return nil, "TypeSafe.ai settings could not be read"
	}
	if !ts.Enabled {
		return nil, "TypeSafe.ai is switched off"
	}
	if !useSwitch(ts, use) {
		return nil, "this use of TypeSafe.ai is switched off in its settings"
	}
	key, err := h.store.TypeSafeAPIKey()
	if err != nil || key == "" {
		return nil, "TypeSafe.ai has no usable API key"
	}
	redact := true
	if fa, err := h.store.GetFailureAnalysisSettings(); err == nil {
		redact = fa.RedactionEnabled
	}
	return &TypeSafeUse{Client: h.newTypeSafeClient(key, time.Duration(ts.TimeoutSeconds)*time.Second), Model: ts.Model,
		Redact: redact, PricePerMTok: ts.PricePerMTok}, ""
}

// recordTypeSafeUse bills one TypeSafe request of a use outside failure analysis (no run).
func (h *Handler) recordTypeSafeUse(kind string, use *TypeSafeUse, resp *typesafe.Response) {
	if use == nil || resp == nil || resp.Usage.InputTokens <= 0 {
		return
	}
	cost := float64(resp.Usage.InputTokens) * use.PricePerMTok / 1e6
	ev := &models.AIAnalysisCostEvent{Kind: kind, Engine: models.AnalysisCostEngineTypeSafe, Model: resp.Model,
		TypeSafeInputTokens: resp.Usage.InputTokens, EstimatedCost: &cost}
	if err := h.store.RecordAnalysisCostEvent(ev); err != nil {
		slog.Warn("typesafe: cost event not recorded", "kind", kind, "err", err)
	}
}

// GetTypeSafeUses says which uses of TypeSafe outside failure analysis would work now.
//
// @Summary      TypeSafe.ai uses available now
// @Description  For each use of TypeSafe.ai outside failure analysis (import structure recovery, AI-draft review, defect assist, search re-ranking): whether it would run now — AI features on, TypeSafe.ai enabled with a usable key, and the use's own switch on. The UI shows a TypeSafe control only when its use is available.
// @Tags         ai
// @Produce      json
// @Success      200  {object}  map[string]bool
// @Router       /ai/typesafe/features [get]
// @Security     BearerAuth
func (h *Handler) GetTypeSafeUses(w http.ResponseWriter, r *http.Request) {
	out := map[string]bool{}
	for _, use := range []string{TypeSafeUseImportStructure, TypeSafeUseDraftReview, TypeSafeUseDefectAssist, TypeSafeUseSearchRerank} {
		u, _ := h.TypeSafeFor(use)
		out[use] = u != nil
	}
	httpx.JSON(w, http.StatusOK, out)
}
