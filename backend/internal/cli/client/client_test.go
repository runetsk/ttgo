package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", r.Header.Get("Authorization"))
		}
		if r.Method != http.MethodGet {
			t.Errorf("Method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/api/folders/tree" {
			t.Errorf("Path = %q, want /api/folders/tree", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]string{{"id": "abc", "name": "Root"}})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token")
	var result []map[string]string
	err := c.Get("/api/folders/tree", nil, &result)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if len(result) != 1 || result[0]["name"] != "Root" {
		t.Errorf("result = %v, want [{id:abc name:Root}]", result)
	}
}

func TestPostWithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "My Folder" {
			t.Errorf("body name = %q, want My Folder", body["name"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "new-id", "name": "My Folder"})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token")
	body := map[string]string{"name": "My Folder"}
	var result map[string]string
	err := c.Post("/api/folders", body, &result)
	if err != nil {
		t.Fatalf("Post() error: %v", err)
	}
	if result["id"] != "new-id" {
		t.Errorf("result id = %q, want new-id", result["id"])
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "folder not found"})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token")
	var result map[string]string
	err := c.Get("/api/folders/bad-id", nil, &result)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
	if apiErr.Message != "folder not found" {
		t.Errorf("Message = %q, want 'folder not found'", apiErr.Message)
	}
}

func TestConnectionRefused(t *testing.T) {
	c := New("http://127.0.0.1:19999", "token")
	var result interface{}
	err := c.Get("/api/anything", nil, &result)
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestQueryParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("folder_id") != "abc" {
			t.Errorf("folder_id = %q, want abc", r.URL.Query().Get("folder_id"))
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("limit = %q, want 10", r.URL.Query().Get("limit"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-token")
	params := map[string]string{"folder_id": "abc", "limit": "10"}
	var result map[string]string
	err := c.Get("/api/tests", params, &result)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
}
