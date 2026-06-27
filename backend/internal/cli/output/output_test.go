package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]string{{"id": "1", "name": "Test"}}
	if err := Print(&buf, "json", data, nil); err != nil {
		t.Fatalf("Print() error: %v", err)
	}
	var result []map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if result[0]["name"] != "Test" {
		t.Errorf("name = %q, want Test", result[0]["name"])
	}
}

func TestTableOutput(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]interface{}{
		{"id": "abc-123", "name": "Login Test", "status": "PASS"},
		{"id": "def-456", "name": "Logout Test", "status": "FAIL"},
	}
	columns := []Column{
		{Header: "ID", Key: "id"},
		{Header: "NAME", Key: "name"},
		{Header: "STATUS", Key: "status"},
	}
	if err := Print(&buf, "table", data, columns); err != nil {
		t.Fatalf("Print() error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "ID") {
		t.Error("missing header ID")
	}
	if !strings.Contains(out, "Login Test") {
		t.Error("missing Login Test")
	}
	if !strings.Contains(out, "FAIL") {
		t.Error("missing FAIL")
	}
}

func TestPlainOutput(t *testing.T) {
	var buf bytes.Buffer
	data := []map[string]interface{}{
		{"id": "abc", "name": "Test"},
	}
	columns := []Column{
		{Header: "ID", Key: "id"},
		{Header: "NAME", Key: "name"},
	}
	if err := Print(&buf, "plain", data, columns); err != nil {
		t.Fatalf("Print() error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "abc\tTest") {
		t.Errorf("plain output = %q, want tab-separated values", out)
	}
}

func TestRawJSONOutput(t *testing.T) {
	var buf bytes.Buffer
	raw := json.RawMessage(`{"id":"1","name":"Test"}`)
	if err := PrintRaw(&buf, "json", raw); err != nil {
		t.Fatalf("PrintRaw() error: %v", err)
	}
	if !strings.Contains(buf.String(), `"id"`) {
		t.Error("missing id in raw output")
	}
}
