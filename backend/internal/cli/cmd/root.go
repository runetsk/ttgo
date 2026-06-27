package cmd

import (
	"fmt"
	"os"

	"ttgo/internal/cli/client"
	"ttgo/internal/cli/config"

	"github.com/spf13/cobra"
)

var (
	flagServer string
	flagToken  string
	flagOutput string
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:          "ttgo",
		Short:        "CLI for TTGO test management",
		Long:         "A command-line interface for managing test cases, runs, and analytics in TTGO.",
		SilenceUsage: true,
	}

	root.PersistentFlags().StringVarP(&flagServer, "server", "s", "", "TTGO server URL (overrides config)")
	root.PersistentFlags().StringVarP(&flagToken, "token", "t", "", "API token (overrides config)")
	root.PersistentFlags().StringVarP(&flagOutput, "output", "o", "table", "Output format: table, json, plain")

	root.AddCommand(newConfigCmd())
	root.AddCommand(newFoldersCmd())
	root.AddCommand(newTestsCmd())
	root.AddCommand(newRunsCmd())
	root.AddCommand(newSearchCmd())
	root.AddCommand(newCategoriesCmd())
	root.AddCommand(newRequirementsCmd())
	root.AddCommand(newAnalyticsCmd())
	root.AddCommand(newDefectsCmd())
	root.AddCommand(newBackupsCmd())
	root.AddCommand(newWebhooksCmd())
	root.AddCommand(newAICmd())
	root.AddCommand(newUsersCmd())

	return root
}

// newClient creates an API client from config + flag overrides.
func newClient() (*client.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	serverURL := cfg.ServerURL
	if flagServer != "" {
		serverURL = flagServer
	}

	token := cfg.APIToken
	if flagToken != "" {
		token = flagToken
	}

	if token == "" {
		return nil, fmt.Errorf("no API token configured. Run 'ttgo config set-token <token>' or set TTGO_API_TOKEN")
	}

	return client.New(serverURL, token), nil
}

func outputMode() string {
	return flagOutput
}

func Execute() {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
