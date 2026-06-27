package cmd

import (
	"fmt"
	"ttgo/internal/cli/output"

	"github.com/spf13/cobra"
)

func newWebhooksCmd() *cobra.Command {
	webhooksCmd := &cobra.Command{
		Use:   "webhooks",
		Short: "Manage webhook configurations",
	}

	webhooksCmd.AddCommand(
		newWebhooksListCmd(),
		newWebhooksCreateCmd(),
		newWebhooksDeleteCmd(),
	)

	return webhooksCmd
}

func newWebhooksListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all webhooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.ListWebhooks()
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}

func newWebhooksCreateCmd() *cobra.Command {
	var url, eventType, description string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a webhook",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			result, err := c.CreateWebhook(url, eventType, description)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.Print(cmd.OutOrStdout(), "json", result, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created webhook (ID: %s)\n", result["id"])
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "Webhook URL (required)")
	cmd.Flags().StringVar(&eventType, "event", "", "Event type, e.g. run.completed (required)")
	cmd.Flags().StringVar(&description, "description", "", "Description")
	cmd.MarkFlagRequired("url")
	cmd.MarkFlagRequired("event")
	return cmd
}

func newWebhooksDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a webhook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.DeleteWebhook(args[0]); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Webhook deleted.")
			return nil
		},
	}
}
