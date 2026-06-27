package cmd

import (
	"fmt"
	"strings"
	"ttgo/internal/cli/config"

	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
	}

	configCmd.AddCommand(newConfigShowCmd())
	configCmd.AddCommand(newConfigSetServerCmd())
	configCmd.AddCommand(newConfigSetTokenCmd())

	return configCmd
}

func newConfigShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			masked := cfg.APIToken
			if len(masked) > 8 {
				masked = masked[:4] + strings.Repeat("*", len(masked)-8) + masked[len(masked)-4:]
			} else if masked != "" {
				masked = "****"
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Server URL: %s\n", cfg.ServerURL)
			fmt.Fprintf(cmd.OutOrStdout(), "API Token:  %s\n", masked)
			return nil
		},
	}
}

func newConfigSetServerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-server <url>",
		Short: "Set the TTGO server URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			cfg.ServerURL = args[0]
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Server URL set to %s\n", args[0])
			return nil
		},
	}
}

func newConfigSetTokenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-token <token>",
		Short: "Set the API token",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			cfg.APIToken = args[0]
			if err := config.Save(cfg); err != nil {
				return fmt.Errorf("saving config: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "API token saved.")
			return nil
		},
	}
}
