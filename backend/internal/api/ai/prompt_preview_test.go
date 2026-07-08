package ai_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"ttgo/pkg/tracker/models"
)

type previewSegment struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Empty bool   `json:"empty"`
}

type previewResponse struct {
	SystemMessage   string           `json:"system_message"`
	TemplateType    string           `json:"template_type"`
	MaxTokens       int              `json:"max_tokens"`
	TemplateWarning string           `json:"template_warning"`
	Segments        []previewSegment `json:"segments"`
}

func findPreviewSeg(t *testing.T, segs []previewSegment, typ string) previewSegment {
	t.Helper()
	for _, s := range segs {
		if s.Type == typ {
			return s
		}
	}
	t.Fatalf("no %q segment in response", typ)
	return previewSegment{}
}

func createPreviewRequirement(t *testing.T, env *testEnv, identifier, title, description string) string {
	t.Helper()
	rr := doRequest(env, "POST", "/api/requirements", map[string]string{
		"identifier": identifier, "title": title, "description": description,
	})
	if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
		t.Fatalf("create requirement: %d %s", rr.Code, rr.Body.String())
	}
	var resp struct {
		ID string `json:"id"`
	}
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.ID == "" {
		t.Fatal("no requirement id in response")
	}
	return resp.ID
}

func TestPromptPreview_NoRequirement(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()

	rr := doRequest(env, "POST", "/api/ai-gen/prompt-preview", map[string]string{
		"coverage_level": "thorough",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rr.Code, rr.Body.String())
	}
	var out previewResponse
	json.NewDecoder(rr.Body).Decode(&out)

	if out.SystemMessage == "" {
		t.Fatal("system_message missing")
	}
	if out.TemplateType != "standard" {
		t.Fatalf("template_type: %q", out.TemplateType)
	}
	if out.MaxTokens != 8192 { // thorough default
		t.Fatalf("max_tokens: %d", out.MaxTokens)
	}
	if s := findPreviewSeg(t, out.Segments, "title"); !s.Empty || s.Text != "" {
		t.Fatalf("title should be empty-flagged: %+v", s)
	}
	if s := findPreviewSeg(t, out.Segments, "description"); !s.Empty {
		t.Fatalf("description should be empty-flagged: %+v", s)
	}
	if s := findPreviewSeg(t, out.Segments, "coverage"); s.Text == "" {
		t.Fatal("coverage guidance empty")
	}
	if s := findPreviewSeg(t, out.Segments, "detail"); s.Text != "Standard" {
		t.Fatalf("detail default: %+v", s)
	}
}

func TestPromptPreview_LinkedRequirement(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	reqID := createPreviewRequirement(t, env, "REQ-PP-1", "Login flow", "Users must be able to log in.")

	rr := doRequest(env, "POST", "/api/ai-gen/prompt-preview", map[string]string{
		"requirement_id":          reqID,
		"coverage_level":          "essential",
		"detail_level":            "Detailed",
		"additional_instructions": "focus on edge cases",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rr.Code, rr.Body.String())
	}
	var out previewResponse
	json.NewDecoder(rr.Body).Decode(&out)

	if s := findPreviewSeg(t, out.Segments, "title"); s.Text != "Login flow" || s.Empty {
		t.Fatalf("title: %+v", s)
	}
	if s := findPreviewSeg(t, out.Segments, "description"); !strings.Contains(s.Text, "log in") {
		t.Fatalf("description: %+v", s)
	}
	if s := findPreviewSeg(t, out.Segments, "detail"); s.Text != "Detailed" {
		t.Fatalf("detail: %+v", s)
	}
	if s := findPreviewSeg(t, out.Segments, "instructions"); s.Text != "Additional Instructions: focus on edge cases" {
		t.Fatalf("instructions: %+v", s)
	}
	if out.MaxTokens != 4096 { // essential default
		t.Fatalf("max_tokens: %d", out.MaxTokens)
	}
	// no children → no children segment
	for _, s := range out.Segments {
		if s.Type == "children" {
			t.Fatalf("unexpected children segment: %+v", s)
		}
	}
}

func TestPromptPreview_WithChildren(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	parentID := createPreviewRequirement(t, env, "REQ-PP-P", "Epic", "Parent epic.")

	child := &models.Requirement{
		Identifier: "REQ-PP-C1", Title: "Child one", Description: "First child.",
		ParentID: &parentID,
	}
	if err := env.store.CreateRequirement(child); err != nil {
		t.Fatalf("create child: %v", err)
	}

	rr := doRequest(env, "POST", "/api/ai-gen/prompt-preview", map[string]string{
		"requirement_id": parentID,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rr.Code, rr.Body.String())
	}
	var out previewResponse
	json.NewDecoder(rr.Body).Decode(&out)

	if out.TemplateType != "parent" {
		t.Fatalf("template_type: %q", out.TemplateType)
	}
	if s := findPreviewSeg(t, out.Segments, "children"); !strings.Contains(s.Text, "REQ-PP-C1") {
		t.Fatalf("children segment: %+v", s)
	}
	if s := findPreviewSeg(t, out.Segments, "coverage"); !strings.Contains(s.Text, "child issues/sub-tickets") {
		t.Fatalf("coverage booster missing: %+v", s)
	}
}

func TestPromptPreview_UnknownCoverage(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	rr := doRequest(env, "POST", "/api/ai-gen/prompt-preview", map[string]string{
		"coverage_level": "maximal",
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status: %d %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "coverage_level must be one of") {
		t.Fatalf("error body: %s", rr.Body.String())
	}
}

func TestPromptPreview_RequirementNotFound(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	rr := doRequest(env, "POST", "/api/ai-gen/prompt-preview", map[string]string{
		"requirement_id": "does-not-exist",
	})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status: %d %s", rr.Code, rr.Body.String())
	}
}

func TestPromptPreview_RequiresAuth(t *testing.T) {
	env, cleanup := testServer(t)
	defer cleanup()
	req := httptest.NewRequest("POST", "/api/ai-gen/prompt-preview", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	env.srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", rr.Code)
	}
}
