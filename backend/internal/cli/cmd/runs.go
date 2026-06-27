package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"ttgo/internal/cli/output"

	"github.com/spf13/cobra"
)

func newRunsCmd() *cobra.Command {
	runsCmd := &cobra.Command{
		Use:   "runs",
		Short: "Manage test runs",
	}

	resultsCmd := &cobra.Command{
		Use:   "results",
		Short: "Manage run results",
	}
	resultsCmd.AddCommand(
		newRunsResultsAddCmd(),
		newRunsResultsUpdateCmd(),
		newRunsResultsRetryCmd(),
		newRunsResultsBulkUpdateCmd(),
		newRunsResultsDeleteCmd(),
	)

	runsCmd.AddCommand(
		newRunsListCmd(),
		newRunsGetCmd(),
		newRunsCreateCmd(),
		newRunsCompleteCmd(),
		newRunsReopenCmd(),
		newRunsCopyCmd(),
		newRunsDeleteCmd(),
		resultsCmd,
	)

	return runsCmd
}

func newRunsListCmd() *cobra.Command {
	var categoryID, status, folderID string
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List test runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			if categoryID != "" {
				params["category_id"] = categoryID
			}
			if status != "" {
				params["status"] = status
			}
			if folderID != "" {
				params["run_folder_id"] = folderID
			}
			if limit > 0 {
				params["limit"] = fmt.Sprintf("%d", limit)
			}
			if offset > 0 {
				params["offset"] = fmt.Sprintf("%d", offset)
			}
			raw, err := c.ListRuns(params)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			var resp struct {
				Runs  []map[string]interface{} `json:"runs"`
				Total int                      `json:"total"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
			}
			return output.Print(cmd.OutOrStdout(), outputMode(), resp.Runs, []output.Column{
				{Header: "ID", Key: "id"},
				{Header: "NAME", Key: "name"},
				{Header: "STATUS", Key: "status"},
				{Header: "PASSED", Key: "passed_results"},
				{Header: "FAILED", Key: "failed_results"},
				{Header: "TOTAL", Key: "total_results"},
			})
		},
	}
	cmd.Flags().StringVar(&categoryID, "category", "", "Filter by category ID")
	cmd.Flags().StringVar(&status, "status", "", "Filter by status")
	cmd.Flags().StringVar(&folderID, "folder", "", "Filter by run folder ID")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max results per page")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")
	return cmd
}

func newRunsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a test run by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.GetRun(args[0])
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}

func newRunsCreateCmd() *cobra.Command {
	var name, categoryID, folderID string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new test run",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			result, err := c.CreateRun(name, categoryID, folderID)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.Print(cmd.OutOrStdout(), "json", result, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created run %q (ID: %s)\n", result["name"], result["id"])
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Run name (required)")
	cmd.Flags().StringVar(&categoryID, "category", "", "Category ID")
	cmd.Flags().StringVar(&folderID, "folder", "", "Run folder ID")
	cmd.MarkFlagRequired("name")
	return cmd
}

func newRunsCompleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "complete <id>",
		Short: "Complete a test run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.CompleteRun(args[0])
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Run completed.")
			return nil
		},
	}
}

func newRunsReopenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reopen <id>",
		Short: "Reopen a completed test run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.ReopenRun(args[0])
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Run reopened.")
			return nil
		},
	}
}

func newRunsCopyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "copy <id>",
		Short: "Copy a test run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			result, err := c.CopyRun(args[0])
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.Print(cmd.OutOrStdout(), "json", result, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Copied run (new ID: %s)\n", result["id"])
			return nil
		},
	}
}

func newRunsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a test run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.DeleteRun(args[0]); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Run deleted.")
			return nil
		},
	}
}

func newRunsResultsAddCmd() *cobra.Command {
	var testID, status, errorMsg, defectType string
	cmd := &cobra.Command{
		Use:   "add <run-id>",
		Short: "Add a result to a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			extra := map[string]interface{}{}
			if errorMsg != "" {
				extra["error_message"] = errorMsg
			}
			if defectType != "" {
				extra["defect_type"] = defectType
			}
			result, err := c.AddRunResult(args[0], testID, status, extra)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.Print(cmd.OutOrStdout(), "json", result, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added result (ID: %s)\n", result["id"])
			return nil
		},
	}
	cmd.Flags().StringVar(&testID, "test", "", "Test case ID (required)")
	cmd.Flags().StringVar(&status, "status", "", "Status: PASS, FAIL, SKIP (required)")
	cmd.Flags().StringVar(&errorMsg, "error", "", "Error message")
	cmd.Flags().StringVar(&defectType, "defect-type", "", "Defect type")
	cmd.MarkFlagRequired("test")
	cmd.MarkFlagRequired("status")
	return cmd
}

func newRunsResultsUpdateCmd() *cobra.Command {
	var status, errorMsg, defectType string
	cmd := &cobra.Command{
		Use:   "update <run-id> <result-id>",
		Short: "Update a run result",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			fields := map[string]interface{}{}
			if status != "" {
				fields["status"] = status
			}
			if errorMsg != "" {
				fields["error_message"] = errorMsg
			}
			if defectType != "" {
				fields["defect_type"] = defectType
			}
			raw, err := c.UpdateRunResult(args[0], args[1], fields)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Result updated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "New status")
	cmd.Flags().StringVar(&errorMsg, "error", "", "Error message")
	cmd.Flags().StringVar(&defectType, "defect-type", "", "Defect type")
	return cmd
}

func newRunsResultsRetryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "retry <run-id> <result-id>",
		Short: "Retry a run result (increment attempt)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.RetryRunResult(args[0], args[1])
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Result retried.")
			return nil
		},
	}
}

func newRunsResultsBulkUpdateCmd() *cobra.Command {
	var ids, status, defectType string
	cmd := &cobra.Command{
		Use:   "bulk-update <run-id>",
		Short: "Bulk update run results",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			idList := strings.Split(ids, ",")
			raw, err := c.BulkUpdateRunResults(args[0], idList, status, defectType)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Results updated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&ids, "ids", "", "Comma-separated result IDs (required)")
	cmd.Flags().StringVar(&status, "status", "", "New status (required)")
	cmd.Flags().StringVar(&defectType, "defect-type", "", "Defect type")
	cmd.MarkFlagRequired("ids")
	cmd.MarkFlagRequired("status")
	return cmd
}

func newRunsResultsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <run-id> <result-id>",
		Short: "Delete a run result",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.DeleteRunResult(args[0], args[1]); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Result deleted.")
			return nil
		},
	}
}
