package jira

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
	"ttgo/internal/api/httpx"
	apiws "ttgo/internal/api/websocket"
	"ttgo/internal/safehttp"
	"ttgo/pkg/tracker/models"

	"github.com/microcosm-cc/bluemonday"
)

// jiraKeyRe matches a well-formed Jira issue key (e.g. PROJ-123). Validating a
// caller-supplied key before any outbound use prevents path traversal into the
// Jira host and JQL injection (F-027).
var jiraKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]+-[0-9]+$`)

// ValidJiraKey reports whether s is a well-formed Jira issue key.
func ValidJiraKey(s string) bool { return jiraKeyRe.MatchString(s) }

// ────────────────────────────────────────────────────────────────────────────
// Jira integration handlers (007-req-traceability)
// ────────────────────────────────────────────────────────────────────────────

// handleGetJiraConfig godoc
// @Summary      Get Jira integration configuration
// @Description  Returns the masked Jira configuration. Returns 404 if not yet configured.
// @Tags         jira
// @Produce      json
// @Success      200  {object}  models.JiraConfigResponse
// @Failure      404  {object}  map[string]string
// @Router       /settings/jira [get]
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.store.GetJiraConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil {
		httpx.JSON(w, http.StatusOK, map[string]interface{}{"enabled": false})
		return
	}
	httpx.JSON(w, http.StatusOK, cfg.MaskedConfig())
}

// handleUpsertJiraConfig godoc
// @Summary      Create or update Jira integration configuration
// @Description  Upserts the workspace Jira configuration. If api_token is empty, the existing token is preserved.
// @Tags         jira
// @Accept       json
// @Produce      json
// @Param        body  body      object{base_url=string,email=string,api_token=string,enabled=bool}  true  "Jira config payload"
// @Success      200  {object}  models.JiraConfigResponse
// @Failure      400  {object}  map[string]string
// @Router       /settings/jira [put]
func (h *Handler) UpsertConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BaseURL           string `json:"base_url"`
		Email             string `json:"email"`
		APIToken          string `json:"api_token"`
		Enabled           bool   `json:"enabled"`
		DefaultProjectKey string `json:"default_project_key"`
		DefaultIssueType  string `json:"default_issue_type"`
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
	// SSRF guard: the base URL drives server-side outbound requests with stored
	// credentials, so reject internal/metadata destinations at config time (F-003).
	if err := safehttp.ValidateIntegrationURL(strings.TrimSpace(req.BaseURL)); err != nil {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "base_url rejected: " + err.Error()})
		return
	}

	cfg, err := h.store.UpsertJiraConfig(req.BaseURL, req.Email, req.APIToken, req.Enabled, req.DefaultProjectKey, req.DefaultIssueType)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if h.hub != nil {
		h.hub.Broadcast(apiws.NewEvent(apiws.EventSettingsChanged, "settings:*", map[string]string{"integration": "jira"}))
	}

	httpx.JSON(w, http.StatusOK, cfg.MaskedConfig())
}

// handleFetchJiraTicket godoc
// @Summary      Fetch a Jira ticket (server-side proxy)
// @Description  Always returns HTTP 200. Check the `success` field. Returns 400 if Jira is not enabled/configured.
// @Tags         jira
// @Produce      json
// @Param        ticketId  path      string  true  "Jira issue key, e.g. PROJ-123"
// @Success      200  {object}  models.JiraTicketResult
// @Failure      400  {object}  map[string]string
// @Router       /jira/ticket/{ticketId} [get]
func (h *Handler) GetTicket(w http.ResponseWriter, r *http.Request) {
	ticketID := r.PathValue("ticketId")

	cfg, err := h.store.GetJiraConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil || !cfg.Enabled {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Jira integration is not enabled. Configure it in Settings."})
		return
	}
	if cfg.APIToken == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Jira integration is not configured (missing API token)."})
		return
	}

	result := FetchTicket(cfg, ticketID, h.sanitizer)

	// 011: Enrich with duplicate detection
	if result.Success {
		existing, _ := h.store.FindRequirementBySource("jira", result.Key)
		if existing != nil {
			result.AlreadyImported = true
			result.ExistingRequirementID = &existing.ID
		}
	}

	httpx.JSON(w, http.StatusOK, result)
}

// FetchTicket performs the actual Jira Cloud REST API call via jiraRequest.
// It always returns a JiraTicketResult — failures are encoded as success:false.
func FetchTicket(cfg *models.JiraConfig, ticketID string, sanitizer *bluemonday.Policy) models.JiraTicketResult {
	if !ValidJiraKey(ticketID) {
		return models.JiraTicketResult{Success: false, Identifier: ticketID, Error: "invalid Jira issue key format"}
	}
	base := strings.TrimRight(cfg.BaseURL, "/")

	path := fmt.Sprintf("/rest/api/3/issue/%s", url.PathEscape(ticketID))
	respBody, err := jiraRequest(cfg, http.MethodGet, path, nil)
	if err != nil {
		return models.JiraTicketResult{
			Success: false, Identifier: ticketID,
			Error: err.Error(),
		}
	}

	// Parse Jira issue JSON.
	var issue struct {
		Key    string `json:"key"`
		Fields struct {
			Summary     string          `json:"summary"`
			Description json.RawMessage `json:"description"`
			Status      struct {
				Name string `json:"name"`
			} `json:"status"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(respBody, &issue); err != nil {
		return models.JiraTicketResult{
			Success: false, Identifier: ticketID,
			Error: fmt.Sprintf("Failed to parse Jira response: %v", err),
		}
	}

	key := issue.Key
	if key == "" {
		key = ticketID
	}
	title := issue.Fields.Summary
	description := extractADFToHTML(issue.Fields.Description, sanitizer)
	ticketURL := fmt.Sprintf("%s/browse/%s", base, key)

	return models.JiraTicketResult{
		Success:     true,
		Identifier:  key,
		Key:         key,
		Title:       title,
		Description: description,
		Status:      issue.Fields.Status.Name,
		URL:         ticketURL,
	}
}

// extractADFText recursively walks an Atlassian Document Format (ADF) JSON tree
// and extracts plain text from all "text" leaf nodes. (Legacy — kept for backward compat.)
func extractADFText(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var node map[string]json.RawMessage
	if err := json.Unmarshal(raw, &node); err != nil {
		return ""
	}

	if nodeType, ok := node["type"]; ok {
		var t string
		if json.Unmarshal(nodeType, &t) == nil && t == "text" {
			if textVal, ok := node["text"]; ok {
				var s string
				if json.Unmarshal(textVal, &s) == nil {
					return s
				}
			}
		}
	}

	var sb strings.Builder
	if contentRaw, ok := node["content"]; ok {
		var children []json.RawMessage
		if json.Unmarshal(contentRaw, &children) == nil {
			for _, child := range children {
				text := extractADFText(child)
				if text != "" {
					if sb.Len() > 0 {
						sb.WriteByte(' ')
					}
					sb.WriteString(text)
				}
			}
		}
	}
	return sb.String()
}

// ────────────────────────────────────────────────────────────────────────────
// ADF-to-HTML conversion (011-jira-confluence-import, T005)
// ────────────────────────────────────────────────────────────────────────────

// extractADFToHTML converts an Atlassian Document Format (ADF) JSON tree to
// sanitized HTML, preserving headings, lists, links, tables, and text marks.
func extractADFToHTML(raw json.RawMessage, sanitizer *bluemonday.Policy) string {
	if raw == nil {
		return ""
	}
	var node map[string]json.RawMessage
	if err := json.Unmarshal(raw, &node); err != nil {
		return ""
	}
	html := renderADFNode(node)
	p := sanitizer
	return p.Sanitize(html)
}

func renderADFNode(node map[string]json.RawMessage) string {
	typeRaw, ok := node["type"]
	if !ok {
		return ""
	}
	var nodeType string
	if err := json.Unmarshal(typeRaw, &nodeType); err != nil {
		return ""
	}

	switch nodeType {
	case "doc":
		return renderADFChildren(node)
	case "paragraph":
		inner := renderADFChildren(node)
		return "<p>" + inner + "</p>"
	case "heading":
		level := 1
		if attrsRaw, ok := node["attrs"]; ok {
			var attrs struct {
				Level int `json:"level"`
			}
			if json.Unmarshal(attrsRaw, &attrs) == nil && attrs.Level >= 1 && attrs.Level <= 6 {
				level = attrs.Level
			}
		}
		inner := renderADFChildren(node)
		return fmt.Sprintf("<h%d>%s</h%d>", level, inner, level)
	case "bulletList":
		return "<ul>" + renderADFChildren(node) + "</ul>"
	case "orderedList":
		return "<ol>" + renderADFChildren(node) + "</ol>"
	case "listItem":
		return "<li>" + renderADFChildren(node) + "</li>"
	case "codeBlock":
		return "<pre><code>" + renderADFChildren(node) + "</code></pre>"
	case "blockquote":
		return "<blockquote>" + renderADFChildren(node) + "</blockquote>"
	case "table":
		return "<table>" + renderADFChildren(node) + "</table>"
	case "tableRow":
		return "<tr>" + renderADFChildren(node) + "</tr>"
	case "tableCell":
		return "<td>" + renderADFChildren(node) + "</td>"
	case "tableHeader":
		return "<th>" + renderADFChildren(node) + "</th>"
	case "hardBreak":
		return "<br>"
	case "rule":
		return "<hr>"
	case "text":
		textRaw, ok := node["text"]
		if !ok {
			return ""
		}
		var text string
		if err := json.Unmarshal(textRaw, &text); err != nil {
			return ""
		}
		return applyADFMarks(text, node)
	default:
		return renderADFChildren(node)
	}
}

func renderADFChildren(node map[string]json.RawMessage) string {
	contentRaw, ok := node["content"]
	if !ok {
		return ""
	}
	var children []json.RawMessage
	if err := json.Unmarshal(contentRaw, &children); err != nil {
		return ""
	}
	var sb strings.Builder
	for _, child := range children {
		var childNode map[string]json.RawMessage
		if json.Unmarshal(child, &childNode) == nil {
			sb.WriteString(renderADFNode(childNode))
		}
	}
	return sb.String()
}

func applyADFMarks(text string, node map[string]json.RawMessage) string {
	marksRaw, ok := node["marks"]
	if !ok {
		return text
	}
	var marks []struct {
		Type  string          `json:"type"`
		Attrs json.RawMessage `json:"attrs"`
	}
	if err := json.Unmarshal(marksRaw, &marks); err != nil {
		return text
	}
	result := text
	for _, mark := range marks {
		switch mark.Type {
		case "strong":
			result = "<strong>" + result + "</strong>"
		case "em":
			result = "<em>" + result + "</em>"
		case "code":
			result = "<code>" + result + "</code>"
		case "link":
			var attrs struct {
				Href string `json:"href"`
			}
			if json.Unmarshal(mark.Attrs, &attrs) == nil && attrs.Href != "" {
				result = fmt.Sprintf(`<a href="%s">%s</a>`, attrs.Href, result)
			}
		case "strike":
			result = "<s>" + result + "</s>"
		case "underline":
			result = "<u>" + result + "</u>"
		}
	}
	return result
}

// ────────────────────────────────────────────────────────────────────────────
// Fetch Children (epic child requirements)
// ────────────────────────────────────────────────────────────────────────────

type jiraChildIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			Name string `json:"name"`
		} `json:"status"`
	} `json:"fields"`
}

// FetchChildren returns child issues for an epic/parent ticket using multiple
// Jira API strategies with deduplication.
func FetchChildren(cfg *models.JiraConfig, parentKey string, sanitizer *bluemonday.Policy) []models.JiraTicketChild {
	// Reject malformed keys before building any outbound path/JQL (F-027): a
	// validated key cannot contain path separators or JQL metacharacters.
	if !ValidJiraKey(parentKey) {
		return nil
	}
	base := strings.TrimRight(cfg.BaseURL, "/")
	seen := make(map[string]bool)
	var children []models.JiraTicketChild

	addIssues := func(issues []jiraChildIssue) {
		for _, issue := range issues {
			if seen[issue.Key] {
				continue
			}
			seen[issue.Key] = true
			children = append(children, models.JiraTicketChild{
				Key:    issue.Key,
				Title:  issue.Fields.Summary,
				Status: issue.Fields.Status.Name,
				URL:    fmt.Sprintf("%s/browse/%s", base, issue.Key),
			})
		}
	}

	// Strategy 1: Jira Agile API (official endpoint for epic children)
	agileURL := fmt.Sprintf("/rest/agile/1.0/epic/%s/issue?maxResults=100&fields=summary,status", parentKey)
	if respBody, err := jiraRequest(cfg, http.MethodGet, agileURL, nil); err == nil {
		var agileResp struct {
			Issues []jiraChildIssue `json:"issues"`
		}
		if json.Unmarshal(respBody, &agileResp) == nil {
			addIssues(agileResp.Issues)
		}
	}

	// Strategy 2: JQL queries (parent field + Epic Link custom field)
	jqlQueries := []string{
		fmt.Sprintf(`parent = %s OR "Epic Link" = %s ORDER BY key ASC`, parentKey, parentKey),
		fmt.Sprintf("parent = %s ORDER BY key ASC", parentKey),
	}
	for _, jql := range jqlQueries {
		searchBody := fmt.Sprintf(`{"jql":%q,"fields":["summary","status"],"maxResults":100}`, jql)
		respBody, err := jiraRequest(cfg, http.MethodPost, "/rest/api/3/search", strings.NewReader(searchBody))
		if err != nil {
			continue
		}
		var jiraResp struct {
			Issues []jiraChildIssue `json:"issues"`
		}
		if json.Unmarshal(respBody, &jiraResp) == nil {
			addIssues(jiraResp.Issues)
			break
		}
	}

	return children
}

// ────────────────────────────────────────────────────────────────────────────
// Jira Search (US3 — Bulk Import)
// ────────────────────────────────────────────────────────────────────────────

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JQL        string `json:"jql"`
		StartAt    int    `json:"start_at"`
		MaxResults int    `json:"max_results"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.Error(w, http.StatusBadRequest, err)
		return
	}
	if req.JQL == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "jql is required"})
		return
	}
	if req.MaxResults <= 0 || req.MaxResults > 100 {
		req.MaxResults = 25
	}

	cfg, err := h.store.GetJiraConfig()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err)
		return
	}
	if cfg == nil || !cfg.Enabled || cfg.APIToken == "" {
		httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Jira integration is not configured"})
		return
	}

	// Build JQL search request body
	searchBody := fmt.Sprintf(`{"jql":%q,"fields":["summary","description","status"],"startAt":%d,"maxResults":%d}`,
		req.JQL, req.StartAt, req.MaxResults)

	respBody, err := jiraRequest(cfg, http.MethodPost, "/rest/api/3/search", strings.NewReader(searchBody))
	if err != nil {
		// Check if it's a JQL syntax error
		if strings.Contains(err.Error(), "returned HTTP 400") {
			httpx.JSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JQL query"})
			return
		}
		httpx.JSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	var jiraResp struct {
		Issues []struct {
			Key    string `json:"key"`
			Fields struct {
				Summary string `json:"summary"`
				Status  struct {
					Name string `json:"name"`
				} `json:"status"`
			} `json:"fields"`
		} `json:"issues"`
		Total      int `json:"total"`
		StartAt    int `json:"startAt"`
		MaxResults int `json:"maxResults"`
	}
	if err := json.Unmarshal(respBody, &jiraResp); err != nil {
		httpx.JSON(w, http.StatusBadGateway, map[string]string{"error": "Failed to parse Jira search response"})
		return
	}

	base := strings.TrimRight(cfg.BaseURL, "/")

	type ticketResult struct {
		Key             string `json:"key"`
		Summary         string `json:"summary"`
		Status          string `json:"status"`
		URL             string `json:"url"`
		AlreadyImported bool   `json:"already_imported"`
	}

	tickets := make([]ticketResult, len(jiraResp.Issues))
	for i, issue := range jiraResp.Issues {
		tickets[i] = ticketResult{
			Key:     issue.Key,
			Summary: issue.Fields.Summary,
			Status:  issue.Fields.Status.Name,
			URL:     fmt.Sprintf("%s/browse/%s", base, issue.Key),
		}
		existing, _ := h.store.FindRequirementBySource("jira", issue.Key)
		if existing != nil {
			tickets[i].AlreadyImported = true
		}
	}

	httpx.JSON(w, http.StatusOK, map[string]interface{}{
		"tickets":     tickets,
		"total":       jiraResp.Total,
		"start_at":    jiraResp.StartAt,
		"max_results": jiraResp.MaxResults,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Jira HTTP helper (011-jira-confluence-import)
// ────────────────────────────────────────────────────────────────────────────

// jiraRequest performs an authenticated HTTP request to the Jira Cloud REST API.
// Returns the response body and nil error on success; returns a user-friendly
// error message on failure per the error pattern table in contracts.
func jiraRequest(cfg *models.JiraConfig, method, path string, body io.Reader) ([]byte, error) {
	base := strings.TrimRight(cfg.BaseURL, "/")
	url := base + path

	client := safehttp.IntegrationClient(10 * time.Second) // SSRF guard, allows self-hosted private hosts (F-003)
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("Jira is unreachable — check your connection and try again")
	}

	creds := base64.StdEncoding.EncodeToString([]byte(cfg.Email + ":" + cfg.APIToken))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Jira is unreachable — check your connection and try again")
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxExternalResponseSize))

	switch {
	case resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated:
		return respBody, nil
	case resp.StatusCode == http.StatusUnauthorized:
		return nil, fmt.Errorf("Jira credentials are invalid — check your API token in Settings")
	case resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("You do not have permission to access this resource in Jira")
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("Jira resource could not be found")
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("Jira rate limit reached — wait a moment and try again")
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("Jira is temporarily unavailable")
	default:
		return nil, fmt.Errorf("Jira returned HTTP %d", resp.StatusCode)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Jira Comment Posting
// ────────────────────────────────────────────────────────────────────────────

// PostComment posts an ADF-formatted comment to a Jira issue.
// The adfBody should be a valid ADF document node (the "body" value).
func PostComment(cfg *models.JiraConfig, issueKey string, adfBody interface{}) error {
	if !ValidJiraKey(issueKey) {
		return fmt.Errorf("invalid Jira issue key %q", issueKey)
	}
	payload := map[string]interface{}{
		"body": adfBody,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to build Jira comment request: %v", err)
	}

	path := fmt.Sprintf("/rest/api/3/issue/%s/comment", url.PathEscape(issueKey))
	_, err = jiraRequest(cfg, http.MethodPost, path, strings.NewReader(string(body)))
	return err
}
