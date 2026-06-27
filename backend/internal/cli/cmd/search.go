package cmd

import (
	"encoding/json"
	"ttgo/internal/cli/output"

	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search across tests, requirements, and runs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			raw, err := c.Search(args[0], limit, offset)
			if err != nil {
				return err
			}
			if outputMode() == "json" {
				return output.PrintRaw(cmd.OutOrStdout(), "json", raw)
			}
			var resp struct {
				Results []map[string]interface{} `json:"results"`
				Total   int                      `json:"total"`
				Query   string                   `json:"query"`
			}
			if err := json.Unmarshal(raw, &resp); err != nil {
				return output.PrintRaw(cmd.OutOrStdout(), outputMode(), raw)
			}
			return output.Print(cmd.OutOrStdout(), outputMode(), resp.Results, []output.Column{
				{Header: "TYPE", Key: "type"},
				{Header: "ID", Key: "id"},
				{Header: "NAME", Key: "name"},
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset")
	return cmd
}
