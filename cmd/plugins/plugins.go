package plugins

import (
	"fmt"
	"os"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
	"github.com/PhilipKram/jenkins-cli/internal/jenkins"
	"github.com/PhilipKram/jenkins-cli/internal/output"
	"github.com/spf13/cobra"
)

func newClient(cmd *cobra.Command) (*jenkins.Client, error) {
	timeout, _ := cmd.Root().Flags().GetDuration("timeout")
	retries, _ := cmd.Root().Flags().GetInt("retries")
	return clientutil.NewClient(timeout, retries)
}

var Cmd = &cobra.Command{
	Use:   "plugins",
	Short: "Manage Jenkins plugins",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		plugins, err := client.ListPlugins(ctx)
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, plugins)
		}

		updatesOnly, _ := cmd.Flags().GetBool("updates")

		headers := []string{"NAME", "VERSION", "ENABLED", "UPDATE"}
		var rows [][]string
		for _, p := range plugins {
			if updatesOnly && !p.HasUpdate {
				continue
			}
			enabled := "yes"
			if !p.Enabled {
				enabled = "no"
			}
			update := ""
			if p.HasUpdate {
				update = "available"
			}
			rows = append(rows, []string{p.ShortName, p.Version, enabled, update})
		}

		if len(rows) == 0 {
			if updatesOnly {
				fmt.Println("All plugins are up to date")
			} else {
				fmt.Println("No plugins installed")
			}
			return nil
		}

		output.PrintTable(os.Stdout, headers, rows)
		fmt.Printf("\nTotal: %d plugins\n", len(rows))
		return nil
	},
}

func init() {
	listCmd.Flags().Bool("json", false, "Output in JSON format")
	listCmd.Flags().BoolP("updates", "u", false, "Show only plugins with available updates")
	Cmd.AddCommand(listCmd)
}
