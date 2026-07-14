package aigen

import (
	"encoding/json"
	"errors"
	"strings"
)

// CriticSchemaName identifies the critic response schema.
const CriticSchemaName = "ttgo_critic_findings"

// CriticSchemaJSON is the strict schema for the optional LLM critic pass.
// The critic adds semantic findings only; it can never override the
// deterministic structural validator.
var CriticSchemaJSON = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["findings"],
  "properties": {
    "findings": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["draft_index", "dimension", "message"],
        "properties": {
          "draft_index": {"type": "integer", "description": "0-based index into the reviewed drafts array"},
          "dimension": {"type": "string", "description": "short axis label, e.g. relevance, uniqueness, clarity"},
          "message": {"type": "string", "description": "one-sentence human-readable finding"}
        }
      }
    }
  }
}`)

// CriticFinding is one semantic finding from the critic.
type CriticFinding struct {
	DraftIndex int    `json:"draft_index"`
	Dimension  string `json:"dimension"`
	Message    string `json:"message"`
}

// ParseCriticResponse parses the critic reply (markdown-fence tolerant).
func ParseCriticResponse(raw string) ([]CriticFinding, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(strings.TrimSpace(raw), "```")
	var envelope struct {
		Findings []CriticFinding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &envelope); err != nil {
		return nil, errors.New("critic reply is not the expected findings JSON")
	}
	return envelope.Findings, nil
}
