package aigen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"ttgo/pkg/tracker/models"
)

type benchmarkCase struct {
	Name            string                     `json:"name"`
	DescriptionHTML string                     `json:"description_html"`
	Children        []*models.Requirement      `json:"children"`
	Drafts          []models.GeneratedTestCase `json:"drafts"`
	Expect          struct {
		TargetIDs               []string         `json:"target_ids"`
		UncoveredIDs            []string         `json:"uncovered_ids"`
		DraftFindingCodes       [][]string       `json:"draft_finding_codes"`
		BatchDuplicatePositions map[string][]int `json:"batch_duplicate_positions"`
	} `json:"expect"`
}

// TestBenchmarkV1 runs the versioned deterministic evaluation set in CI
// (spec: "Feedback and evaluation" — deterministic evaluation runs in CI).
func TestBenchmarkV1(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "benchmarks", "v1", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "benchmark fixtures missing")

	for _, file := range files {
		raw, err := os.ReadFile(file)
		require.NoError(t, err, file)
		var bc benchmarkCase
		require.NoError(t, json.Unmarshal(raw, &bc), file)

		t.Run(bc.Name, func(t *testing.T) {
			targets := ExtractCoverageTargets(bc.DescriptionHTML, bc.Children)
			var ids []string
			for _, tg := range targets {
				ids = append(ids, tg.ID)
			}
			assert.Equal(t, bc.Expect.TargetIDs, ids, "coverage targets")

			rep := BuildCoverageReport(targets, bc.Drafts)
			var uncovered []string
			for _, tc := range rep.Targets {
				if tc.Status == TargetStatusUncovered {
					uncovered = append(uncovered, tc.ID)
				}
			}
			assert.ElementsMatch(t, bc.Expect.UncoveredIDs, uncovered, "uncovered targets")

			nameCounts := map[string]int{}
			batch := make([]BatchDraft, len(bc.Drafts))
			for i, d := range bc.Drafts {
				nameCounts[NormalizeTestText(d.Name)]++
				batch[i] = BatchDraft{Position: i, Draft: d}
			}
			require.Len(t, bc.Expect.DraftFindingCodes, len(bc.Drafts), "fixture shape")
			for i, d := range bc.Drafts {
				var codes []string
				for _, dim := range EvaluateDraftQuality(d, nameCounts, targets) {
					for _, f := range dim.Findings {
						codes = append(codes, dim.Key+":"+f.Code)
					}
				}
				assert.ElementsMatch(t, bc.Expect.DraftFindingCodes[i], codes,
					fmt.Sprintf("draft %d finding codes", i))
			}

			dupes := FindBatchDuplicates(batch)
			got := map[string][]int{}
			for pos, cands := range dupes {
				for _, c := range cands {
					if c.Kind == DupKindBatch && c.DraftPosition != nil {
						got[strconv.Itoa(pos)] = append(got[strconv.Itoa(pos)], *c.DraftPosition)
					}
				}
			}
			if len(bc.Expect.BatchDuplicatePositions) == 0 {
				assert.Empty(t, got, "batch duplicates")
			} else {
				assert.Equal(t, bc.Expect.BatchDuplicatePositions, got, "batch duplicates")
			}
		})
	}
}
