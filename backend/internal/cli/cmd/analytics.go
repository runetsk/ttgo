package cmd

import (
	"fmt"
	"ttgo/internal/cli/output"

	"github.com/spf13/cobra"
)

func newAnalyticsCmd() *cobra.Command {
	analyticsCmd := &cobra.Command{
		Use:   "analytics",
		Short: "Query analytics and reports",
	}

	subcommands := []struct {
		use, short, path string
		extraFlags       func(cmd *cobra.Command)
	}{
		{"summary", "Overall pass/fail/skip counts", "summary", nil},
		{"trend", "Pass rate over time", "trend", func(cmd *cobra.Command) {
			cmd.Flags().Int("days", 30, "Number of days")
		}},
		{"flaky", "List flaky tests", "flaky", func(cmd *cobra.Command) {
			cmd.Flags().Int("lookback", 30, "Number of recent runs per test")
			cmd.Flags().Int("limit", 20, "Max results")
		}},
		{"most-failed", "Most failed tests", "most-failed", func(cmd *cobra.Command) {
			cmd.Flags().Int("limit", 20, "Max results")
		}},
		{"duration", "Duration trends", "duration", nil},
		{"duration-top", "Slowest test cases", "duration/top", func(cmd *cobra.Command) {
			cmd.Flags().Int("limit", 20, "Max results")
		}},
		{"component-health", "Passing rate by folder", "component-health", func(cmd *cobra.Command) {
			cmd.Flags().Float64("threshold", 80.0, "Health threshold percentage")
		}},
		{"growth", "Test case growth over time", "growth", nil},
		{"passing-rate", "Pass rate per folder", "passing-rate", func(cmd *cobra.Command) {
			cmd.Flags().Bool("exclude-skipped", false, "Exclude skipped tests")
		}},
		{"unique-bugs", "Count of distinct Jira issues", "unique-bugs", func(cmd *cobra.Command) {
			cmd.Flags().Int("limit", 50, "Max results")
		}},
		{"activity", "Test run activity over time", "activity", func(cmd *cobra.Command) {
			cmd.Flags().Int("limit", 30, "Max results")
		}},
	}

	for _, sc := range subcommands {
		sub := sc
		cmd := &cobra.Command{
			Use:   sub.use,
			Short: sub.short,
			RunE:  analyticsRunE(sub.path),
		}
		cmd.Flags().String("start-date", "", "Start date (YYYY-MM-DD)")
		cmd.Flags().String("end-date", "", "End date (YYYY-MM-DD)")
		cmd.Flags().String("folder", "", "Folder ID filter")
		if sub.extraFlags != nil {
			sub.extraFlags(cmd)
		}
		analyticsCmd.AddCommand(cmd)
	}

	analyticsCmd.AddCommand(newAnalyticsCompareRunsCmd())

	return analyticsCmd
}

func analyticsRunE(path string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		c, err := newClient()
		if err != nil {
			return err
		}
		params := map[string]string{}
		if v, _ := cmd.Flags().GetString("start-date"); v != "" {
			params["start_date"] = v
		}
		if v, _ := cmd.Flags().GetString("end-date"); v != "" {
			params["end_date"] = v
		}
		if v, _ := cmd.Flags().GetString("folder"); v != "" {
			params["folder_id"] = v
		}
		if v, _ := cmd.Flags().GetInt("days"); v > 0 {
			params["days"] = fmt.Sprintf("%d", v)
		}
		if v, _ := cmd.Flags().GetInt("lookback"); v > 0 {
			params["lookback"] = fmt.Sprintf("%d", v)
		}
		if v, _ := cmd.Flags().GetInt("limit"); v > 0 {
			params["limit"] = fmt.Sprintf("%d", v)
		}
		if v, _ := cmd.Flags().GetFloat64("threshold"); v > 0 {
			params["threshold"] = fmt.Sprintf("%.1f", v)
		}
		if v, _ := cmd.Flags().GetBool("exclude-skipped"); v {
			params["exclude_skipped"] = "true"
		}
		raw, err := c.AnalyticsGet(path, params)
		if err != nil {
			return err
		}
		return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
	}
}

func newAnalyticsCompareRunsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "compare-runs <run1-id> <run2-id>",
		Short: "Compare two test runs side-by-side",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			params := map[string]string{"run1": args[0], "run2": args[1]}
			raw, err := c.AnalyticsGet("compare-runs", params)
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}
