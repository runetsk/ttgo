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
	var answerKeyPath, jobID string
	var byJob bool
	var defectMode string
	var contract bool
	cmd := &cobra.Command{
		Use:   "compare <run-id>",
		Short: "Compare AI failure-analysis engines on a run",
		Long: `Pivots every stored analysis of the run's failing results by engine and model
(TypeSafe vs generative LLMs), taking the latest version per column, flags the
rows where the columns disagree, and grades each column against the AI demo
dataset's answer key when the run's failures match its planted templates.

Re-analyzing a run with another engine or model adds a column; nothing is
overwritten. Failed attempts show as FAILED cells and are counted, not graded.

--job <id> limits the report to what one analysis job stored (an id prefix is
enough). --by-job makes one column per job instead, labelled by the pipeline the
job ran (decider, narrator, explanations, takeover threshold, fallback), so two
passes with the same model but different settings stay apart. Analyses made
outside a job (a single re-analyze) are left out of both.

--defect-type-mode mapping re-grades every engine's defect type as the mapping of
its verdict (the rule the LLM path uses), so TypeSafe and the LLMs compare like for
like with no new calls; native (the default) grades the stored suggestions.
--explanation-contract checks each column's explanations: one summary sentence,
exactly one next action, at most two cited evidence items, and length percentiles.
Grading is also split into rows analyzed with and without the test's history.

With -o json the full pivot and summary are printed for scripting.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if jobID != "" && byJob {
				return fmt.Errorf("--job and --by-job cannot be combined")
			}
			mode, err := compare.ParseDefectTypeMode(defectMode)
			if err != nil {
				return err
			}
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
			var opts compare.Options
			if jobID != "" || byJob {
				raw, err := c.ListAnalysisJobs(args[0])
				if err != nil {
					return fmt.Errorf("analysis jobs of run %s: %w", args[0], err)
				}
				jobs, err := compare.ParseJobs(raw)
				if err != nil {
					return err
				}
				opts = compare.Options{ByJob: byJob, Jobs: jobs}
				if jobID != "" {
					j, err := compare.ResolveJob(jobs, jobID)
					if err != nil {
						return err
					}
					opts.Job = j.ID
				}
			}
			opts.DefectTypeMode = mode
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
			rep := compare.PivotWith(results, histories, opts)
			rep.RunID, rep.RunName = args[0], name

			gt, note := loadAnswerKey(c, answerKeyPath)
			if len(gt) > 0 {
				compare.Grade(&rep, gt)
			}
			sum := compare.Summarize(rep)
			if n := compare.GradingNote(sum, len(gt) > 0); n != "" {
				note = n
			}
			var checks []compare.ContractStats
			if contract {
				checks = compare.CheckExplanations(rep)
			}
			if outputMode() == "json" {
				payload := map[string]any{"report": rep, "summary": sum, "note": note}
				if contract {
					payload["contract"] = checks
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				enc.SetEscapeHTML(false)
				return enc.Encode(payload)
			}
			compare.Render(cmd.OutOrStdout(), rep, sum)
			if contract {
				fmt.Fprintln(cmd.OutOrStdout())
				compare.RenderContract(cmd.OutOrStdout(), checks)
			}
			if note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&answerKeyPath, "answer-key", "",
		"JSON answer key to grade against (a saved GET /api/seed/ai response or a perfseed manifest) instead of fetching it from the server")
	cmd.Flags().StringVar(&jobID, "job", "", "only the analyses stored by this analysis job (id or unique prefix)")
	cmd.Flags().BoolVar(&byJob, "by-job", false, "one column per analysis job, labelled by its pipeline, instead of per engine/model")
	cmd.Flags().StringVar(&defectMode, "defect-type-mode", compare.DefectTypeModeNative,
		"native: grade each engine's stored defect-type suggestion; mapping: re-grade every engine's defect type as its verdict's mapping (like for like, no new calls)")
	cmd.Flags().BoolVar(&contract, "explanation-contract", false,
		"also check each column's explanations: one summary sentence, one next action, at most two cited evidence items, plus length percentiles")
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
