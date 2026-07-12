// Package aigen defines the canonical AI-generation contract: the JSON Schema
// sent to schema-capable providers and the deterministic draft validator.
// It is provider-agnostic and has no HTTP or store dependencies.
package aigen

import "encoding/json"

// EnvelopeSchemaName identifies the schema in provider json_schema requests.
const EnvelopeSchemaName = "ttgo_test_cases"

// EnvelopeSchemaJSON is the strict JSON Schema for the canonical response
// envelope {"test_cases":[...]}. It intentionally avoids keywords OpenAI's
// strict mode rejects (minItems, maxLength); hard limits are enforced by the
// deterministic validator instead.
var EnvelopeSchemaJSON = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["test_cases"],
  "properties": {
    "test_cases": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["name", "category", "description", "source_refs", "steps"],
        "properties": {
          "name": {"type": "string", "description": "Unique descriptive test title"},
          "category": {"type": "string", "description": "Test category, e.g. Functional, Negative, Boundary"},
          "description": {"type": "string", "description": "One sentence: what this test validates"},
          "source_refs": {"type": "array", "items": {"type": "string"}, "description": "IDs of covered acceptance criteria or child requirements; empty if none"},
          "steps": {
            "type": "array",
            "items": {
              "type": "object",
              "additionalProperties": false,
              "required": ["action", "expected_result"],
              "properties": {
                "action": {"type": "string"},
                "expected_result": {"type": "string"}
              }
            }
          }
        }
      }
    }
  }
}`)
