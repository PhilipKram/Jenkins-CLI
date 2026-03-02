package system

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "View Jenkins system log",
	Example: `  jenkins-cli system log
  jenkins-cli system log -n 50`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		ctx := cmd.Context()
		logText, err := client.GetSystemLog(ctx)
		if err != nil {
			return err
		}

		lines, _ := cmd.Flags().GetInt("lines")
		if lines > 0 {
			all := strings.Split(logText, "\n")
			if len(all) > lines {
				all = all[len(all)-lines:]
			}
			logText = strings.Join(all, "\n")
		}

		fmt.Print(logText)
		return nil
	},
}

func init() {
	logCmd.Flags().IntP("lines", "n", 0, "Show last N lines (default: all)")

	Cmd.AddCommand(logCmd)
}
