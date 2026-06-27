package cmd

import (
	"ttgo/internal/cli/output"

	"github.com/spf13/cobra"
)

func newDefectsCmd() *cobra.Command {
	defectsCmd := &cobra.Command{
		Use:   "defects",
		Short: "Manage Jira defect links",
	}

	defectsCmd.AddCommand(
		newDefectsListCmd(),
		newDefectsLinkCmd(),
		newDefectsUnlinkCmd(),
		newDefectsCreateIssueCmd(),
	)

	return defectsCmd
}

func newDefectsListCmd() *cobra.Command {
	var testID, runID, resultID string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List defect links",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			var raw []byte
			var fetchErr error
			switch {
			case testID != "":
				raw, fetchErr = c.ListTestDefects(testID)
			case resultID != "" && runID != "":
				raw, fetchErr = c.ListResultDefects(runID, resultID)
			case runID != "":
				raw, fetchErr = c.ListRunDefects(runID)
			default:
				return cmd.Help()
			}
			if fetchErr != nil {
				return fetchErr
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
	cmd.Flags().StringVar(&testID, "test", "", "Test case ID")
	cmd.Flags().StringVar(&runID, "run", "", "Run ID")
	cmd.Flags().StringVar(&resultID, "result", "", "Result ID (requires --run)")
	return cmd
}

func newDefectsLinkCmd() *cobra.Command {
	var runID, resultID, jiraKey string
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Link a Jira issue to a run result",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.LinkDefect(runID, resultID, jiraKey); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Defect linked.")
			return nil
		},
	}
	cmd.Flags().StringVar(&runID, "run", "", "Run ID (required)")
	cmd.Flags().StringVar(&resultID, "result", "", "Result ID (required)")
	cmd.Flags().StringVar(&jiraKey, "jira-key", "", "Jira issue key (required)")
	cmd.MarkFlagRequired("run")
	cmd.MarkFlagRequired("result")
	cmd.MarkFlagRequired("jira-key")
	return cmd
}

func newDefectsUnlinkCmd() *cobra.Command {
	var runID, resultID, jiraKey string
	cmd := &cobra.Command{
		Use:   "unlink",
		Short: "Unlink a Jira issue from a run result",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.UnlinkDefect(runID, resultID, jiraKey); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Defect unlinked.")
			return nil
		},
	}
	cmd.Flags().StringVar(&runID, "run", "", "Run ID (required)")
	cmd.Flags().StringVar(&resultID, "result", "", "Result ID (required)")
	cmd.Flags().StringVar(&jiraKey, "jira-key", "", "Jira issue key (required)")
	cmd.MarkFlagRequired("run")
	cmd.MarkFlagRequired("result")
	cmd.MarkFlagRequired("jira-key")
	return cmd
}

func newDefectsCreateIssueCmd() *cobra.Command {
	var testID string
	cmd := &cobra.Command{
		Use:   "create-issue",
		Short: "Create a new Jira issue from a test case",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.CreateDefectIssue(testID)
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
	cmd.Flags().StringVar(&testID, "test", "", "Test case ID (required)")
	cmd.MarkFlagRequired("test")
	return cmd
}
