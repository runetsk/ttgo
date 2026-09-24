package ai

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
	"ttgo/internal/api/httpx"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/store"
	"ttgo/pkg/tracker/typesafe"
)

// GetTypeSafeSettings returns the TypeSafe.ai configuration with the key masked.
//
// @Summary      TypeSafe settings
// @Description  Vendor master switch, pinned model, per-feature switches and the API key's status (missing | ok | undecryptable). The key itself is never returned.
// @Tags         ai-settings
// @Produce      json
// @Success      200  {object}  models.TypeSafeSettingsResponse
// @Failure      500  {object}  map[string]interface{}
// @Router       /settings/typesafe [get]
// @Security     BearerAuth
func (h *Handler) GetTypeSafeSettings(w http.ResponseWriter, r *http.Request) {
	resp, err := h.store.TypeSafeSettingsResponse()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// UpdateTypeSafeSettings applies a partial update (admin).
//
// @Summary      Update TypeSafe settings
// @Description  Partial update. An omitted or blank api_key preserves the stored key; clear_api_key removes it; both together is rejected. timeout_seconds must be 5..300. escalate_below_pct must be 0..100 (0 = never ask the LLM to decide).
// @Tags         ai-settings
// @Accept       json
// @Produce      json
// @Param        body  body      models.TypeSafeSettingsPatch  true  "Fields to change"
// @Success      200   {object}  models.TypeSafeSettingsResponse
// @Failure      400   {object}  map[string]interface{}
// @Failure      422   {object}  map[string]interface{}
// @Router       /settings/typesafe [put]
// @Security     BearerAuth
func (h *Handler) UpdateTypeSafeSettings(w http.ResponseWriter, r *http.Request) {
	var p models.TypeSafeSettingsPatch
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if p.ClearAPIKey && p.APIKey != nil && *p.APIKey != "" {
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "api_key and clear_api_key are mutually exclusive"})
		return
	}
	if p.Model != nil && *p.Model == "" {
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "model must not be empty"})
		return
	}
	if p.TimeoutSeconds != nil && (*p.TimeoutSeconds < 5 || *p.TimeoutSeconds > 300) {
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "timeout_seconds must be between 5 and 300"})
		return
	}
	if p.EscalateBelowPct != nil && (*p.EscalateBelowPct < 0 || *p.EscalateBelowPct > 100) {
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "escalate_below_pct must be between 0 and 100"})
		return
	}
	if _, err := h.store.UpdateTypeSafeSettings(p); err != nil {
		if errors.Is(err, store.ErrKeyAndClearExclusive) {
			httpx.JSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	resp, err := h.store.TypeSafeSettingsResponse()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// TestTypeSafeConnection lists the account's models with the stored key (admin).
//
// @Summary      Test TypeSafe connection
// @Description  Calls the vendor's models endpoint with the stored key. Always 200: {ok:true, models:[...]} or {ok:false, category, message}. A missing or undecryptable key reports category "auth".
// @Tags         ai-settings
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /settings/typesafe/test [post]
// @Security     BearerAuth
func (h *Handler) TestTypeSafeConnection(w http.ResponseWriter, r *http.Request) {
	fail := func(category, msg string) {
		httpx.JSON(w, http.StatusOK, map[string]interface{}{"ok": false, "category": category, "message": msg})
	}
	settings, err := h.store.GetTypeSafeSettings()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	key, err := h.store.TypeSafeAPIKey()
	if err != nil {
		fail(string(typesafe.CategoryAuth), "API key cannot be decrypted; re-enter it")
		return
	}
	if key == "" {
		fail(string(typesafe.CategoryAuth), "no API key stored")
		return
	}
	report := func(err error) {
		var te *typesafe.Error
		if errors.As(err, &te) {
			fail(string(te.Category), te.Message)
			return
		}
		fail(string(typesafe.CategoryNetwork), err.Error())
	}
	client := h.newTypeSafeClient(key, time.Duration(settings.TimeoutSeconds)*time.Second)
	ms, err := client.ListModels(r.Context())
	if err != nil {
		report(err)
		return
	}
	if len(ms) == 0 {
		// Gateways such as OpenRouter serve the System One endpoint but not
		// TypeSafe's model list, so an empty list proves nothing about the key.
		// Prove key and model with the cheapest valid evaluation instead.
		if _, err := client.Evaluate(r.Context(), probeRequest(settings.Model)); err != nil {
			report(err)
			return
		}
		ms = []typesafe.Model{{Name: settings.Model}}
	}
	httpx.JSON(w, http.StatusOK, map[string]interface{}{"ok": true, "models": ms})
}

// probeRequest is a one-question, one-line System One call used only by the
// connection test when the endpoint lists no models.
func probeRequest(model string) typesafe.Request {
	return typesafe.Request{
		Model: model,
		State: "TTGO connection probe.",
		Questions: map[string]typesafe.Question{
			"probe": {Type: "noul", Instructions: "Is this state a connection probe?"},
		},
	}
}
