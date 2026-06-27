package cmd

import (
	"ttgo/internal/cli/output"

	"github.com/spf13/cobra"
)

func newBackupsCmd() *cobra.Command {
	backupsCmd := &cobra.Command{
		Use:   "backups",
		Short: "Manage database backups (requires admin)",
	}

	scheduleCmd := &cobra.Command{
		Use:   "schedule",
		Short: "Manage backup schedule",
	}
	scheduleCmd.AddCommand(newBackupsScheduleGetCmd(), newBackupsScheduleSetCmd())

	backupsCmd.AddCommand(
		newBackupsListCmd(),
		newBackupsCreateCmd(),
		newBackupsRestoreCmd(),
		newBackupsDeleteCmd(),
		scheduleCmd,
	)

	return backupsCmd
}

func newBackupsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all backups",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.ListBackups()
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}

func newBackupsCreateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create",
		Short: "Create a manual backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.CreateBackup()
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Backup created.")
			return nil
		},
	}
}

func newBackupsRestoreCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <id>",
		Short: "Restore from a backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.RestoreBackup(args[0])
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Backup restored.")
			return nil
		},
	}
}

func newBackupsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			if err := c.DeleteBackup(args[0]); err != nil {
				return err
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Backup deleted.")
			return nil
		},
	}
}

func newBackupsScheduleGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Show backup schedule",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.GetBackupSchedule()
			if err != nil {
				return err
			}
			return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
		},
	}
}

func newBackupsScheduleSetCmd() *cobra.Command {
	var cron string
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set backup schedule",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.SetBackupSchedule(cron)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			output.PrintMessage(cmd.OutOrStdout(), outputMode(), "Backup schedule updated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&cron, "cron", "", "Cron expression (required)")
	cmd.MarkFlagRequired("cron")
	return cmd
}
