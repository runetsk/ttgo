package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	"ttgo/internal/cli/client"
	"ttgo/internal/cli/output"
)

// adminOnly explains the 403 an admin write gets when it is made with an API token: the server
// accepts admin settings only from a signed-in admin session (requireAdmin refuses Bearer tokens).
func adminOnly(err error) error {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
		return fmt.Errorf("%w (this setting can only be changed by an admin signed in to the web UI; the server does not accept API tokens for admin settings)", err)
	}
	return err
}

func newAITypeSafeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "typesafe",
		Short: "TypeSafe.ai failure-analysis settings",
	}
	cmd.AddCommand(newAITypeSafeGetCmd(), newAITypeSafeSetCmd(), newAITypeSafeTestCmd())
	return cmd
}

func newAITypeSafeGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Show the TypeSafe.ai settings (the key is masked)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.GetTypeSafeSettings()
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}

// typeSafeFlagFields maps each `ttgo ai typesafe set` flag to its settings patch field.
var typeSafeFlagFields = []struct{ flag, field string }{
	{"enabled", "enabled"},
	{"model", "model"},
	{"timeout-seconds", "timeout_seconds"},
	{"verdict-engine", "verdict_engine_enabled"},
	{"narrative", "narrative_enabled"},
	{"llm-fallback", "llm_fallback_enabled"},
	{"escalate-below-pct", "escalate_below_pct"},
	{"semantic-dedup", "semantic_dedup_enabled"},
	{"auto-analysis", "allow_auto_failure_analysis"},
	{"price-per-mtok", "price_per_mtok"},
	{"clear-api-key", "clear_api_key"},
}

// typeSafePatch builds the PUT /settings/typesafe body from the flags that were actually set, so
// an omitted flag leaves its setting alone. The API key comes from stdin only: a flag value would
// land in shell history and the process list.
func typeSafePatch(cmd *cobra.Command, stdin io.Reader) (map[string]interface{}, error) {
	flags := cmd.Flags()
	patch := map[string]interface{}{}
	for _, m := range typeSafeFlagFields {
		f := flags.Lookup(m.flag)
		if f == nil || !f.Changed {
			continue
		}
		var v interface{}
		var err error
		switch f.Value.Type() {
		case "bool":
			v, err = flags.GetBool(m.flag)
		case "int":
			v, err = flags.GetInt(m.flag)
		case "float64":
			v, err = flags.GetFloat64(m.flag)
		default:
			v, err = flags.GetString(m.flag)
		}
		if err != nil {
			return nil, err
		}
		patch[m.field] = v
	}
	if keyFromStdin, _ := flags.GetBool("api-key-stdin"); keyFromStdin {
		if cleared, _ := flags.GetBool("clear-api-key"); cleared {
			return nil, fmt.Errorf("--api-key-stdin and --clear-api-key cannot be combined")
		}
		b, err := io.ReadAll(io.LimitReader(stdin, 64*1024))
		if err != nil {
			return nil, fmt.Errorf("reading the API key from stdin: %w", err)
		}
		key := strings.TrimSpace(string(b))
		if key == "" {
			return nil, fmt.Errorf("--api-key-stdin: stdin was empty")
		}
		patch["api_key"] = key
	}
	if len(patch) == 0 {
		return nil, fmt.Errorf("nothing to change: pass at least one setting flag (see --help)")
	}
	return patch, nil
}

func newAITypeSafeSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Change TypeSafe.ai settings (only the flags you pass are sent)",
		Long: `Changes the TypeSafe.ai failure-analysis settings. Only the flags you pass are
sent; every other setting keeps its value. Boolean flags need an explicit value to
switch something off, e.g. --narrative=false.

The API key is never a flag value (it would end up in shell history). Pipe it in:
  Get-Content key.txt | ttgo ai typesafe set --api-key-stdin

Admin only: the server accepts this from an admin signed in to the web UI, not from
an API token.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			patch, err := typeSafePatch(cmd, cmd.InOrStdin())
			if err != nil {
				return err
			}
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.UpdateTypeSafeSettings(patch)
			if err != nil {
				return adminOnly(err)
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
	f := cmd.Flags()
	f.Bool("enabled", false, "use TypeSafe.ai for failure analysis")
	f.String("model", "", "TypeSafe model id, e.g. jev-1.13.0 or jev-latest")
	f.Int("timeout-seconds", 30, "per-call timeout in seconds, 5-300")
	f.Bool("verdict-engine", true, "TypeSafe decides the verdict and the defect-type suggestion")
	f.Bool("narrative", true, "the default LLM writes an explanation of TypeSafe's decision")
	f.Bool("llm-fallback", true, "the default LLM decides when TypeSafe cannot answer")
	f.Int("escalate-below-pct", 0, "the LLM decides when TypeSafe's verdict confidence is below this percentage (0 = never)")
	f.Bool("semantic-dedup", true, "TypeSafe merges failure groups that share a cause")
	f.Bool("auto-analysis", false, "allow TypeSafe on analyses started automatically when a run completes")
	f.Float64("price-per-mtok", 0.042, "USD per million TypeSafe input tokens, used for cost estimates and budgets")
	f.Bool("clear-api-key", false, "remove the stored API key")
	f.Bool("api-key-stdin", false, "read a new API key from stdin")
	return cmd
}

func newAITypeSafeTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test",
		Short: "Test the stored TypeSafe.ai key and model (admin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.TestTypeSafeConnection()
			if err != nil {
				return adminOnly(err)
			}
			if err := output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw); err != nil {
				return err
			}
			var res struct {
				OK       bool   `json:"ok"`
				Category string `json:"category"`
				Message  string `json:"message"`
			}
			if json.Unmarshal(raw, &res) == nil && !res.OK {
				return fmt.Errorf("TypeSafe.ai connection test failed: %s: %s", res.Category, res.Message)
			}
			return nil
		},
	}
}

func newAIFeaturesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "features",
		Short: "The AI master switch",
	}
	get := &cobra.Command{
		Use:   "get",
		Short: "Show whether AI features are switched on",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.GetAIFeatureSettings()
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
	var enabled bool
	set := &cobra.Command{
		Use:   "set",
		Short: "Switch every AI feature on or off (admin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("enabled") {
				return fmt.Errorf("--enabled=<true|false> is required")
			}
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.SetAIFeatureSettings(enabled)
			if err != nil {
				return adminOnly(err)
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
	set.Flags().BoolVar(&enabled, "enabled", false, "true switches AI features on, false switches them off (write --enabled=false)")
	cmd.AddCommand(get, set)
	return cmd
}
