package queue

import (
	"fmt"
	"os"
	"strconv"
	"time"

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
	Use:   "queue",
	Short: "Manage Jenkins build queue",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List queued items",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		items, err := client.GetQueue(ctx)
		if err != nil {
			return err
		}

		if len(items) == 0 {
			fmt.Println("Build queue is empty")
			return nil
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, items)
		}

		headers := []string{"ID", "JOB", "WAITING SINCE", "REASON"}
		rows := make([][]string, len(items))
		for i, item := range items {
			since := time.UnixMilli(item.InQueueSince).Format("15:04:05")
			reason := item.Why
			if len(reason) > 60 {
				reason = reason[:57] + "..."
			}
			rows[i] = []string{
				fmt.Sprintf("%d", item.ID),
				item.Task.Name,
				since,
				reason,
			}
		}
		output.PrintTable(os.Stdout, headers, rows)
		return nil
	},
}

var cancelCmd = &cobra.Command{
	Use:   "cancel <queue-id>",
	Short: "Cancel a queued item",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid queue ID: %s", args[0])
		}

		if err := client.CancelQueueItem(ctx, id); err != nil {
			return err
		}

		fmt.Printf("Queue item %d cancelled\n", id)
		return nil
	},
}

func init() {
	listCmd.Flags().Bool("json", false, "Output in JSON format")
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(cancelCmd)
}
