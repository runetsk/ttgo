package requirements

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"ttgo/internal/api/httpx"
	apiws "ttgo/internal/api/websocket"
	"ttgo/pkg/tracker/models"

	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
)

// jiraPostHash is a stable content fingerprint of the linked test cases, used to
// skip re-posting unchanged content to Jira (F-056).
func jiraPostHash(tcs []*models.TestCase) string {
	h := sha256.New()
	for _, tc := range tcs {
		h.Write([]byte(tc.ID))
		h.Write([]byte{0})
		h.Write([]byte(tc.Name))
		h.Write([]byte{0})
		h.Write([]byte(tc.Description))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (h *Handler) broadcastRequirementUpdate(topic string, data interface{}) {
	if h.hub == nil {
		return
	}
	h.hub.Broadcast(apiws.NewEvent(apiws.EventRequirementUpdated, topic, data))
}

func (h *Handler) fetchJiraRequirement(cfg *models.JiraConfig, sourceKey string) (title, description, sourceURL string, err error) {
	if h.fetchJiraTicket == nil {
		return "", "", "", errors.New("jira fetcher is not configured")
	}
	return h.fetchJiraTicket(cfg, sourceKey, h.sanitizer)
}

func (h *Handler) fetchConfluenceRequirement(cfg *models.ConfluenceConfig, sourceKey string) (title, description, sourceURL string, err error) {
	if h.fetchConfluence == nil {
		return "", "", "", errors.New("confluence fetcher is not configured")
	}
	return h.fetchConfluence(cfg, sourceKey, h.sanitizer)
}

// ────────────────────────────────────────────────────────────────────────────
// Requirements handlers (007-req-traceability)
// ────────────────────────────────────────────────────────────────────────────

// handleListRequirements godoc
// @Summary      List requirements
// @Description  Returns all requirements ordered by identifier
// @Tags         requirements
// @Produce      json
// @Success      200  {array}   models.Requirement
// @Failure      500  {object}  map[string]string
// @Router       /requirements [get]
func (h *Handler) ListRequirements(w http.ResponseWriter, r *http.Request) {
	reqs, err := h.store.ListRequirements()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	// Enrich with child counts
	ids := make([]string, len(reqs))
	for i, req := range reqs {
		ids[i] = req.ID
	}
	childCounts, _ := h.store.GetRequirementChildCounts(ids)

	type enrichedReq struct {
		*models.Requirement
		ChildCount int `json:"child_count"`
	}
	result := make([]enrichedReq, len(reqs))
	for i, req := range reqs {
		result[i] = enrichedReq{
			Requirement: req,
			ChildCount:  childCounts[req.ID],
		}
	}

	httpx.JSON(w, http.StatusOK, result)
}

// handleCreateRequirement godoc
// @Summary      Create a requirement
// @Description  Creates a new requirement. Returns 409 if the identifier is already in use.
// @Tags         requirements
// @Accept       json
// @Produce      json
// @Param        body  body      object{identifier=string,title=string,description=string}  true  "Requirement payload"
// @Success      201  {object}  models.Requirement
// @Failure      400  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /requirements [post]
func (h *Handler) CreateRequirement(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Identifier  string `json:"identifier"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.Identifier == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "identifier is required"})
		return
	}
	if req.Title == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	r2 := &models.Requirement{
		Identifier:  req.Identifier,
		Title:       req.Title,
		Description: h.sanitizer.Sanitize(req.Description), // stored-XSS guard (F-011)
	}
	if err := h.store.CreateRequirement(r2); err != nil {
		if errors.Is(err, models.ErrDuplicateRequirementIdentifier) {
			httpx.Error(w, http.StatusConflict, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	h.broadcastRequirementUpdate("requirement:"+r2.ID, r2)

	httpx.JSON(w, http.StatusCreated, r2)
}

// handleGetRequirement godoc
// @Summary      Get a requirement
// @Description  Returns a single requirement by ID
// @Tags         requirements
// @Produce      json
// @Param        id  path      string  true  "Requirement ID"
// @Success      200  {object}  models.Requirement
// @Failure      404  {object}  map[string]string
// @Router       /requirements/{id} [get]
func (h *Handler) GetRequirement(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	req, err := h.store.GetRequirement(id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}
	httpx.JSON(w, http.StatusOK, req)
}

// handleUpdateRequirement godoc
// @Summary      Update a requirement
// @Description  Updates identifier, title, and/or description. Returns 409 if the new identifier conflicts.
// @Tags         requirements
// @Accept       json
// @Produce      json
// @Param        id    path      string  true  "Requirement ID"
// @Param        body  body      object{identifier=string,title=string,description=string}  true  "Requirement update payload"
// @Success      200  {object}  models.Requirement
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /requirements/{id} [put]
func (h *Handler) UpdateRequirement(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Verify the requirement exists first.
	existing, err := h.store.GetRequirement(id)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, err)
		return
	}

	var req struct {
		Identifier  string `json:"identifier"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.Identifier == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "identifier is required"})
		return
	}
	if req.Title == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	existing.Identifier = req.Identifier
	existing.Title = req.Title
	existing.Description = h.sanitizer.Sanitize(req.Description) // stored-XSS guard (F-011)

	if err := h.store.UpdateRequirement(existing); err != nil {
		if errors.Is(err, models.ErrDuplicateRequirementIdentifier) {
			httpx.Error(w, http.StatusConflict, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	h.broadcastRequirementUpdate("requirement:"+existing.ID, existing)

	httpx.JSON(w, http.StatusOK, existing)
}

// handleDeleteRequirement godoc
// @Summary      Delete a requirement
// @Description  Deletes a requirement and all its traceability links unconditionally. Frontend is responsible for confirmation prompt.
// @Tags         requirements
// @Param        id  path  string  true  "Requirement ID"
// @Success      204
// @Failure      500  {object}  map[string]string
// @Router       /requirements/{id} [delete]
func (h *Handler) DeleteRequirement(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.store.DeleteRequirement(id); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	h.broadcastRequirementUpdate("requirement:"+id, map[string]string{"id": id, "deleted": "true"})

	w.WriteHeader(http.StatusNoContent)
}

// BulkDeleteRequirements deletes multiple requirements by IDs.
func (h *Handler) BulkDeleteRequirements(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if len(req.IDs) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "ids must not be empty"})
		return
	}
	if len(req.IDs) > httpx.MaxBulkIDs {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many ids (max 500 per request)"})
		return
	}
	if err := h.store.DeleteRequirements(req.IDs); err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListChildren returns child requirements for a given parent.
func (h *Handler) ListChildren(w http.ResponseWriter, r *http.Request) {
	parentID := r.PathValue("id")
	children, err := h.store.ListChildRequirements(parentID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if children == nil {
		children = []*models.Requirement{}
	}
	httpx.JSON(w, http.StatusOK, children)
}

// ────────────────────────────────────────────────────────────────────────────
// Import handlers (011-jira-confluence-import)
// ────────────────────────────────────────────────────────────────────────────

// handleImportRequirement godoc
// @Summary      Import a requirement from Jira or Confluence
// @Description  Creates a requirement from an external source with source tracking.
// @Tags         requirements
// @Accept       json
// @Produce      json
// @Param        body  body  object{source_type=string,source_key=string}  true  "Import payload"
// @Success      201  {object}  models.Requirement
// @Failure      400  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Router       /requirements/import [post]
func (h *Handler) ImportRequirement(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceType      string `json:"source_type"`
		SourceKey       string `json:"source_key"`
		IncludeChildren bool   `json:"include_children"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.SourceType != "jira" && req.SourceType != "confluence" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "source_type must be 'jira' or 'confluence'"})
		return
	}
	if req.SourceKey == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "source_key is required"})
		return
	}

	// Check for existing import (duplicate detection)
	existing, err := h.store.FindRequirementBySource(req.SourceType, req.SourceKey)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if existing != nil {
		httpx.JSON(w, http.StatusConflict, map[string]interface{}{
			"error":       "A requirement from this source already exists",
			"existing_id": existing.ID,
		})
		return
	}

	var title, description, sourceURL string

	switch req.SourceType {
	case "jira":
		cfg, err := h.store.GetJiraConfig()
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		if cfg == nil || !cfg.Enabled || cfg.APIToken == "" {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Jira integration is not configured"})
			return
		}
		title, description, sourceURL, err = h.fetchJiraRequirement(cfg, req.SourceKey)
		if err != nil {
			httpx.JSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}

	case "confluence":
		cfg, err := h.store.GetConfluenceConfig()
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		if cfg == nil || !cfg.Enabled || cfg.APIToken == "" {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Confluence integration is not configured"})
			return
		}
		title, description, sourceURL, err = h.fetchConfluenceRequirement(cfg, req.SourceKey)
		if err != nil {
			httpx.JSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
	}

	newReq := &models.Requirement{
		Identifier:  req.SourceKey,
		Title:       title,
		Description: description,
		SourceType:  req.SourceType,
		SourceKey:   req.SourceKey,
		SourceURL:   sourceURL,
	}
	if err := h.store.CreateImportedRequirement(newReq); err != nil {
		if errors.Is(err, models.ErrDuplicateRequirementIdentifier) {
			// Identifier conflict (different from source duplicate)
			newReq.Identifier = fmt.Sprintf("%s-%s", req.SourceType, req.SourceKey)
			if err2 := h.store.CreateImportedRequirement(newReq); err2 != nil {
				httpx.Error(w, http.StatusConflict, err2)
				return
			}
		} else {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
	}

	// Import children if requested (Jira only)
	if req.IncludeChildren && req.SourceType == "jira" && h.fetchJiraChildren != nil {
		cfg, _ := h.store.GetJiraConfig()
		if cfg != nil && cfg.Enabled && cfg.APIToken != "" {
			children := h.fetchJiraChildren(cfg, req.SourceKey, h.sanitizer)
			for _, child := range children {
				// Skip already-imported children
				if existing, _ := h.store.FindRequirementBySource("jira", child.Key); existing != nil {
					// If already imported, just set its parent
					_ = h.store.SetRequirementParent(existing.ID, newReq.ID)
					continue
				}
				// Fetch full details for the child
				childTitle, childDesc, childURL, err := h.fetchJiraRequirement(cfg, child.Key)
				if err != nil {
					continue
				}
				childReq := &models.Requirement{
					Identifier:  child.Key,
					Title:       childTitle,
					Description: childDesc,
					ParentID:    &newReq.ID,
					SourceType:  "jira",
					SourceKey:   child.Key,
					SourceURL:   childURL,
				}
				if err := h.store.CreateImportedRequirement(childReq); err != nil {
					if errors.Is(err, models.ErrDuplicateRequirementIdentifier) {
						childReq.Identifier = fmt.Sprintf("jira-%s", child.Key)
						_ = h.store.CreateImportedRequirement(childReq)
					}
				}
			}
		}
	}

	httpx.JSON(w, http.StatusCreated, newReq)
}

// ────────────────────────────────────────────────────────────────────────────
// Re-sync & Unlink handlers (011-jira-confluence-import, US5)
// ────────────────────────────────────────────────────────────────────────────

// handleResync re-syncs a requirement from its external source (Jira/Confluence).
//
// @Summary      Re-sync requirement
// @Description  Fetch the latest version from the external source. Auto-updates if no local edits; returns a conflict payload if the user has edited locally.
// @Tags         requirements
// @Produce      json
// @Param        id  path  string  true  "Requirement ID"
// @Success      200  {object}  object{action=string}
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Router       /requirements/{id}/resync [post]
// @Security     BearerAuth
func (h *Handler) Resync(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	req, err := h.store.GetRequirementForResync(id)
	if err != nil {
		if err.Error() == "requirement not found" {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		} else {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return
	}

	// Fetch latest from source
	var remoteTitle, remoteDesc string
	switch req.SourceType {
	case "jira":
		cfg, err := h.store.GetJiraConfig()
		if err != nil || cfg == nil || !cfg.Enabled || cfg.APIToken == "" {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Jira integration is not configured"})
			return
		}
		remoteTitle, remoteDesc, _, err = h.fetchJiraRequirement(cfg, req.SourceKey)
		if err != nil {
			httpx.JSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
	case "confluence":
		cfg, err := h.store.GetConfluenceConfig()
		if err != nil || cfg == nil || !cfg.Enabled || cfg.APIToken == "" {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Confluence integration is not configured"})
			return
		}
		remoteTitle, remoteDesc, _, err = h.fetchConfluenceRequirement(cfg, req.SourceKey)
		if err != nil {
			httpx.JSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
			return
		}
	default:
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Unknown source type: " + req.SourceType})
		return
	}

	// Detect local edit: updated_at > imported_at means user edited after import
	hasLocalEdit := false
	if req.ImportedAt != nil {
		hasLocalEdit = req.UpdatedAt.After(*req.ImportedAt)
	}

	if !hasLocalEdit {
		// Auto-update
		updated, err := h.store.ApplyResyncUpdate(id, remoteTitle, remoteDesc)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		_ = h.store.CreateAuditLog(&models.AuditLog{
			ID:        uuid.New().String(),
			Action:    fmt.Sprintf("requirement:resynced:%s", id),
			Timestamp: time.Now(),
		})
		httpx.JSON(w, http.StatusOK, map[string]interface{}{
			"action":      "auto_updated",
			"requirement": updated,
		})
		return
	}

	// Conflict — return both local and remote for user to choose
	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"action": "conflict",
		"local": map[string]string{
			"title":       req.Title,
			"description": req.Description,
		},
		"remote": map[string]string{
			"title":       remoteTitle,
			"description": remoteDesc,
		},
	})
}

// handleResyncResolve resolves a resync conflict by accepting remote or keeping local.
//
// @Summary      Resolve resync conflict
// @Description  Resolve a requirement resync conflict by accepting the remote version or keeping the local version.
// @Tags         requirements
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Requirement ID"
// @Param        body  body  object{resolution=string,remote_title=string,remote_description=string}  true  "Resolution payload"
// @Success      200  {object}  object{id=string,resolution=string}
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /requirements/{id}/resync/resolve [post]
// @Security     BearerAuth
func (h *Handler) ResyncResolve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Resolution        string `json:"resolution"`
		RemoteTitle       string `json:"remote_title"`
		RemoteDescription string `json:"remote_description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}

	switch body.Resolution {
	case "accept_remote":
		// Client-supplied "remote" content bypasses the fetch+sanitize path, so
		// sanitize it here before storage (stored-XSS guard) (F-011).
		_, err := h.store.ApplyResyncUpdate(id, body.RemoteTitle, h.sanitizer.Sanitize(body.RemoteDescription))
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		_ = h.store.CreateAuditLog(&models.AuditLog{
			ID:        uuid.New().String(),
			Action:    fmt.Sprintf("requirement:resynced:%s", id),
			Timestamp: time.Now(),
		})
		httpx.JSON(w, http.StatusOK, map[string]string{"id": id, "resolution": "accept_remote"})
	case "keep_local":
		if err := h.store.MarkSynced(id); err != nil {
			httpx.Error(w, http.StatusInternalServerError, err)
			return
		}
		_ = h.store.CreateAuditLog(&models.AuditLog{
			ID:        uuid.New().String(),
			Action:    fmt.Sprintf("requirement:resync_kept_local:%s", id),
			Timestamp: time.Now(),
		})
		httpx.JSON(w, http.StatusOK, map[string]string{"id": id, "resolution": "keep_local"})
	default:
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "resolution must be 'accept_remote' or 'keep_local'"})
	}
}

// handleUnlink removes the external source link from a requirement.
//
// @Summary      Unlink requirement source
// @Description  Remove the external source (Jira/Confluence) link from a requirement, making it local-only.
// @Tags         requirements
// @Produce      json
// @Param        id  path  string  true  "Requirement ID"
// @Success      200  {object}  object{id=string,status=string}
// @Failure      500  {object}  map[string]string
// @Router       /requirements/{id}/unlink [post]
// @Security     BearerAuth
func (h *Handler) Unlink(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	_, err := h.store.UnlinkRequirement(id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	_ = h.store.CreateAuditLog(&models.AuditLog{
		ID:        uuid.New().String(),
		Action:    fmt.Sprintf("requirement:unlinked:%s", id),
		Timestamp: time.Now(),
	})
	httpx.JSON(w, http.StatusOK, map[string]string{"id": id, "status": "unlinked"})
}

// handleBulkImport godoc
// @Summary      Bulk import requirements from Jira or Confluence
// @Description  Imports multiple requirements. Skips already-imported sources and reports failures.
// @Tags         requirements
// @Accept       json
// @Produce      json
// @Param        body  body  object{source_type=string,source_keys=[]string}  true  "Bulk import payload"
// @Success      200  {object}  map[string]interface{}
// @Failure      400  {object}  map[string]string
// @Router       /requirements/bulk-import [post]
func (h *Handler) BulkImport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceType      string   `json:"source_type"`
		SourceKeys      []string `json:"source_keys"`
		IncludeChildren bool     `json:"include_children"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.SourceType != "jira" && req.SourceType != "confluence" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "source_type must be 'jira' or 'confluence'"})
		return
	}
	if len(req.SourceKeys) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "source_keys must not be empty"})
		return
	}
	// Bound the fan-out: each key triggers external Jira/Confluence fetches (F-028).
	if len(req.SourceKeys) > 200 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "too many source_keys (max 200 per request)"})
		return
	}

	type importedItem struct {
		SourceKey     string `json:"source_key"`
		RequirementID string `json:"requirement_id"`
		Identifier    string `json:"identifier"`
		Title         string `json:"title"`
	}
	type skippedItem struct {
		SourceKey             string `json:"source_key"`
		Reason                string `json:"reason"`
		ExistingRequirementID string `json:"existing_requirement_id,omitempty"`
	}
	type failedItem struct {
		SourceKey string `json:"source_key"`
		Reason    string `json:"reason"`
	}

	var imported []importedItem
	var skipped []skippedItem
	var failed []failedItem

	for _, key := range req.SourceKeys {
		// Check duplicate
		existing, err := h.store.FindRequirementBySource(req.SourceType, key)
		if err != nil {
			failed = append(failed, failedItem{SourceKey: key, Reason: err.Error()})
			continue
		}
		if existing != nil {
			skipped = append(skipped, skippedItem{SourceKey: key, Reason: "already_imported", ExistingRequirementID: existing.ID})
			continue
		}

		var title, description, sourceURL string

		switch req.SourceType {
		case "jira":
			cfg, err := h.store.GetJiraConfig()
			if err != nil || cfg == nil || !cfg.Enabled || cfg.APIToken == "" {
				failed = append(failed, failedItem{SourceKey: key, Reason: "Jira integration is not configured"})
				continue
			}
			title, description, sourceURL, err = h.fetchJiraRequirement(cfg, key)
			if err != nil {
				failed = append(failed, failedItem{SourceKey: key, Reason: err.Error()})
				continue
			}
		case "confluence":
			cfg, err := h.store.GetConfluenceConfig()
			if err != nil || cfg == nil || !cfg.Enabled || cfg.APIToken == "" {
				failed = append(failed, failedItem{SourceKey: key, Reason: "Confluence integration is not configured"})
				continue
			}
			title, description, sourceURL, err = h.fetchConfluenceRequirement(cfg, key)
			if err != nil {
				failed = append(failed, failedItem{SourceKey: key, Reason: err.Error()})
				continue
			}
		}

		newReq := &models.Requirement{
			Identifier:  key,
			Title:       title,
			Description: description,
			SourceType:  req.SourceType,
			SourceKey:   key,
			SourceURL:   sourceURL,
		}
		if err := h.store.CreateImportedRequirement(newReq); err != nil {
			if errors.Is(err, models.ErrDuplicateRequirementIdentifier) {
				// Try with prefixed identifier
				newReq.Identifier = fmt.Sprintf("%s-%s", req.SourceType, key)
				if err2 := h.store.CreateImportedRequirement(newReq); err2 != nil {
					failed = append(failed, failedItem{SourceKey: key, Reason: err2.Error()})
					continue
				}
			} else {
				failed = append(failed, failedItem{SourceKey: key, Reason: err.Error()})
				continue
			}
		}
		imported = append(imported, importedItem{
			SourceKey:     key,
			RequirementID: newReq.ID,
			Identifier:    newReq.Identifier,
			Title:         newReq.Title,
		})

		// Import children if requested (Jira only)
		if req.IncludeChildren && req.SourceType == "jira" && h.fetchJiraChildren != nil {
			cfg, _ := h.store.GetJiraConfig()
			if cfg != nil && cfg.Enabled && cfg.APIToken != "" {
				children := h.fetchJiraChildren(cfg, key, h.sanitizer)
				for _, child := range children {
					if existing, _ := h.store.FindRequirementBySource("jira", child.Key); existing != nil {
						_ = h.store.SetRequirementParent(existing.ID, newReq.ID)
						continue
					}
					childTitle, childDesc, childURL, err := h.fetchJiraRequirement(cfg, child.Key)
					if err != nil {
						continue
					}
					childReq := &models.Requirement{
						Identifier:  child.Key,
						Title:       childTitle,
						Description: childDesc,
						ParentID:    &newReq.ID,
						SourceType:  "jira",
						SourceKey:   child.Key,
						SourceURL:   childURL,
					}
					if err := h.store.CreateImportedRequirement(childReq); err != nil {
						if errors.Is(err, models.ErrDuplicateRequirementIdentifier) {
							childReq.Identifier = fmt.Sprintf("jira-%s", child.Key)
							_ = h.store.CreateImportedRequirement(childReq)
						}
					}
				}
			}
		}
	}

	if imported == nil {
		imported = []importedItem{}
	}
	if skipped == nil {
		skipped = []skippedItem{}
	}
	if failed == nil {
		failed = []failedItem{}
	}

	// Log aggregate audit event for bulk import
	if len(imported) > 0 {
		_ = h.store.CreateAuditLog(&models.AuditLog{
			ID:        uuid.New().String(),
			Action:    fmt.Sprintf("requirement:bulk_imported_%s:%d_items", req.SourceType, len(imported)),
			Timestamp: time.Now(),
		})
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"imported": imported,
		"skipped":  skipped,
		"failed":   failed,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Traceability link handlers
// ────────────────────────────────────────────────────────────────────────────

// handleCreateLink godoc
// @Summary      Link a test case to a requirement
// @Description  Creates a traceability link. Returns 409 if the link already exists, 404 if either entity is not found.
// @Tags         requirements
// @Accept       json
// @Produce      json
// @Param        id    path      string  true  "Requirement ID"
// @Param        body  body      object{test_case_id=string}  true  "Link payload"
// @Success      201  {object}  models.RequirementTestCaseLink
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /requirements/{id}/links [post]
func (h *Handler) CreateLink(w http.ResponseWriter, r *http.Request) {
	requirementID := r.PathValue("id")

	var req struct {
		TestCaseID string `json:"test_case_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.TestCaseID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "test_case_id is required"})
		return
	}

	link, err := h.store.CreateLink(requirementID, req.TestCaseID)
	if err != nil {
		if errors.Is(err, models.ErrDuplicateLink) {
			httpx.Error(w, http.StatusConflict, err)
			return
		}
		// "requirement not found" or "test case not found" → 404.
		httpx.Error(w, http.StatusNotFound, err)
		return
	}

	if req2, err := h.store.GetRequirement(requirementID); err == nil && req2 != nil {
		h.broadcastRequirementUpdate("requirement:"+requirementID, req2)
	}

	httpx.JSON(w, http.StatusCreated, link)
}

// handleDeleteLink godoc
// @Summary      Remove a traceability link
// @Description  Removes the link between a requirement and a test case. Returns 404 if no such link exists.
// @Tags         requirements
// @Param        id          path  string  true  "Requirement ID"
// @Param        testCaseId  path  string  true  "Test case ID"
// @Success      204
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /requirements/{id}/links/{testCaseId} [delete]
func (h *Handler) DeleteLink(w http.ResponseWriter, r *http.Request) {
	requirementID := r.PathValue("id")
	testCaseID := r.PathValue("testCaseId")

	if err := h.store.DeleteLink(requirementID, testCaseID); err != nil {
		if err.Error() == "link not found" {
			httpx.Error(w, http.StatusNotFound, err)
			return
		}
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}

	if req2, err := h.store.GetRequirement(requirementID); err == nil && req2 != nil {
		h.broadcastRequirementUpdate("requirement:"+requirementID, req2)
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListTestCaseRequirements godoc
// @Summary      List requirements linked to a test case
// @Description  Returns all requirements linked to the given test case, ordered by identifier.
// @Tags         requirements
// @Produce      json
// @Param        id  path      string  true  "Test case ID"
// @Success      200  {array}   models.Requirement
// @Failure      500  {object}  map[string]string
// @Router       /tests/{id}/requirements [get]
func (h *Handler) ListTestCaseRequirements(w http.ResponseWriter, r *http.Request) {
	testCaseID := r.PathValue("id")
	reqs, err := h.store.ListRequirementsByTestCase(testCaseID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	httpx.JSON(w, http.StatusOK, reqs)
}

// ────────────────────────────────────────────────────────────────────────────
// Post Test Cases to Jira (post-tests-to-jira feature)
// ────────────────────────────────────────────────────────────────────────────

func (h *Handler) PostToJira(w http.ResponseWriter, r *http.Request) {
	reqID := r.PathValue("id")

	// 1. Fetch requirement
	req, err := h.store.GetRequirement(reqID)
	if err != nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	// 2. Validate it's a Jira-sourced requirement
	if req.SourceType != "jira" || req.SourceKey == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Requirement is not linked to a Jira ticket"})
		return
	}

	// 3. Get Jira config
	cfg, err := h.store.GetJiraConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil || !cfg.Enabled || cfg.APIToken == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Jira integration is not configured"})
		return
	}

	// 4. Fetch linked test cases
	testCases, err := h.store.ListTestCasesByRequirement(reqID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if len(testCases) == 0 {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "No test cases linked to this requirement"})
		return
	}

	// Idempotency: skip re-posting when the linked test cases are unchanged since
	// the last successful post, so repeated calls don't spam duplicate comments (F-056).
	postHash := jiraPostHash(testCases)
	if req.LastJiraPostHash == postHash {
		httpx.JSON(w, http.StatusOK, map[string]string{"message": "No changes since the last post; skipped"})
		return
	}

	// 5. Build ADF comment as a table
	const maxDescLen = 200

	// Table header row
	headerRow := map[string]interface{}{
		"type": "tableRow",
		"content": []interface{}{
			adfTableHeader("#", 36),
			adfTableHeader("Test Case"),
			adfTableHeader("Description"),
		},
	}

	// Table body rows
	tableRows := []interface{}{headerRow}
	for i, tc := range testCases {
		desc := ""
		if tc.Description != "" {
			desc = bluemonday.StrictPolicy().Sanitize(tc.Description)
			desc = strings.TrimSpace(desc)
			if len(desc) > maxDescLen {
				desc = desc[:maxDescLen] + "…"
			}
		}
		tableRows = append(tableRows, map[string]interface{}{
			"type": "tableRow",
			"content": []interface{}{
				adfTableCell(fmt.Sprintf("%d", i+1), 36),
				adfTableCell(tc.Name),
				adfTableCell(desc),
			},
		})
	}

	adfBody := map[string]interface{}{
		"type":    "doc",
		"version": 1,
		"content": []interface{}{
			map[string]interface{}{
				"type": "paragraph",
				"content": []interface{}{
					map[string]interface{}{
						"type": "text",
						"text": fmt.Sprintf("Test Cases Generated from TTGO (%d):", len(testCases)),
						"marks": []interface{}{
							map[string]interface{}{"type": "strong"},
						},
					},
				},
			},
			map[string]interface{}{
				"type":    "table",
				"attrs":   map[string]interface{}{"isNumberColumnEnabled": false, "layout": "default"},
				"content": tableRows,
			},
		},
	}

	// 6. Post to Jira
	if err := h.postJiraComment(cfg, req.SourceKey, adfBody); err != nil {
		httpx.JSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	_ = h.store.SetRequirementJiraPostHash(reqID, postHash) // remember for idempotency (F-056)

	httpx.JSON(w, http.StatusOK, map[string]string{"message": "Test cases posted to Jira ticket"})
}

// adfTableHeader builds an ADF tableHeader cell with bold text and optional column width.
func adfTableHeader(text string, colwidth ...int) map[string]interface{} {
	cell := map[string]interface{}{
		"type": "tableHeader",
		"content": []interface{}{
			map[string]interface{}{
				"type": "paragraph",
				"content": []interface{}{
					map[string]interface{}{
						"type":  "text",
						"text":  text,
						"marks": []interface{}{map[string]interface{}{"type": "strong"}},
					},
				},
			},
		},
	}
	if len(colwidth) > 0 {
		cell["attrs"] = map[string]interface{}{"colwidth": []int{colwidth[0]}}
	}
	return cell
}

// adfTableCell builds an ADF tableCell with plain text and optional column width.
func adfTableCell(text string, colwidth ...int) map[string]interface{} {
	cell := map[string]interface{}{
		"type": "tableCell",
		"content": []interface{}{
			map[string]interface{}{
				"type": "paragraph",
				"content": []interface{}{
					map[string]interface{}{"type": "text", "text": text},
				},
			},
		},
	}
	if len(colwidth) > 0 {
		cell["attrs"] = map[string]interface{}{"colwidth": []int{colwidth[0]}}
	}
	return cell
}
