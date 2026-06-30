package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDefectsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "defects", Short: "Manage native defects"}
	cmd.AddCommand(newDefectsListCmd(), newDefectsCreateCmd(), newDefectsLinkCmd(), newDefectsUnlinkCmd())
	return cmd
}

func newDefectsListCmd() *cobra.Command {
	var status, severity, q string
	cmd := &cobra.Command{
		Use: "list", Short: "List defects",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			params := map[string]string{}
			for k, v := range map[string]string{"status": status, "severity": severity, "q": q} {
				if v != "" {
					params[k] = v
				}
			}
			raw, err := c.ListDefects(params)
			if err != nil {
				return err
			}
			fmt.Println(string(raw))
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status (open|closed)")
	cmd.Flags().StringVar(&severity, "severity", "", "filter by severity")
	cmd.Flags().StringVar(&q, "q", "", "search title/external key")
	return cmd
}

func newDefectsCreateCmd() *cobra.Command {
	var title, desc, severity string
	cmd := &cobra.Command{
		Use: "create", Short: "Create a defect",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.CreateDefect(map[string]interface{}{"title": title, "description": desc, "severity": severity})
			if err != nil {
				return err
			}
			fmt.Println(string(raw))
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "defect title (required)")
	cmd.Flags().StringVar(&desc, "description", "", "defect description")
	cmd.Flags().StringVar(&severity, "severity", "minor", "severity")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

func newDefectsLinkCmd() *cobra.Command {
	return &cobra.Command{
		Use: "link <run-id> <result-id> <defect-id>", Short: "Link an existing defect to a run result",
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			return c.LinkDefect(args[0], args[1], args[2])
		},
	}
}

func newDefectsUnlinkCmd() *cobra.Command {
	return &cobra.Command{
		Use: "unlink <run-id> <result-id> <defect-id>", Short: "Unlink a defect from a run result",
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			return c.UnlinkDefect(args[0], args[1], args[2])
		},
	}
}
