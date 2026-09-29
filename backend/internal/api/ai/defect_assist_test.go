package ai

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ttgo/pkg/tracker/failureanalysis"
	"ttgo/pkg/tracker/models"
	"ttgo/pkg/tracker/typesafe"

	"github.com/stretchr/testify/require"
)

func TestSelectDefectCandidates(t *testing.T) {
	same := []models.Defect{{ID: "d1", Title: "Anything", Status: "open"}, {ID: "d2", Title: "Old", Status: "closed"}}
	all := []models.Defect{
		{ID: "d1", Title: "Anything", Status: "open"},
		{ID: "d3", Title: "Checkout total wrong after discount", Status: "open"},
		{ID: "d4", Title: "Checkout total wrong", Status: "fixed"},
		{ID: "d5", Title: "Profile page crashes", Status: "open"},
		{ID: "d6", Title: "Checkout total wrong after discount code", Status: "closed"},
	}
	got := selectDefectCandidates("Checkout total is wrong after a discount", same, all)
	ids := []string{}
	for _, d := range got {
		ids = append(ids, d.ID)
	}
	require.Equal(t, []string{"d1", "d3", "d4"}, ids, "the test's own open defects first, then title matches; never closed ones")
}

func postAssist(e *useEnv, req DefectAssistRequest) *httptest.ResponseRecorder {
	b, _ := json.Marshal(req)
	rec := httptest.NewRecorder()
	e.h.AssistDefect(rec, httptest.NewRequest("POST", "/api/defects/assist", bytes.NewReader(b)))
	return rec
}

func TestAssistDefect_SuggestsSeverityAndFindsDuplicates(t *testing.T) {
	e := newUseEnv(t, true, answerAll(
		func(id string, _ typesafe.Question) string { return failureanalysis.SeverityMajor },
		func(id string) float64 {
			if id == "dup_0" {
				return 0.91
			}
			return 0.2
		},
	))
	dup := &models.Defect{Title: "Checkout total wrong after discount", Status: "open", ExternalKey: "PAY-42"}
	require.NoError(t, e.s.CreateDefect(dup))
	require.NoError(t, e.s.CreateDefect(&models.Defect{Title: "Checkout total wrong on mobile", Status: "open"}))

	rec := postAssist(e, DefectAssistRequest{Title: "Checkout total is wrong after a discount", Description: "1499.00 instead of 1499.99"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var got DefectAssistResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.Equal(t, "major", got.Severity.Value)
	require.InDelta(t, 0.9, got.Severity.Confidence, 1e-12)
	require.Len(t, got.Duplicates, 1, "only answers at 70% or more are reported")
	require.Equal(t, dup.ID, got.Duplicates[0].DefectID)
	require.Equal(t, "PAY-42", got.Duplicates[0].ExternalKey)

	req := e.client.requests()[0]
	require.Contains(t, req.Questions, "severity")
	require.Contains(t, req.Questions, "dup_1")
	var cost []models.AIAnalysisCostEvent
	require.NoError(t, e.s.DB().Where("kind = ?", models.AnalysisCostKindDefectAssist).Find(&cost).Error)
	require.Len(t, cost, 1)
}

func TestAssistDefect_RefusalsAndNoCandidates(t *testing.T) {
	off := newUseEnv(t, false, nil)
	rec := postAssist(off, DefectAssistRequest{Title: "x"})
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Contains(t, rec.Body.String(), "switched off")

	on := newUseEnv(t, true, answerAll(func(string, typesafe.Question) string { return "minor" }, nil))
	require.Equal(t, http.StatusBadRequest, postAssist(on, DefectAssistRequest{Title: "  "}).Code)
	rec = postAssist(on, DefectAssistRequest{Title: "Profile avatar is blurry"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.True(t, strings.Contains(rec.Body.String(), `"duplicates":[]`), rec.Body.String())
	require.Len(t, on.client.requests()[0].Questions, 1, "no open defect to compare: severity only")
}
