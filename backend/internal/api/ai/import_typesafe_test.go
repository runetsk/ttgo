package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

const proseImport = `Login works for a returning customer
Open the sign-in page
Enter a valid email and password
The dashboard should appear
Contact the QA team with questions`

// roleByText classifies the prose lines the way a reader would.
func roleByText(req typesafe.Request) map[string]string {
	lines := req.State.(map[string]any)["lines"].([]map[string]any)
	out := map[string]string{}
	for id := range req.Questions {
		var i int
		_, _ = fmtSscanf(id, &i)
		text := lines[i]["text"].(string)
		switch {
		case strings.HasPrefix(text, "Login"):
			out[id] = failureanalysis.ImportRoleTitle
		case strings.HasPrefix(text, "Open"), strings.HasPrefix(text, "Enter"):
			out[id] = failureanalysis.ImportRoleStep
		case strings.Contains(text, "should"):
			out[id] = failureanalysis.ImportRoleExpected
		default:
			out[id] = failureanalysis.ImportRoleNoise
		}
	}
	return out
}

func postImport(e *useEnv, content string) *httptest.ResponseRecorder {
	body, _ := json.Marshal(models.ParseImportRequest{Content: content})
	rec := httptest.NewRecorder()
	e.h.ParseImport(rec, httptest.NewRequest("POST", "/api/import/parse", bytes.NewReader(body)))
	return rec
}

func TestParseImport_TypeSafeRecoversTheStructureBeforeTheLLM(t *testing.T) {
	e := newUseEnv(t, true, func(req typesafe.Request) (*typesafe.Response, error) {
		roles := roleByText(req)
		return answerAll(func(id string, _ typesafe.Question) string { return roles[id] }, nil)(req)
	})
	rec := postImport(e, proseImport)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got models.ParseImportResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "typesafe", got.DetectedFormat)
	require.Len(t, got.TestCases, 1)
	tc := got.TestCases[0]
	require.Equal(t, "Login works for a returning customer", tc.Name)
	require.Equal(t, []models.GeneratedStep{
		{Action: "Open the sign-in page"},
		{Action: "Enter a valid email and password", ExpectedResult: "The dashboard should appear"},
	}, tc.Steps)
	require.NotEmpty(t, tc.TempID)

	reqs := e.client.requests()
	require.Len(t, reqs, 1)
	require.Len(t, reqs[0].Questions, 5, "one question per line")
	var rows []models.AIAnalysisCostEvent
	require.NoError(t, e.s.DB().Where("kind = ?", models.AnalysisCostKindImport).Find(&rows).Error)
	require.Len(t, rows, 1)
}

func TestParseImport_TypeSafeFailureFallsBackToTheLLMPath(t *testing.T) {
	e := newUseEnv(t, true, func(typesafe.Request) (*typesafe.Response, error) { return nil, errors.New("down") })
	rec := postImport(e, proseImport)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, "no LLM provider configured: the old 422")
	require.Len(t, e.client.requests(), 1)
}

func TestParseImport_DeterministicFormatsNeverAskTypeSafe(t *testing.T) {
	e := newUseEnv(t, true, func(typesafe.Request) (*typesafe.Response, error) {
		t.Fatal("no TypeSafe call expected")
		return nil, nil
	})
	rec := postImport(e, "1. Open the page\n2. Click Save\n3. The item is saved")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// fmtSscanf reads the line index out of a "line_<i>" question id.
func fmtSscanf(id string, i *int) (int, error) { return fmt.Sscanf(id, "line_%d", i) }
