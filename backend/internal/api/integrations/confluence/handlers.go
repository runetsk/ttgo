package confluence

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"ttgo/internal/api/httpx"
	apiws "ttgo/internal/api/websocket"
	"ttgo/internal/safehttp"
	"ttgo/pkg/tracker/models"

	"github.com/microcosm-cc/bluemonday"
)

// ────────────────────────────────────────────────────────────────────────────
// Confluence HTTP helper & page fetcher (011-jira-confluence-import)
// ────────────────────────────────────────────────────────────────────────────

// ConfluencePageResult holds the fetched data for a single Confluence page.
type ConfluencePageResult struct {
	ID       string
	Title    string
	SpaceID  string
	BodyHTML string
	URL      string
}

// isNumericID reports whether s is a non-empty all-digits string (Confluence IDs).
func isNumericID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// confluenceRequest performs an authenticated HTTP request to the Confluence Cloud REST API.
func confluenceRequest(cfg *models.ConfluenceConfig, method, path string, body io.Reader) ([]byte, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	url := base + path

	// SSRF-guarded client: blocks metadata/link-local, allows self-hosted private hosts (F-003).
	client := safehttp.IntegrationClient(10 * time.Second)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("Confluence is unreachable — check your connection and try again")
	}

	creds := base64.StdEncoding.EncodeToString([]byte(cfg.Email + ":" + cfg.APIToken))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Confluence is unreachable — check your connection and try again")
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxExternalResponseSize))

	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated:
		return respBody, nil
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, fmt.Errorf("Confluence credentials are invalid — check your API token in Settings")
	case resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("You do not have permission to access this resource in Confluence")
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("Confluence resource could not be found")
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("Confluence rate limit reached — wait a moment and try again")
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("Confluence is temporarily unavailable")
	default:
		return nil, fmt.Errorf("Confluence returned HTTP %d", resp.StatusCode)
	}
}

// ConfluenceSpaceResult holds summary data for a Confluence space.
type ConfluenceSpaceResult struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// FetchConfluenceSpaces lists available Confluence spaces with cursor pagination.
func FetchConfluenceSpaces(cfg *models.ConfluenceConfig, cursor string, limit int) ([]ConfluenceSpaceResult, string, error) {
	if limit <= 0 || limit > 250 {
		limit = 50
	}
	path := fmt.Sprintf("/wiki/api/v2/spaces?limit=%d", limit)
	if cursor != "" {
		path += "&cursor=" + url.QueryEscape(cursor)
	}

	body, err := confluenceRequest(cfg, http.MethodGet, path, nil)
	if err != nil {
		return nil, "", err
	}

	var resp struct {
		Results []struct {
			ID   string `json:"id"`
			Key  string `json:"key"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"results"`
		Links struct {
			Next string `json:"next"`
		} `json:"_links"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, "", fmt.Errorf("failed to parse Confluence spaces response: %v", err)
	}

	spaces := make([]ConfluenceSpaceResult, len(resp.Results))
	for i, s := range resp.Results {
		spaces[i] = ConfluenceSpaceResult{ID: s.ID, Key: s.Key, Name: s.Name, Type: s.Type}
	}

	// Extract cursor from next link if present
	nextCursor := ""
	if resp.Links.Next != "" {
		// The next link is a full URL; extract the cursor param
		if idx := strings.Index(resp.Links.Next, "cursor="); idx >= 0 {
			nextCursor = resp.Links.Next[idx+7:]
			if ampIdx := strings.Index(nextCursor, "&"); ampIdx >= 0 {
				nextCursor = nextCursor[:ampIdx]
			}
		}
	}

	return spaces, nextCursor, nil
}

// ConfluencePageListItem holds summary data for a page in a list.
type ConfluencePageListItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	SpaceID  string `json:"space_id,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
	Status   string `json:"status"`
	URL      string `json:"url,omitempty"`
}

// confluencePageResponse is the shape returned by GET /spaces/{id}/pages (v2).
// Fields are a superset — child page responses may omit some (e.g. _links).
type confluencePageResponse struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	SpaceID  string `json:"spaceId"`
	ParentID string `json:"parentId"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	Links    struct {
		WebUI string `json:"webui"`
	} `json:"_links"`
}

// parseConfluencePages converts raw API page objects into ConfluencePageListItems.
func parseConfluencePages(cfg *models.ConfluenceConfig, results []confluencePageResponse) []ConfluencePageListItem {
	base := strings.TrimRight(cfg.BaseURL, "/")
	pages := make([]ConfluencePageListItem, 0, len(results))
	for _, p := range results {
		// The /children endpoint can return non-page types (whiteboards, databases);
		// skip anything that isn't a page.
		if p.Type != "" && p.Type != "page" {
			continue
		}
		pageURL := ""
		if p.Links.WebUI != "" {
			pageURL = base + "/wiki" + p.Links.WebUI
		}
		pages = append(pages, ConfluencePageListItem{
			ID:       p.ID,
			Title:    p.Title,
			SpaceID:  p.SpaceID,
			ParentID: p.ParentID,
			Status:   p.Status,
			URL:      pageURL,
		})
	}
	return pages
}

// extractNextCursor pulls the cursor value from a Confluence _links.next URL.
func extractNextCursor(nextLink string) string {
	if nextLink == "" {
		return ""
	}
	if idx := strings.Index(nextLink, "cursor="); idx >= 0 {
		c := nextLink[idx+7:]
		if ampIdx := strings.Index(c, "&"); ampIdx >= 0 {
			c = c[:ampIdx]
		}
		return c
	}
	return ""
}

// FetchConfluencePages lists pages in a space.
// v2 endpoint: GET /wiki/api/v2/spaces/{spaceId}/pages
// Supported params: cursor, limit, status, title, sort, body-format
func FetchConfluencePages(cfg *models.ConfluenceConfig, spaceID, title, label, cursor string, limit int) ([]ConfluencePageListItem, string, error) {
	if limit <= 0 || limit > 250 {
		limit = 50
	}

	path := fmt.Sprintf("/wiki/api/v2/spaces/%s/pages?status=current&limit=%d", url.PathEscape(spaceID), limit)
	if title != "" {
		path += "&title=" + url.QueryEscape(title)
	}
	// Note: "label" is not a v2 param — kept in signature for backward compat but ignored.
	if cursor != "" {
		path += "&cursor=" + url.QueryEscape(cursor)
	}

	body, err := confluenceRequest(cfg, http.MethodGet, path, nil)
	if err != nil {
		return nil, "", err
	}

	var resp struct {
		Results []confluencePageResponse `json:"results"`
		Links   struct {
			Next string `json:"next"`
		} `json:"_links"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, "", fmt.Errorf("failed to parse Confluence pages response: %v", err)
	}

	pages := parseConfluencePages(cfg, resp.Results)

	return pages, extractNextCursor(resp.Links.Next), nil
}

// FetchConfluenceChildPages fetches direct child pages of a given page.
// v2 endpoint: GET /wiki/api/v2/pages/{pageId}/children
// Supported params: cursor, limit, sort, status
func FetchConfluenceChildPages(cfg *models.ConfluenceConfig, parentID, cursor string, limit int) ([]ConfluencePageListItem, string, error) {
	if limit <= 0 || limit > 250 {
		limit = 50
	}

	path := fmt.Sprintf("/wiki/api/v2/pages/%s/children?limit=%d", url.PathEscape(parentID), limit)
	if cursor != "" {
		path += "&cursor=" + url.QueryEscape(cursor)
	}

	body, err := confluenceRequest(cfg, http.MethodGet, path, nil)
	if err != nil {
		return nil, "", err
	}

	var resp struct {
		Results []confluencePageResponse `json:"results"`
		Links   struct {
			Next string `json:"next"`
		} `json:"_links"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, "", fmt.Errorf("failed to parse Confluence child pages response: %v", err)
	}

	// parseConfluencePages filters out non-page types (whiteboards, databases, etc.)
	pages := parseConfluencePages(cfg, resp.Results)

	return pages, extractNextCursor(resp.Links.Next), nil
}

// FetchConfluencePage fetches a single Confluence page by ID with its body content.
func FetchConfluencePage(cfg *models.ConfluenceConfig, pageID string, sanitizer *bluemonday.Policy) (*ConfluencePageResult, error) {
	// Confluence page IDs are numeric; reject anything else before it drives an
	// outbound request path (prevents path traversal / SSRF-path injection) (F-027).
	if !isNumericID(pageID) {
		return nil, fmt.Errorf("invalid Confluence page id %q", pageID)
	}
	path := fmt.Sprintf("/wiki/api/v2/pages/%s?body-format=storage", url.PathEscape(pageID))
	body, err := confluenceRequest(cfg, http.MethodGet, path, nil)
	if err != nil {
		if strings.Contains(err.Error(), "could not be found") {
			return nil, fmt.Errorf("Confluence page %s could not be found", pageID)
		}
		return nil, err
	}

	var page struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		SpaceID string `json:"spaceId"`
		Body    struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
		Links struct {
			WebUI string `json:"webui"`
		} `json:"_links"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("failed to parse Confluence response: %v", err)
	}

	// Sanitize the HTML body
	p := sanitizer
	sanitizedHTML := p.Sanitize(page.Body.Storage.Value)

	// Construct full URL
	base := strings.TrimRight(cfg.BaseURL, "/")
	pageURL := base + "/wiki" + page.Links.WebUI
	if page.Links.WebUI == "" {
		pageURL = fmt.Sprintf("%s/wiki/spaces/pages/%s", base, pageID)
	}

	return &ConfluencePageResult{
		ID:       page.ID,
		Title:    page.Title,
		SpaceID:  page.SpaceID,
		BodyHTML: sanitizedHTML,
		URL:      pageURL,
	}, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Confluence API handlers (T014)
// ────────────────────────────────────────────────────────────────────────────

// handleGetConfluenceConfig returns the current Confluence integration configuration.
//
// @Summary      Get Confluence config
// @Description  Return the current Confluence integration configuration.
// @Tags         confluence
// @Produce      json
// @Success      200  {object}  object
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /settings/confluence [get]
// @Security     BearerAuth
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetConfluenceConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil {
		httpx.JSON(w, http.StatusNotFound, map[string]string{"error": "Confluence integration is not configured"})
		return
	}
	httpx.JSON(w, http.StatusOK, cfg.ToResponse())
}

// handleUpsertConfluenceConfig creates or updates the Confluence integration configuration.
//
// @Summary      Upsert Confluence config
// @Description  Create or update the Confluence integration configuration (base URL, email, API token).
// @Tags         confluence
// @Accept       json
// @Produce      json
// @Param        body  body  object{base_url=string,email=string,api_token=string,enabled=bool}  true  "Confluence config"
// @Success      200  {object}  object
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /settings/confluence [put]
// @Security     BearerAuth
func (h *Handler) UpsertConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BaseURL  string `json:"base_url"`
		Email    string `json:"email"`
		APIToken string `json:"api_token"`
		Enabled  bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.BaseURL) == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "base_url is required"})
		return
	}
	if strings.TrimSpace(req.Email) == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}
	if err := safehttp.ValidateIntegrationURL(strings.TrimSpace(req.BaseURL)); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "base_url rejected: " + err.Error()})
		return
	}

	cfg, err := h.store.UpsertConfluenceConfig(req.BaseURL, req.Email, req.APIToken, req.Enabled)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventSettingsChanged, "settings:*", map[string]string{"integration": "confluence"}))
	}

	httpx.JSON(w, http.StatusOK, cfg.ToResponse())
}

// handleListConfluenceSpaces lists available Confluence spaces.
//
// @Summary      List Confluence spaces
// @Description  Fetch available Confluence spaces with cursor-based pagination.
// @Tags         confluence
// @Produce      json
// @Param        cursor  query  string  false  "Pagination cursor"
// @Param        limit   query  int     false  "Page size (max 250)"  default(50)
// @Success      200  {object}  object{spaces=[]object,next_cursor=string}
// @Failure      400  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Router       /confluence/spaces [get]
// @Security     BearerAuth
func (h *Handler) ListSpaces(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetConfluenceConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil || !cfg.Enabled || cfg.APIToken == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Confluence integration is not configured"})
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	spaces, nextCursor, err := FetchConfluenceSpaces(cfg, cursor, limit)
	if err != nil {
		httpx.JSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	result := map[string]interface{}{
		"spaces": spaces,
	}
	if nextCursor != "" {
		result["next_cursor"] = nextCursor
	} else {
		result["next_cursor"] = nil
	}
	httpx.JSON(w, http.StatusOK, result)
}

// handleListConfluencePages lists pages in a Confluence space.
//
// @Summary      List Confluence pages
// @Description  Fetch pages in a Confluence space with optional title filter and cursor pagination. Includes already_imported flag.
// @Tags         confluence
// @Produce      json
// @Param        space_id  query  string  true   "Confluence space ID"
// @Param        title     query  string  false  "Filter by page title"
// @Param        label     query  string  false  "Filter by label (reserved)"
// @Param        cursor    query  string  false  "Pagination cursor"
// @Param        limit     query  int     false  "Page size"  default(25)
// @Success      200  {object}  object{pages=[]object,next_cursor=string}
// @Failure      400  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Router       /confluence/pages [get]
// @Security     BearerAuth
func (h *Handler) ListPages(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetConfluenceConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil || !cfg.Enabled || cfg.APIToken == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Confluence integration is not configured"})
		return
	}

	spaceID := r.URL.Query().Get("space_id")
	if spaceID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "space_id is required"})
		return
	}

	title := r.URL.Query().Get("title")
	label := r.URL.Query().Get("label")
	cursor := r.URL.Query().Get("cursor")
	limit := 25
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	pages, nextCursor, err := FetchConfluencePages(cfg, spaceID, title, label, cursor, limit)
	if err != nil {
		httpx.JSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	// Enrich with already_imported flag
	type enrichedPage struct {
		ConfluencePageListItem
		AlreadyImported bool `json:"already_imported"`
	}
	enrichedPages := make([]enrichedPage, len(pages))
	for i, p := range pages {
		enrichedPages[i] = enrichedPage{ConfluencePageListItem: p}
		existing, _ := h.store.FindRequirementBySource("confluence", p.ID)
		if existing != nil {
			enrichedPages[i].AlreadyImported = true
		}
	}

	result := map[string]interface{}{
		"pages": enrichedPages,
	}
	if nextCursor != "" {
		result["next_cursor"] = nextCursor
	} else {
		result["next_cursor"] = nil
	}
	httpx.JSON(w, http.StatusOK, result)
}

// handleListConfluenceChildPages lists child pages of a Confluence page.
//
// @Summary      List Confluence child pages
// @Description  Fetch direct child pages of a given Confluence page with cursor pagination. Includes already_imported flag.
// @Tags         confluence
// @Produce      json
// @Param        pageId  path   string  true   "Parent page ID"
// @Param        cursor  query  string  false  "Pagination cursor"
// @Param        limit   query  int     false  "Page size"  default(50)
// @Success      200  {object}  object{pages=[]object,next_cursor=string}
// @Failure      400  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Router       /confluence/pages/{pageId}/children [get]
// @Security     BearerAuth
func (h *Handler) ListChildPages(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetConfluenceConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil || !cfg.Enabled || cfg.APIToken == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Confluence integration is not configured"})
		return
	}

	parentID := r.PathValue("pageId")
	if parentID == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "pageId is required"})
		return
	}

	cursor := r.URL.Query().Get("cursor")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	pages, nextCursor, err := FetchConfluenceChildPages(cfg, parentID, cursor, limit)
	if err != nil {
		httpx.JSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	// Enrich with already_imported flag
	type enrichedPage struct {
		ConfluencePageListItem
		AlreadyImported bool `json:"already_imported"`
	}
	enrichedPages := make([]enrichedPage, len(pages))
	for i, p := range pages {
		enrichedPages[i] = enrichedPage{ConfluencePageListItem: p}
		existing, _ := h.store.FindRequirementBySource("confluence", p.ID)
		if existing != nil {
			enrichedPages[i].AlreadyImported = true
		}
	}

	result := map[string]interface{}{
		"pages": enrichedPages,
	}
	if nextCursor != "" {
		result["next_cursor"] = nextCursor
	} else {
		result["next_cursor"] = nil
	}
	httpx.JSON(w, http.StatusOK, result)
}

// handleGetConfluencePage fetches a single Confluence page with its HTML body.
//
// @Summary      Get Confluence page
// @Description  Fetch a single Confluence page by ID with sanitized HTML body content. Includes already_imported flag.
// @Tags         confluence
// @Produce      json
// @Param        pageId  path  string  true  "Confluence page ID"
// @Success      200  {object}  object{id=string,title=string,body_html=string,already_imported=bool}
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Router       /confluence/pages/{pageId} [get]
// @Security     BearerAuth
func (h *Handler) GetPage(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetConfluenceConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil || !cfg.Enabled || cfg.APIToken == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Confluence integration is not configured"})
		return
	}

	pageID := r.PathValue("pageId")
	page, err := FetchConfluencePage(cfg, pageID, h.sanitizer)
	if err != nil {
		if strings.Contains(err.Error(), "could not be found") {
			httpx.JSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		} else {
			httpx.JSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		return
	}

	// Check if already imported
	alreadyImported := false
	var existingID *string
	existing, _ := h.store.FindRequirementBySource("confluence", pageID)
	if existing != nil {
		alreadyImported = true
		existingID = &existing.ID
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"id":                      page.ID,
		"title":                   page.Title,
		"space_id":                page.SpaceID,
		"body_html":               page.BodyHTML,
		"url":                     page.URL,
		"already_imported":        alreadyImported,
		"existing_requirement_id": existingID,
	})
}
