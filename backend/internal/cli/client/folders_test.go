package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetFolderTree(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/folders/tree" {
			t.Errorf("path = %q, want /api/folders/tree", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"id": "f1", "name": "Root", "sub_folders": []interface{}{}},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	result, err := c.GetFolderTree()
	if err != nil {
		t.Fatalf("GetFolderTree() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(result))
	}
}

func TestCreateFolder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "New Folder" {
			t.Errorf("name = %q, want New Folder", body["name"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "new-id", "name": "New Folder"})
	}))
	defer srv.Close()

	c := New(srv.URL, "tok")
	result, err := c.CreateFolder("New Folder", nil)
	if err != nil {
		t.Fatalf("CreateFolder() error: %v", err)
	}
	if result["id"] != "new-id" {
		t.Errorf("id = %v, want new-id", result["id"])
	}
}
