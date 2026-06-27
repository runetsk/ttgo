package cmd

import (
	"encoding/json"
	"fmt"
	"ttgo/internal/cli/output"

	"github.com/spf13/cobra"
)

func newFoldersCmd() *cobra.Command {
	foldersCmd := &cobra.Command{
		Use:   "folders",
		Short: "Manage test case folders",
	}

	foldersCmd.AddCommand(
		newFoldersTreeCmd(),
		newFoldersGetCmd(),
		newFoldersCreateCmd(),
		newFoldersRenameCmd(),
		newFoldersMoveCmd(),
		newFoldersDeleteCmd(),
	)

	return foldersCmd
}

func newFoldersTreeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tree",
		Short: "Show the full folder hierarchy",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.GetFolderTreeRaw()
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			var folders []map[string]interface{}
			if err := json.Unmarshal(raw, &folders); err != nil {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			return output.Print(cmd.OutOrStdout(), outputMode(), folders, []output.Column{
				{Header: "ID", Key: "id"},
				{Header: "NAME", Key: "name"},
				{Header: "PARENT", Key: "parent_id"},
			})
		},
	}
}

func newFoldersGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a folder by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.GetFolder(args[0])
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}

func newFoldersCreateCmd() *cobra.Command {
	var name, parentID string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new folder",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			var pid *string
			if parentID != "" {
				pid = &parentID
			}
			result, err := c.CreateFolder(name, pid)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.Print(cmd.OutOrStdout(), "json", result, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created folder %s (ID: %s)\n", result["name"], result["id"])
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Folder name (required)")
	cmd.Flags().StringVar(&parentID, "parent", "", "Parent folder ID")
	cmd.MarkFlagRequired("name")
	return cmd
}

func newFoldersRenameCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "rename <id>",
		Short: "Rename a folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.RenameFolder(args[0], name); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Folder renamed.")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "New folder name (required)")
	cmd.MarkFlagRequired("name")
	return cmd
}

func newFoldersMoveCmd() *cobra.Command {
	var parentID string
	cmd := &cobra.Command{
		Use:   "move <id>",
		Short: "Move a folder to a new parent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			var pid *string
			if parentID != "" {
				pid = &parentID
			}
			if err := c.MoveFolder(args[0], pid); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Folder moved.")
			return nil
		},
	}
	cmd.Flags().StringVar(&parentID, "parent", "", "New parent folder ID (omit for root)")
	cmd.MarkFlagRequired("parent")
	return cmd
}

func newFoldersDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a folder",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.DeleteFolder(args[0]); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Folder deleted.")
			return nil
		},
	}
}
