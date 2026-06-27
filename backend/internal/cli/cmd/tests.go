package cmd

import (
	"encoding/json"
	"fmt"
	"ttgo/internal/cli/output"

	"github.com/spf13/cobra"
)

func newTestsCmd() *cobra.Command {
	testsCmd := &cobra.Command{
		Use:   "tests",
		Short: "Manage test cases",
	}

	testsCmd.AddCommand(
		newTestsListCmd(),
		newTestsGetCmd(),
		newTestsCreateCmd(),
		newTestsUpdateCmd(),
		newTestsDeleteCmd(),
		newTestsVersionsCmd(),
		newTestsRestoreCmd(),
		newTestsExecutionsCmd(),
	)

	return testsCmd
}

func newTestsListCmd() *cobra.Command {
	var folderID, categoryID string
	var summary bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List test cases",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.ListTests(folderID, categoryID, summary)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			var tests []map[string]interface{}
			if err := json.Unmarshal(raw, &tests); err != nil {
				return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
			}
			return output.Print(cmd.OutOrStdout(), outputMode(), tests, []output.Column{
				{Header: "ID", Key: "id"},
				{Header: "NAME", Key: "name"},
				{Header: "FOLDER", Key: "folder_id"},
			})
		},
	}
	cmd.Flags().StringVar(&folderID, "folder", "", "Filter by folder ID")
	cmd.Flags().StringVar(&categoryID, "category", "", "Filter by category ID")
	cmd.Flags().BoolVar(&summary, "summary", false, "Fetch slim payload (omits full steps and custom values, includes steps_count)")
	return cmd
}

func newTestsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a test case by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.GetTest(args[0])
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}

func newTestsCreateCmd() *cobra.Command {
	var name, folderID, description string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new test case",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			result, err := c.CreateTest(name, folderID, description)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.Print(cmd.OutOrStdout(), "json", result, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created test %q (ID: %s)\n", result["name"], result["id"])
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Test name (required)")
	cmd.Flags().StringVar(&folderID, "folder", "", "Folder ID (required)")
	cmd.Flags().StringVar(&description, "description", "", "Test description (HTML)")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("folder")
	return cmd
}

func newTestsUpdateCmd() *cobra.Command {
	var name, folderID, description string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a test case",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			fields := map[string]interface{}{}
			if name != "" {
				fields["name"] = name
			}
			if folderID != "" {
				fields["folder_id"] = folderID
			}
			if description != "" {
				fields["description"] = description
			}
			raw, err := c.UpdateTest(args[0], fields)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Test updated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New test name")
	cmd.Flags().StringVar(&folderID, "folder", "", "New folder ID")
	cmd.Flags().StringVar(&description, "description", "", "New description")
	return cmd
}

func newTestsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a test case",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.DeleteTest(args[0]); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Test deleted.")
			return nil
		},
	}
}

func newTestsVersionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "versions <id>",
		Short: "List version history for a test case",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.ListTestVersions(args[0])
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}

func newTestsRestoreCmd() *cobra.Command {
	var versionID string
	cmd := &cobra.Command{
		Use:   "restore <id>",
		Short: "Restore a test case to a specific version",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.RestoreTestVersion(args[0], versionID); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Test restored to version "+versionID+".")
			return nil
		},
	}
	cmd.Flags().StringVar(&versionID, "version", "", "Version ID to restore (required)")
	cmd.MarkFlagRequired("version")
	return cmd
}

func newTestsExecutionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "executions <id>",
		Short: "List execution history for a test case",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.ListTestExecutions(args[0])
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}
