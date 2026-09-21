package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"ttgo/internal/cli/client"
	"ttgo/internal/cli/compare"
)

func newAICompareCmd() *cobra.Command {
	var answerKeyPath string
	cmd := &cobra.Command{
		Use:   "compare <run-id>",
		Short: "Compare AI failure-analysis engines on a run",
		Long: `Pivots every stored analysis of the run's failing results by engine and model
(TypeSafe vs generative LLMs), taking the latest version per column, flags the
rows where the engines disagree, and grades each column against the AI demo
dataset's answer key when the run's failures match its planted templates.

Re-analyzing a run with another engine or model adds a column; nothing is
overwritten. With -o json the full pivot and summary are printed for scripting.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.GetRun(args[0])
			if err != nil {
				return err
			}
			name, results, err := compare.ParseRun(raw)
			if err != nil {
				return err
			}
			histories := map[string][]compare.Analysis{}
			for _, r := range results {
				raw, err := c.ListRunResultAnalyses(r.ID)
				if err != nil {
					return fmt.Errorf("analyses of result %s: %w", r.ID, err)
				}
				list, err := compare.ParseAnalyses(raw)
				if err != nil {
					return err
				}
				histories[r.ID] = list
			}
			rep := compare.Pivot(results, histories)
			rep.RunID, rep.RunName = args[0], name

			gt, note := loadAnswerKey(c, answerKeyPath)
			if len(gt) > 0 {
				compare.Grade(&rep, gt)
			}
			sum := compare.Summarize(rep)
			if n := compare.GradingNote(sum, len(gt) > 0); n != "" {
				note = n
			}
			if outputMode() == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				enc.SetEscapeHTML(false)
				return enc.Encode(map[string]any{"report": rep, "summary": sum, "note": note})
			}
			compare.Render(cmd.OutOrStdout(), rep, sum)
			if note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&answerKeyPath, "answer-key", "",
		"JSON answer key to grade against (a saved GET /api/seed/ai response or a perfseed manifest) instead of fetching it from the server")
	return cmd
}

// loadAnswerKey returns the planted-template answer key from the given file
// or, by default, from GET /api/seed/ai. The note says why grading is skipped.
func loadAnswerKey(c *client.Client, path string) ([]compare.GroundTruth, string) {
	var raw []byte
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, "Grading skipped: " + err.Error()
		}
		raw = b
	} else {
		b, err := c.GetAISeedStatus()
		if err != nil {
			return nil, fmt.Sprintf("Grading skipped: the answer key (GET /api/seed/ai, admin only) could not be fetched: %v. Pass --answer-key <file> to grade with a saved copy.", err)
		}
		raw = b
	}
	gt, err := compare.ParseGroundTruth(raw)
	if err != nil {
		return nil, "Grading skipped: " + err.Error()
	}
	return gt, ""
}
