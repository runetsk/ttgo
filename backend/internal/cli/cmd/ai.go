package cmd

import (
	"fmt"
	"ttgo/internal/cli/output"

	"github.com/spf13/cobra"
)

func newAICmd() *cobra.Command {
	aiCmd := &cobra.Command{
		Use:   "ai",
		Short: "AI test generation",
	}

	providersCmd := &cobra.Command{
		Use:   "providers",
		Short: "Manage LLM providers",
	}
	providersCmd.AddCommand(
		newAIProvidersListCmd(),
		newAIProvidersCreateCmd(),
		newAIProvidersTestCmd(),
		newAIProvidersSetDefaultCmd(),
		newAIProvidersDeleteCmd(),
	)

	templateCmd := &cobra.Command{
		Use:   "template",
		Short: "Manage AI prompt template",
	}
	templateCmd.AddCommand(
		newAITemplateGetCmd(),
		newAITemplateSetCmd(),
		newAITemplateResetCmd(),
	)

	aiCmd.AddCommand(
		providersCmd,
		newAIGenerateCmd(),
		newAIAcceptCmd(),
		templateCmd,
	)

	return aiCmd
}

func newAIProvidersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List LLM providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.ListLLMProviders()
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}

func newAIProvidersCreateCmd() *cobra.Command {
	var label, providerType, endpoint, model string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new LLM provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			result, err := c.CreateLLMProvider(label, providerType, endpoint, model)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.Print(cmd.OutOrStdout(), "json", result, nil)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created provider %q (ID: %s)\n", result["label"], result["id"])
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "", "Provider label (required)")
	cmd.Flags().StringVar(&providerType, "type", "", "Provider type: local, openai, gemini, anthropic (required)")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "Endpoint URL (required)")
	cmd.Flags().StringVar(&model, "model", "", "Model name")
	cmd.MarkFlagRequired("label")
	cmd.MarkFlagRequired("type")
	cmd.MarkFlagRequired("endpoint")
	return cmd
}

func newAIProvidersTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <id>",
		Short: "Test a provider connection",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.TestLLMProvider(args[0])
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}

func newAIProvidersSetDefaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set-default <id>",
		Short: "Set a provider as default",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.SetDefaultLLMProvider(args[0]); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Default provider set.")
			return nil
		},
	}
}

func newAIProvidersDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.DeleteLLMProvider(args[0]); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Provider deleted.")
			return nil
		},
	}
}

func newAIGenerateCmd() *cobra.Command {
	var reqID, coverage string
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate tests from a requirement",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.GenerateTests(reqID, coverage)
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
	cmd.Flags().StringVar(&reqID, "requirement", "", "Requirement ID (required)")
	cmd.Flags().StringVar(&coverage, "coverage", "", "Coverage level: essential, thorough, comprehensive")
	cmd.MarkFlagRequired("requirement")
	return cmd
}

func newAIAcceptCmd() *cobra.Command {
	var reqID string
	cmd := &cobra.Command{
		Use:   "accept",
		Short: "Accept generated tests for a requirement",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.AcceptGeneratedTests(reqID)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Generated tests accepted.")
			return nil
		},
	}
	cmd.Flags().StringVar(&reqID, "requirement", "", "Requirement ID (required)")
	cmd.MarkFlagRequired("requirement")
	return cmd
}

func newAITemplateGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Show the current prompt template",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.GetAITemplate()
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}

func newAITemplateSetCmd() *cobra.Command {
	var content string
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Update the prompt template",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.SetAITemplate(content)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Template updated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "Template content (required)")
	cmd.MarkFlagRequired("content")
	return cmd
}

func newAITemplateResetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Reset template to default",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.ResetAITemplate(); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Template reset to default.")
			return nil
		},
	}
}
