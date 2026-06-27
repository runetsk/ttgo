package cmd

import (
	"encoding/json"
	"fmt"
	"ttgo/internal/cli/output"

	"github.com/spf13/cobra"
)

func newUsersCmd() *cobra.Command {
	usersCmd := &cobra.Command{
		Use:   "users",
		Short: "Manage users (requires admin)",
	}

	usersCmd.AddCommand(
		newUsersListCmd(),
		newUsersCreateCmd(),
		newUsersUpdateCmd(),
		newUsersDeleteCmd(),
		newUsersRestoreCmd(),
	)

	return usersCmd
}

func newUsersListCmd() *cobra.Command {
	var includeDeleted bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all users",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.ListUsers(includeDeleted)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			var users []map[string]interface{}
			if err := json.Unmarshal(raw, &users); err != nil {
				return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
			}
			return output.Print(cmd.OutOrStdout(), outputMode(), users, []output.Column{
				{Header: "ID", Key: "id"},
				{Header: "EMAIL", Key: "email"},
				{Header: "NAME", Key: "display_name"},
				{Header: "ROLE", Key: "role"},
				{Header: "ACTIVE", Key: "active"},
			})
		},
	}
	cmd.Flags().BoolVar(&includeDeleted, "include-deleted", false, "Include soft-deleted users")
	return cmd
}

func newUsersCreateCmd() *cobra.Command {
	var email, displayName, password, role string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			result, err := c.CreateUser(email, displayName, password, role)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.Print(cmd.OutOrStdout(), "json", result, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created user %q (ID: %s)\n", result["email"], result["id"])
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "Email (required)")
	cmd.Flags().StringVar(&displayName, "display-name", "", "Display name (required)")
	cmd.Flags().StringVar(&password, "password", "", "Password (required)")
	cmd.Flags().StringVar(&role, "role", "member", "Role: member or admin")
	cmd.MarkFlagRequired("email")
	cmd.MarkFlagRequired("display-name")
	cmd.MarkFlagRequired("password")
	return cmd
}

func newUsersUpdateCmd() *cobra.Command {
	var email, displayName, role string
	var active bool
	var activeSet bool
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			fields := map[string]interface{}{}
			if email != "" {
				fields["email"] = email
			}
			if displayName != "" {
				fields["display_name"] = displayName
			}
			if role != "" {
				fields["role"] = role
			}
			if activeSet {
				fields["active"] = active
			}
			raw, err := c.UpdateUser(args[0], fields)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "User updated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "New email")
	cmd.Flags().StringVar(&displayName, "display-name", "", "New display name")
	cmd.Flags().StringVar(&role, "role", "", "New role")
	cmd.Flags().BoolVar(&active, "active", true, "Set active status")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		activeSet = cmd.Flags().Changed("active")
		return nil
	}
	return cmd
}

func newUsersDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Soft-delete a user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.DeleteUser(args[0]); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "User deleted.")
			return nil
		},
	}
}

func newUsersRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <id>",
		Short: "Restore a soft-deleted user",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.RestoreUser(args[0]); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "User restored.")
			return nil
		},
	}
}
