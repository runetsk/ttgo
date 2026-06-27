package cmd

import (
	"encoding/json"
	"fmt"
	"ttgo/internal/cli/client"
	"ttgo/internal/cli/output"

	"github.com/spf13/cobra"
)

func newRequirementsCmd() *cobra.Command {
	reqCmd := &cobra.Command{
		Use:     "requirements",
		Aliases: []string{"reqs"},
		Short:   "Manage requirements",
	}

	reqCmd.AddCommand(
		newReqListCmd(),
		newReqGetCmd(),
		newReqCreateCmd(),
		newReqUpdateCmd(),
		newReqDeleteCmd(),
		newReqLinkCmd(),
		newReqUnlinkCmd(),
		newReqImportCmd(),
		newReqBulkImportCmd(),
		newReqResyncCmd(),
		newReqPostToJiraCmd(),
	)

	return reqCmd
}

func newReqListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all requirements",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.ListRequirements()
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			var reqs []map[string]interface{}
			if err := json.Unmarshal(raw, &reqs); err != nil {
				return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
			}
			return output.Print(cmd.OutOrStdout(), outputMode(), reqs, []output.Column{
				{Header: "ID", Key: "id"},
				{Header: "IDENTIFIER", Key: "identifier"},
				{Header: "TITLE", Key: "title"},
				{Header: "SOURCE", Key: "source_type"},
			})
		},
	}
}

func newReqGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a requirement by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.GetRequirement(args[0])
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}

func newReqCreateCmd() *cobra.Command {
	var identifier, title, description string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new requirement",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			result, err := c.CreateRequirement(identifier, title, description)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.Print(cmd.OutOrStdout(), "json", result, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created requirement %q (ID: %s)\n", result["title"], result["id"])
			return nil
		},
	}
	cmd.Flags().StringVar(&identifier, "identifier", "", "Unique identifier (required)")
	cmd.Flags().StringVar(&title, "title", "", "Title (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	cmd.MarkFlagRequired("identifier")
	cmd.MarkFlagRequired("title")
	return cmd
}

func newReqUpdateCmd() *cobra.Command {
	var identifier, title, description string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a requirement",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			fields := map[string]string{}
			if identifier != "" {
				fields["identifier"] = identifier
			}
			if title != "" {
				fields["title"] = title
			}
			if description != "" {
				fields["description"] = description
			}
			raw, err := c.UpdateRequirement(args[0], fields)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Requirement updated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&identifier, "identifier", "", "New identifier")
	cmd.Flags().StringVar(&title, "title", "", "New title")
	cmd.Flags().StringVar(&description, "description", "", "New description")
	return cmd
}

func newReqDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a requirement",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.DeleteRequirement(args[0]); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Requirement deleted.")
			return nil
		},
	}
}

func newReqLinkCmd() *cobra.Command {
	var testID string
	cmd := &cobra.Command{
		Use:   "link <requirement-id>",
		Short: "Link a test case to a requirement",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.LinkRequirement(args[0], testID); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Test linked to requirement.")
			return nil
		},
	}
	cmd.Flags().StringVar(&testID, "test", "", "Test case ID (required)")
	cmd.MarkFlagRequired("test")
	return cmd
}

func newReqUnlinkCmd() *cobra.Command {
	var testID string
	cmd := &cobra.Command{
		Use:   "unlink <requirement-id>",
		Short: "Unlink a test case from a requirement",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.UnlinkRequirement(args[0], testID); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Test unlinked from requirement.")
			return nil
		},
	}
	cmd.Flags().StringVar(&testID, "test", "", "Test case ID (required)")
	cmd.MarkFlagRequired("test")
	return cmd
}

func newReqImportCmd() *cobra.Command {
	var source, key string
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import a requirement from Jira or Confluence",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.ImportRequirement(source, key)
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "Source type: jira or confluence (required)")
	cmd.Flags().StringVar(&key, "key", "", "Source key (e.g. PROJ-123) (required)")
	cmd.MarkFlagRequired("source")
	cmd.MarkFlagRequired("key")
	return cmd
}

func newReqBulkImportCmd() *cobra.Command {
	var source, keys string
	cmd := &cobra.Command{
		Use:   "bulk-import",
		Short: "Bulk import requirements",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.BulkImportRequirements(source, client.ParseCSV(keys))
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "Source type: jira or confluence (required)")
	cmd.Flags().StringVar(&keys, "keys", "", "Comma-separated source keys (required)")
	cmd.MarkFlagRequired("source")
	cmd.MarkFlagRequired("keys")
	return cmd
}

func newReqResyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resync <id>",
		Short: "Resync a requirement from its external source",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.ResyncRequirement(args[0])
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}

func newReqPostToJiraCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "post-to-jira <id>",
		Short: "Post linked test cases to Jira as a comment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.PostRequirementToJira(args[0])
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Test cases posted to Jira.")
			return nil
		},
	}
}
