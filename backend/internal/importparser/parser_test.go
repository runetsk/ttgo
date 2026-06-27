package importparser_test

import (
	"errors"
	"testing"
	"ttgo/internal/importparser"
	"ttgo/pkg/tracker/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubLLM returns a parser that yields the given drafts/err.
func stubLLM(drafts []models.GeneratedTestCase, err error) importparser.LLMResponseParser {
	return func(raw string) ([]models.GeneratedTestCase, error) {
		return drafts, err
	}
}

func TestDetectFormat(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"json array", `[{"name": "x"}]`, "json"},
		{"json object", `{"tests": []}`, "json"},
		{"markdown table", "| Name | Desc |\n|------|------|\n| foo | bar |", "markdown_table"},
		{"csv header", "name,description\nfoo,bar", "csv"},
		{"csv single keyword insufficient", "name,notes\nfoo,bar", ""},
		{"numbered list", "1. Test one\n2. Test two", "numbered_list"},
		{"bulleted list", "- Test one\n- Test two", "numbered_list"},
		{"empty", "", ""},
		{"plain prose", "just some paragraph\nno structure here", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, importparser.DetectFormat(tc.raw))
		})
	}
}

func TestSplitTableRow(t *testing.T) {
	got := importparser.SplitTableRow("| a | b | c |")
	assert.Equal(t, []string{"a", "b", "c"}, got)

	got = importparser.SplitTableRow("a|b|c")
	assert.Equal(t, []string{"a", "b", "c"}, got)
}

func TestMapColumns(t *testing.T) {
	m := importparser.MapColumns([]string{"Name", "Description", "Action", "Expected Result", "Category"})
	assert.Equal(t, 0, m["name"])
	assert.Equal(t, 1, m["description"])
	assert.Equal(t, 2, m["action"])
	assert.Equal(t, 3, m["expected_result"])
	assert.Equal(t, 4, m["category"])
	assert.Equal(t, -1, m["steps"])

	// Alias: "Title" maps to "name".
	m2 := importparser.MapColumns([]string{"Title", "Summary"})
	assert.Equal(t, 0, m2["name"])
	assert.Equal(t, 1, m2["description"])
}

func TestGetCell(t *testing.T) {
	cells := []string{"a", "b", "c"}
	assert.Equal(t, "a", importparser.GetCell(cells, 0))
	assert.Equal(t, "c", importparser.GetCell(cells, 2))
	assert.Equal(t, "", importparser.GetCell(cells, -1))
	assert.Equal(t, "", importparser.GetCell(cells, 5))
}

func TestSplitMultiStep(t *testing.T) {
	assert.Nil(t, importparser.SplitMultiStep(""))

	// Numbered splits.
	got := importparser.SplitMultiStep("1. Do X 2. Do Y 3. Do Z")
	assert.Equal(t, []string{"Do X", "Do Y", "Do Z"}, got)

	// Semicolons.
	got = importparser.SplitMultiStep("Do X; Do Y; Do Z")
	assert.Equal(t, []string{"Do X", "Do Y", "Do Z"}, got)

	// Single step.
	got = importparser.SplitMultiStep("just one step")
	assert.Equal(t, []string{"just one step"}, got)
}

func TestSplitActionExpected(t *testing.T) {
	// Arrow separator.
	s := importparser.SplitActionExpected("Click button \u2192 Page loads")
	assert.Equal(t, "Click button", s.Action)
	assert.Equal(t, "Page loads", s.ExpectedResult)

	// => separator.
	s = importparser.SplitActionExpected("Type text => Text appears")
	assert.Equal(t, "Type text", s.Action)
	assert.Equal(t, "Text appears", s.ExpectedResult)

	// Expected: separator.
	s = importparser.SplitActionExpected("Do thing Expected: result")
	assert.Equal(t, "Do thing", s.Action)
	assert.Equal(t, "result", s.ExpectedResult)

	// No separator — entire text is action.
	s = importparser.SplitActionExpected("just an action")
	assert.Equal(t, "just an action", s.Action)
	assert.Empty(t, s.ExpectedResult)
}

func TestParseStepsFromLines(t *testing.T) {
	steps := importparser.ParseStepsFromLines([]string{
		"Click login \u2192 Login page opens",
		"Enter credentials",
	})
	require.Len(t, steps, 2)
	assert.Equal(t, "Click login", steps[0].Action)
	assert.Equal(t, "Login page opens", steps[0].ExpectedResult)
	assert.Equal(t, "Enter credentials", steps[1].Action)
}

func TestParseStepsFromText(t *testing.T) {
	// Newline-separated.
	steps := importparser.ParseStepsFromText("1. First step\n2. Second step")
	require.Len(t, steps, 2)
	assert.Equal(t, "First step", steps[0].Action)
	assert.Equal(t, "Second step", steps[1].Action)

	// Semicolon-separated single line.
	steps = importparser.ParseStepsFromText("Step A; Step B")
	require.Len(t, steps, 2)
}

func TestParseMarkdownTable_Success(t *testing.T) {
	raw := `| Name | Description | Action | Expected Result |
|------|-------------|--------|-----------------|
| Login | User login | Enter creds | Redirected |
| Logout | User logout | Click logout | Session ends |`

	tcs, unparseable, err := importparser.ParseMarkdownTable(raw)
	require.NoError(t, err)
	require.Len(t, tcs, 2)
	assert.Empty(t, unparseable)
	assert.Equal(t, "Login", tcs[0].Name)
	assert.Equal(t, "User login", tcs[0].Description)
	require.Len(t, tcs[0].Steps, 1)
	assert.Equal(t, "Enter creds", tcs[0].Steps[0].Action)
	assert.Equal(t, "Redirected", tcs[0].Steps[0].ExpectedResult)
}

func TestParseMarkdownTable_MultiStepColumn(t *testing.T) {
	raw := `| Name | Action | Expected Result |
|------|--------|-----------------|
| MultiStep | 1. First 2. Second | 1. A 2. B |`

	tcs, _, err := importparser.ParseMarkdownTable(raw)
	require.NoError(t, err)
	require.Len(t, tcs, 1)
	require.Len(t, tcs[0].Steps, 2)
	assert.Equal(t, "First", tcs[0].Steps[0].Action)
	assert.Equal(t, "B", tcs[0].Steps[1].ExpectedResult)
}

func TestParseMarkdownTable_StepsColumn(t *testing.T) {
	raw := "| Name | Steps |\n|------|-------|\n| WithSteps | Step one; Step two |"
	tcs, _, err := importparser.ParseMarkdownTable(raw)
	require.NoError(t, err)
	require.Len(t, tcs, 1)
	require.Len(t, tcs[0].Steps, 2)
}

func TestParseMarkdownTable_EmptyName(t *testing.T) {
	raw := `| Name | Description |
|------|-------------|
|      | missing name |
| Good | ok |`
	tcs, unparseable, err := importparser.ParseMarkdownTable(raw)
	require.NoError(t, err)
	require.Len(t, tcs, 1)
	assert.Len(t, unparseable, 1)
}

func TestParseMarkdownTable_NoSeparator(t *testing.T) {
	_, _, err := importparser.ParseMarkdownTable("no table here")
	assert.Error(t, err)
}

func TestParseMarkdownTable_MissingNameColumn(t *testing.T) {
	raw := `| Foo | Bar |
|-----|-----|
| x   | y   |`
	_, _, err := importparser.ParseMarkdownTable(raw)
	assert.Error(t, err)
}

func TestParseCSV_Success(t *testing.T) {
	raw := "name,description,action,expected_result\nLogin,Check login,Enter creds,Redirected\nLogout,Check logout,Click logout,Session ends"
	tcs, _, err := importparser.ParseCSV(raw)
	require.NoError(t, err)
	require.Len(t, tcs, 2)
	assert.Equal(t, "Login", tcs[0].Name)
	assert.Equal(t, "Check login", tcs[0].Description)
	require.Len(t, tcs[0].Steps, 1)
}

func TestParseCSV_MultiRowContinuation(t *testing.T) {
	raw := "name,action,expected_result\nLogin,Step 1,Result 1\nLogin,Step 2,Result 2\nLogout,Click logout,Done"
	tcs, _, err := importparser.ParseCSV(raw)
	require.NoError(t, err)
	require.Len(t, tcs, 2)
	assert.Equal(t, "Login", tcs[0].Name)
	require.Len(t, tcs[0].Steps, 2)
}

func TestParseCSV_EmptyNameRow(t *testing.T) {
	raw := "name,description\nGood,ok\n,skipped"
	tcs, unparseable, err := importparser.ParseCSV(raw)
	require.NoError(t, err)
	require.Len(t, tcs, 1)
	assert.Len(t, unparseable, 1)
}

func TestParseCSV_MissingNameColumn(t *testing.T) {
	raw := "foo,bar\nx,y"
	_, _, err := importparser.ParseCSV(raw)
	assert.Error(t, err)
}

func TestParseCSV_TooShort(t *testing.T) {
	_, _, err := importparser.ParseCSV("name,description")
	assert.Error(t, err)
}

func TestParseNumberedList_Success(t *testing.T) {
	raw := `1. Login Test: Verify user can log in
  - Enter credentials
  - Click login
2. Logout Test: Verify user can log out
  - Click logout button`

	tcs, _, err := importparser.ParseNumberedList(raw)
	require.NoError(t, err)
	require.Len(t, tcs, 2)
	assert.Equal(t, "Login Test", tcs[0].Name)
	assert.Equal(t, "Verify user can log in", tcs[0].Description)
	require.Len(t, tcs[0].Steps, 2)
	assert.Equal(t, "Enter credentials", tcs[0].Steps[0].Action)
}

func TestParseNumberedList_DashSeparator(t *testing.T) {
	raw := "1. Login - Authenticate user"
	tcs, _, err := importparser.ParseNumberedList(raw)
	require.NoError(t, err)
	require.Len(t, tcs, 1)
	assert.Equal(t, "Login", tcs[0].Name)
	assert.Equal(t, "Authenticate user", tcs[0].Description)
}

func TestParseNumberedList_Bullets(t *testing.T) {
	raw := "- Test One\n- Test Two"
	tcs, _, err := importparser.ParseNumberedList(raw)
	require.NoError(t, err)
	require.Len(t, tcs, 2)
	assert.Equal(t, "Test One", tcs[0].Name)
}

func TestParseNumberedList_NoMatch(t *testing.T) {
	_, _, err := importparser.ParseNumberedList("just plain text\nno structure")
	assert.Error(t, err)
}

func TestParseJSON_Success(t *testing.T) {
	drafts := []models.GeneratedTestCase{{Name: "x"}}
	tcs, unparseable, err := importparser.ParseJSON("any", stubLLM(drafts, nil))
	require.NoError(t, err)
	assert.Equal(t, drafts, tcs)
	assert.Nil(t, unparseable)
}

func TestParseJSON_Error(t *testing.T) {
	_, _, err := importparser.ParseJSON("bad", stubLLM(nil, errors.New("boom")))
	assert.Error(t, err)
}

func TestParseImportContent_WithHint(t *testing.T) {
	raw := "| Name |\n|------|\n| foo |"
	tcs, _, format, err := importparser.ParseImportContent(raw, "markdown_table", nil)
	require.NoError(t, err)
	assert.Equal(t, "markdown_table", format)
	require.Len(t, tcs, 1)
}

func TestParseImportContent_AutoDetect(t *testing.T) {
	raw := "name,description\nfoo,bar"
	tcs, _, format, err := importparser.ParseImportContent(raw, "", nil)
	require.NoError(t, err)
	assert.Equal(t, "csv", format)
	require.Len(t, tcs, 1)
}

func TestParseImportContent_JSONViaStub(t *testing.T) {
	drafts := []models.GeneratedTestCase{{Name: "json-case"}}
	tcs, _, format, err := importparser.ParseImportContent(`[{"name":"x"}]`, "", stubLLM(drafts, nil))
	require.NoError(t, err)
	assert.Equal(t, "json", format)
	require.Len(t, tcs, 1)
	assert.Equal(t, "json-case", tcs[0].Name)
}

func TestParseImportContent_FallbackUnrecognized(t *testing.T) {
	// No format hint, no detectable structure, stub returns nothing.
	_, _, _, err := importparser.ParseImportContent("random gibberish text", "", stubLLM(nil, errors.New("no json")))
	assert.Error(t, err)
}

func TestParseImportContent_FallbackFindsTable(t *testing.T) {
	// Format unrecognized by detector, but table parser should succeed in fallback.
	raw := "preamble text\n| Name |\n|------|\n| foo |"
	tcs, _, format, err := importparser.ParseImportContent(raw, "", stubLLM(nil, errors.New("no json")))
	require.NoError(t, err)
	assert.Equal(t, "markdown_table", format)
	require.Len(t, tcs, 1)
}
