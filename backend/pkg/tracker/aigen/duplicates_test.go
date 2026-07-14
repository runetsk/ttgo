package aigen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

func namedDraft(name string) models.GeneratedTestCase {
	return models.GeneratedTestCase{Name: name, Steps: []models.GeneratedStep{{Action: "a", ExpectedResult: "e"}}}
}

func TestNormalizeTestText(t *testing.T) {
	assert.Equal(t, "sign in with valid credentials",
		NormalizeTestText("[Functional] Sign in, with VALID credentials!"))
	assert.Equal(t, "", NormalizeTestText("  [Negative]  "))
}

func TestTokenSimilarity(t *testing.T) {
	assert.Equal(t, 1.0, TokenSimilarity("Sign in works", "[Functional] sign in works"))
	assert.Equal(t, 0.0, TokenSimilarity("", "anything"))
	sim := TokenSimilarity("Sign in with valid credentials", "Sign in with invalid credentials")
	assert.Greater(t, sim, 0.5)
	assert.Less(t, sim, 1.0)
	assert.InDelta(t, TokenSimilarity("a b c", "c b a"), 1.0, 1e-9, "order-insensitive")
}

func TestFindBatchDuplicates(t *testing.T) {
	batch := []BatchDraft{
		{Position: 0, Draft: namedDraft("[Functional] Sign in with valid credentials")},
		{Position: 1, Draft: namedDraft("Sign in with valid credentials")}, // dup of 0
		{Position: 2, Draft: namedDraft("Export report as CSV")},
	}
	dupes := FindBatchDuplicates(batch)

	require.Contains(t, dupes, 0)
	require.Contains(t, dupes, 1)
	assert.NotContains(t, dupes, 2)

	c := dupes[0][0]
	assert.Equal(t, DupKindBatch, c.Kind)
	require.NotNil(t, c.DraftPosition)
	assert.Equal(t, 1, *c.DraftPosition)
	assert.Equal(t, 1.0, c.Similarity)
	assert.NotEmpty(t, c.Reason)
	assert.Empty(t, c.TestCaseID)
}

func TestFindBatchDuplicates_ThresholdAndCap(t *testing.T) {
	batch := []BatchDraft{
		{Position: 0, Draft: namedDraft("Login page loads")},
		{Position: 1, Draft: namedDraft("Totally unrelated CSV export")},
	}
	assert.Empty(t, FindBatchDuplicates(batch))

	var big []BatchDraft
	for i := 0; i < MaxDuplicateCandidates+3; i++ {
		big = append(big, BatchDraft{Position: i, Draft: namedDraft("Sign in with valid credentials")})
	}
	dupes := FindBatchDuplicates(big)
	assert.Len(t, dupes[0], MaxDuplicateCandidates, "per-draft candidate list is capped")
}
