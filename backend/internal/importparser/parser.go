// Package importparser provides deterministic format parsers for importing
// test cases from various text formats (JSON, markdown table, CSV, numbered/
// bulleted lists). All functions are pure — they operate on strings and return
// model structs with no HTTP, store, or server dependencies.
package importparser

import (
	"encoding/csv"
	"fmt"
	"regexp"
	"strings"
	"ttgo/pkg/tracker/models"
)

// ────────────────────────────────────────────────────────────────────────────
// Regex vars
// ────────────────────────────────────────────────────────────────────────────

var (
	MdTableSepRegex   = regexp.MustCompile(`^\|[\s-:|]+\|$`)
	NumberedListRegex = regexp.MustCompile(`(?m)^\s*(\d+[\.\)]\s|[-*]\s)`)
	numberedItemRegex = regexp.MustCompile(`(?m)^\s*\d+[\.\)]\s`)
	bulletItemRegex   = regexp.MustCompile(`(?m)^\s*[-*]\s`)
	subItemRegex      = regexp.MustCompile(`^\s{2,}[-*]\s|^\s{2,}\d+[\.\)]\s`)
)

// LLMResponseParser is a function type that parses raw LLM JSON output into
// generated test cases. This is injected by the caller so that the importparser
// package does not depend on the ai_generation module.
type LLMResponseParser func(raw string) ([]models.GeneratedTestCase, error)

// ────────────────────────────────────────────────────────────────────────────
// Format detection
// ────────────────────────────────────────────────────────────────────────────

// DetectFormat auto-detects the format of the raw content.
// Returns one of: "json", "markdown_table", "csv", "numbered_list", or "".
func DetectFormat(raw string) string {
	trimmed := strings.TrimSpace(raw)

	// JSON: starts with [ or {
	if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") {
		return "json"
	}

	// Markdown table: contains | delimiters with header separator row
	if strings.Contains(trimmed, "|") {
		lines := strings.Split(trimmed, "\n")
		for _, line := range lines {
			stripped := strings.TrimSpace(line)
			if MdTableSepRegex.MatchString(stripped) {
				return "markdown_table"
			}
		}
	}

	// CSV: first line looks like a comma-separated header with known column names
	lines := strings.Split(trimmed, "\n")
	if len(lines) > 1 {
		firstLine := strings.ToLower(strings.TrimSpace(lines[0]))
		csvKeywords := []string{"name", "description", "action", "expected_result", "steps", "expected result", "test case", "test name"}
		if strings.Contains(firstLine, ",") {
			matches := 0
			for _, kw := range csvKeywords {
				if strings.Contains(firstLine, kw) {
					matches++
				}
			}
			if matches >= 2 {
				return "csv"
			}
		}
	}

	// Numbered/bulleted list: lines starting with number+dot/paren or bullet
	if NumberedListRegex.MatchString(trimmed) {
		return "numbered_list"
	}

	return ""
}

// ────────────────────────────────────────────────────────────────────────────
// Orchestrator
// ────────────────────────────────────────────────────────────────────────────

// ParseImportContent orchestrates format detection and parsing.
// Returns parsed test cases, unparseable items, detected format name, and error.
// The llmParser parameter is used by ParseJSON to delegate JSON/LLM response parsing.
func ParseImportContent(raw string, formatHint string, llmParser LLMResponseParser) ([]models.GeneratedTestCase, []models.UnparseableItem, string, error) {
	format := formatHint
	if format == "" {
		format = DetectFormat(raw)
	}

	var testCases []models.GeneratedTestCase
	var unparseable []models.UnparseableItem
	var err error

	switch format {
	case "json":
		testCases, unparseable, err = ParseJSON(raw, llmParser)
	case "markdown_table":
		testCases, unparseable, err = ParseMarkdownTable(raw)
	case "csv":
		testCases, unparseable, err = ParseCSV(raw)
	case "numbered_list":
		testCases, unparseable, err = ParseNumberedList(raw)
	default:
		// Try all parsers in order as fallback.
		testCases, unparseable, err = ParseJSON(raw, llmParser)
		if len(testCases) > 0 {
			format = "json"
			return testCases, unparseable, format, nil
		}
		testCases, unparseable, err = ParseMarkdownTable(raw)
		if len(testCases) > 0 {
			format = "markdown_table"
			return testCases, unparseable, format, nil
		}
		testCases, unparseable, err = ParseCSV(raw)
		if len(testCases) > 0 {
			format = "csv"
			return testCases, unparseable, format, nil
		}
		testCases, unparseable, err = ParseNumberedList(raw)
		if len(testCases) > 0 {
			format = "numbered_list"
			return testCases, unparseable, format, nil
		}
		return nil, nil, "", fmt.Errorf("unable to parse content in any supported format")
	}

	if err != nil && len(testCases) == 0 {
		return nil, nil, format, err
	}
	return testCases, unparseable, format, nil
}

// ────────────────────────────────────────────────────────────────────────────
// JSON parser — delegates to the injected LLM response parser
// ────────────────────────────────────────────────────────────────────────────

// ParseJSON parses JSON content by delegating to the provided LLM response parser,
// which handles JSON arrays, object wrappers, <think> blocks, markdown fences,
// and scattered JSON objects.
func ParseJSON(raw string, llmParser LLMResponseParser) ([]models.GeneratedTestCase, []models.UnparseableItem, error) {
	drafts, err := llmParser(raw)
	if err != nil {
		return nil, nil, err
	}
	return drafts, nil, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Markdown table parser
// ────────────────────────────────────────────────────────────────────────────

// ParseMarkdownTable parses a markdown table into test cases.
func ParseMarkdownTable(raw string) ([]models.GeneratedTestCase, []models.UnparseableItem, error) {
	lines := strings.Split(raw, "\n")

	// Find header row and separator row.
	headerIdx := -1
	for i, line := range lines {
		stripped := strings.TrimSpace(line)
		if MdTableSepRegex.MatchString(stripped) && i > 0 {
			headerIdx = i - 1
			break
		}
	}
	if headerIdx < 0 {
		return nil, nil, fmt.Errorf("no markdown table found")
	}

	// Parse header columns.
	headerCells := SplitTableRow(lines[headerIdx])
	colMap := MapColumns(headerCells)
	if colMap["name"] < 0 {
		return nil, nil, fmt.Errorf("markdown table missing 'name' column")
	}

	// Parse data rows (skip header + separator).
	var testCases []models.GeneratedTestCase
	var unparseable []models.UnparseableItem

	for i := headerIdx + 2; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" || !strings.Contains(line, "|") {
			continue
		}
		cells := SplitTableRow(line)

		name := GetCell(cells, colMap["name"])
		if strings.TrimSpace(name) == "" {
			unparseable = append(unparseable, models.UnparseableItem{
				LineNumber: i + 1, RawText: line, Reason: "empty test case name",
			})
			continue
		}

		tc := models.GeneratedTestCase{
			Name:        strings.TrimSpace(name),
			Description: strings.TrimSpace(GetCell(cells, colMap["description"])),
			Category:    strings.TrimSpace(GetCell(cells, colMap["category"])),
		}

		// Build steps from action/expected_result columns or a combined "steps" column.
		action := strings.TrimSpace(GetCell(cells, colMap["action"]))
		expected := strings.TrimSpace(GetCell(cells, colMap["expected_result"]))
		stepsRaw := strings.TrimSpace(GetCell(cells, colMap["steps"]))

		if action != "" || expected != "" {
			// Split multi-step entries on numbered items or semicolons.
			actions := SplitMultiStep(action)
			expecteds := SplitMultiStep(expected)
			maxLen := len(actions)
			if len(expecteds) > maxLen {
				maxLen = len(expecteds)
			}
			for j := 0; j < maxLen; j++ {
				step := models.GeneratedStep{}
				if j < len(actions) {
					step.Action = strings.TrimSpace(actions[j])
				}
				if j < len(expecteds) {
					step.ExpectedResult = strings.TrimSpace(expecteds[j])
				}
				tc.Steps = append(tc.Steps, step)
			}
		} else if stepsRaw != "" {
			tc.Steps = ParseStepsFromText(stepsRaw)
		}

		testCases = append(testCases, tc)
	}

	if len(testCases) == 0 {
		return nil, unparseable, fmt.Errorf("no test cases parsed from markdown table")
	}
	return testCases, unparseable, nil
}

// SplitTableRow splits a markdown table row into cells, trimming outer pipes.
func SplitTableRow(row string) []string {
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "|")
	row = strings.TrimSuffix(row, "|")
	parts := strings.Split(row, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// MapColumns maps known column names to their index in the header.
// Returns -1 for columns not found.
func MapColumns(headers []string) map[string]int {
	m := map[string]int{
		"name": -1, "description": -1, "category": -1,
		"action": -1, "expected_result": -1, "steps": -1,
	}
	aliases := map[string][]string{
		"name":            {"name", "test case", "test name", "test case name", "title"},
		"description":     {"description", "desc", "summary"},
		"category":        {"category", "type", "test type"},
		"action":          {"action", "actions", "step", "steps action", "test steps"},
		"expected_result": {"expected result", "expected_result", "expected", "expected outcome", "result"},
		"steps":           {"steps", "test steps", "procedure"},
	}
	for key, aliasList := range aliases {
		for i, h := range headers {
			lower := strings.ToLower(strings.TrimSpace(h))
			for _, alias := range aliasList {
				if lower == alias {
					m[key] = i
					break
				}
			}
			if m[key] >= 0 {
				break
			}
		}
	}
	return m
}

// GetCell returns the cell value at the given index, or "" if out of bounds.
func GetCell(cells []string, idx int) string {
	if idx < 0 || idx >= len(cells) {
		return ""
	}
	return cells[idx]
}

// SplitMultiStep splits a cell value that may contain multiple steps separated
// by semicolons or numbered items (e.g., "1. Do X; 2. Do Y").
func SplitMultiStep(s string) []string {
	if s == "" {
		return nil
	}
	// Try numbered splits first: "1. foo 2. bar"
	parts := regexp.MustCompile(`\d+[\.\)]\s`).Split(s, -1)
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) > 1 {
		return result
	}
	// Try semicolons.
	if strings.Contains(s, ";") {
		parts = strings.Split(s, ";")
		result = nil
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 1 {
			return result
		}
	}
	return []string{s}
}

// ────────────────────────────────────────────────────────────────────────────
// Numbered/bulleted list parser
// ────────────────────────────────────────────────────────────────────────────

// ParseNumberedList parses numbered or bulleted list content into test cases.
func ParseNumberedList(raw string) ([]models.GeneratedTestCase, []models.UnparseableItem, error) {
	lines := strings.Split(raw, "\n")
	var testCases []models.GeneratedTestCase
	var unparseable []models.UnparseableItem
	var current *models.GeneratedTestCase
	var currentStepLines []string
	inSubItems := false

	flushCurrent := func(lineNum int) {
		if current == nil {
			return
		}
		// Flush any remaining step lines.
		if len(currentStepLines) > 0 {
			current.Steps = append(current.Steps, ParseStepsFromLines(currentStepLines)...)
			currentStepLines = nil
		}
		if strings.TrimSpace(current.Name) != "" {
			testCases = append(testCases, *current)
		} else {
			unparseable = append(unparseable, models.UnparseableItem{
				LineNumber: lineNum, RawText: "(empty test case name)", Reason: "no name detected",
			})
		}
		current = nil
		inSubItems = false
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Check if this is a top-level numbered/bulleted item (test case boundary).
		isTopLevel := false
		if numberedItemRegex.MatchString(line) && !subItemRegex.MatchString(line) {
			isTopLevel = true
		} else if bulletItemRegex.MatchString(line) && !subItemRegex.MatchString(line) {
			isTopLevel = true
		}

		if isTopLevel {
			flushCurrent(i + 1)
			// Extract the name: strip the bullet/number prefix.
			name := regexp.MustCompile(`^\s*(\d+[\.\)]\s*|[-*]\s*)`).ReplaceAllString(line, "")
			name = strings.TrimSpace(name)

			// Check if name contains description after a colon or dash separator.
			desc := ""
			if idx := strings.Index(name, ": "); idx > 0 && idx < 80 {
				desc = strings.TrimSpace(name[idx+2:])
				name = strings.TrimSpace(name[:idx])
			} else if idx := strings.Index(name, " - "); idx > 0 && idx < 80 {
				desc = strings.TrimSpace(name[idx+3:])
				name = strings.TrimSpace(name[:idx])
			}

			current = &models.GeneratedTestCase{
				Name:        name,
				Description: desc,
			}
			currentStepLines = nil
			inSubItems = true
			continue
		}

		// Sub-item: part of the current test case (step or description continuation).
		if current != nil && inSubItems {
			// Strip sub-item bullet/number prefix.
			stepText := regexp.MustCompile(`^\s*(\d+[\.\)]\s*|[-*]\s*)`).ReplaceAllString(line, "")
			stepText = strings.TrimSpace(stepText)
			if stepText != "" {
				currentStepLines = append(currentStepLines, stepText)
			}
			continue
		}

		// Line doesn't match any pattern and no current context — skip as unparseable.
		if current == nil {
			// Only add to unparseable if it looks like meaningful content (not blank/commentary).
			if len(trimmed) > 5 && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "//") {
				unparseable = append(unparseable, models.UnparseableItem{
					LineNumber: i + 1, RawText: trimmed, Reason: "no test case structure detected",
				})
			}
		}
	}
	flushCurrent(len(lines))

	if len(testCases) == 0 {
		return nil, unparseable, fmt.Errorf("no test cases parsed from numbered list")
	}
	return testCases, unparseable, nil
}

// ParseStepsFromLines converts a list of sub-item text lines into steps.
// Attempts to split action/expected_result on various separators.
func ParseStepsFromLines(lines []string) []models.GeneratedStep {
	var steps []models.GeneratedStep
	for _, line := range lines {
		step := SplitActionExpected(line)
		steps = append(steps, step)
	}
	return steps
}

// ParseStepsFromText splits a single text blob into steps (used by markdown table parser).
func ParseStepsFromText(text string) []models.GeneratedStep {
	// Try splitting on newlines, numbered items, or semicolons.
	var lines []string
	if strings.Contains(text, "\n") {
		lines = strings.Split(text, "\n")
	} else {
		lines = SplitMultiStep(text)
	}
	var steps []models.GeneratedStep
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		trimmed = regexp.MustCompile(`^\s*(\d+[\.\)]\s*|[-*]\s*)`).ReplaceAllString(trimmed, "")
		step := SplitActionExpected(strings.TrimSpace(trimmed))
		steps = append(steps, step)
	}
	return steps
}

// SplitActionExpected splits a step line into action and expected result.
// Recognizes separators: "->", "=>", "Expected:", "expected:".
func SplitActionExpected(text string) models.GeneratedStep {
	separators := []string{" \u2192 ", " => ", " Expected: ", " expected: ", " | Expected: ", " | expected: "}
	for _, sep := range separators {
		if idx := strings.Index(text, sep); idx > 0 {
			return models.GeneratedStep{
				Action:         strings.TrimSpace(text[:idx]),
				ExpectedResult: strings.TrimSpace(text[idx+len(sep):]),
			}
		}
	}
	return models.GeneratedStep{Action: text}
}

// ────────────────────────────────────────────────────────────────────────────
// CSV parser
// ────────────────────────────────────────────────────────────────────────────

// ParseCSV parses CSV content into test cases.
func ParseCSV(raw string) ([]models.GeneratedTestCase, []models.UnparseableItem, error) {
	reader := csv.NewReader(strings.NewReader(raw))
	reader.FieldsPerRecord = -1 // allow variable column counts
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("CSV parse error: %w", err)
	}
	if len(records) < 2 {
		return nil, nil, fmt.Errorf("CSV needs at least a header and one data row")
	}

	// Map headers.
	colMap := MapColumns(records[0])
	if colMap["name"] < 0 {
		return nil, nil, fmt.Errorf("CSV missing 'name' column in header")
	}

	var testCases []models.GeneratedTestCase
	var unparseable []models.UnparseableItem

	// Track multi-row test cases (same name = continuation with additional steps).
	tcMap := make(map[string]int) // name -> index in testCases

	for i := 1; i < len(records); i++ {
		row := records[i]
		name := strings.TrimSpace(GetCell(row, colMap["name"]))
		if name == "" {
			unparseable = append(unparseable, models.UnparseableItem{
				LineNumber: i + 1, RawText: strings.Join(row, ","), Reason: "empty test case name",
			})
			continue
		}

		action := strings.TrimSpace(GetCell(row, colMap["action"]))
		expected := strings.TrimSpace(GetCell(row, colMap["expected_result"]))
		stepsRaw := strings.TrimSpace(GetCell(row, colMap["steps"]))

		// Check if this is a continuation row (same test case name).
		if idx, exists := tcMap[name]; exists && (action != "" || expected != "") {
			step := models.GeneratedStep{Action: action, ExpectedResult: expected}
			testCases[idx].Steps = append(testCases[idx].Steps, step)
			continue
		}

		tc := models.GeneratedTestCase{
			Name:        name,
			Description: strings.TrimSpace(GetCell(row, colMap["description"])),
			Category:    strings.TrimSpace(GetCell(row, colMap["category"])),
		}

		// Build steps.
		if action != "" || expected != "" {
			actions := SplitMultiStep(action)
			expecteds := SplitMultiStep(expected)
			maxLen := len(actions)
			if len(expecteds) > maxLen {
				maxLen = len(expecteds)
			}
			for j := 0; j < maxLen; j++ {
				step := models.GeneratedStep{}
				if j < len(actions) {
					step.Action = strings.TrimSpace(actions[j])
				}
				if j < len(expecteds) {
					step.ExpectedResult = strings.TrimSpace(expecteds[j])
				}
				tc.Steps = append(tc.Steps, step)
			}
		} else if stepsRaw != "" {
			tc.Steps = ParseStepsFromText(stepsRaw)
		}

		tcMap[name] = len(testCases)
		testCases = append(testCases, tc)
	}

	if len(testCases) == 0 {
		return nil, unparseable, fmt.Errorf("no test cases parsed from CSV")
	}
	return testCases, unparseable, nil
}
