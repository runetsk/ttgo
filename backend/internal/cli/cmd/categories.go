package cmd

import (
	"encoding/json"
	"fmt"
	"ttgo/internal/cli/output"

	"github.com/spf13/cobra"
)

func newCategoriesCmd() *cobra.Command {
	categoriesCmd := &cobra.Command{
		Use:   "categories",
		Short: "Manage test categories",
	}

	categoriesCmd.AddCommand(
		newCategoriesListCmd(),
		newCategoriesCreateCmd(),
		newCategoriesDeleteCmd(),
		newCategoriesAssignCmd(),
	)

	return categoriesCmd
}

func newCategoriesListCmd() *cobra.Command {
	var search string
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List test categories",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.ListCategories(search, limit, offset)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			var resp struct {
				Categories []map[string]interface{} `json:"categories"`
				Total      int                      `json:"total"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
			}
			return output.Print(cmd.OutOrStdout(), outputMode(), resp.Categories, []output.Column{
				{Header: "ID", Key: "id"},
				{Header: "NAME", Key: "name"},
				{Header: "DESCRIPTION", Key: "description"},
			})
		},
	}
	cmd.Flags().StringVar(&search, "search", "", "Search query")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max results")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset")
	return cmd
}

func newCategoriesCreateCmd() *cobra.Command {
	var name, description string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new category",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			result, err := c.CreateCategory(name, description)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.Print(cmd.OutOrStdout(), "json", result, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created category %q (ID: %s)\n", result["name"], result["id"])
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Category name (required)")
	cmd.Flags().StringVar(&description, "description", "", "Category description")
	cmd.MarkFlagRequired("name")
	return cmd
}

func newCategoriesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a category",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.DeleteCategory(args[0]); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Category deleted.")
			return nil
		},
	}
}

func newCategoriesAssignCmd() *cobra.Command {
	var testID, categoryID string
	cmd := &cobra.Command{
		Use:   "assign",
		Short: "Assign a test case to a category",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.AssignCategory(testID, categoryID); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Test assigned to category.")
			return nil
		},
	}
	cmd.Flags().StringVar(&testID, "test", "", "Test case ID (required)")
	cmd.Flags().StringVar(&categoryID, "category", "", "Category ID (required)")
	cmd.MarkFlagRequired("test")
	cmd.MarkFlagRequired("category")
	return cmd
}
