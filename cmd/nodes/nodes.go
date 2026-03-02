package nodes

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
	Use:   "nodes",
	Short: "Manage Jenkins nodes/agents",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all nodes",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		nodes, err := client.ListNodes(ctx)
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, nodes)
		}

		headers := []string{"NAME", "STATUS", "EXECUTORS"}
		rows := make([][]string, len(nodes))
		for i, n := range nodes {
			rows[i] = []string{
				n.DisplayName,
				output.StatusColor(n.StatusStr()),
				fmt.Sprintf("%d", n.NumExecutors),
			}
		}
		output.PrintTable(os.Stdout, headers, rows)
		return nil
	},
}

var infoCmd = &cobra.Command{
	Use:   "info <node-name>",
	Short: "Show node details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		node, err := client.GetNode(ctx, args[0])
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, node)
		}

		fmt.Printf("Name:       %s\n", node.DisplayName)
		fmt.Printf("Status:     %s\n", output.StatusColor(node.StatusStr()))
		fmt.Printf("Executors:  %d\n", node.NumExecutors)
		if node.Description != "" {
			fmt.Printf("Description: %s\n", node.Description)
		}
		if node.Offline && node.OfflineCause != nil && node.OfflineCause.Reason != "" {
			fmt.Printf("Offline Reason: %s\n", node.OfflineCause.Reason)
		}
		return nil
	},
}

var offlineCmd = &cobra.Command{
	Use:   "offline <node-name>",
	Short: "Take a node offline",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		msg, _ := cmd.Flags().GetString("message")
		if err := client.ToggleNodeOffline(ctx, args[0], true, msg); err != nil {
			return err
		}
		fmt.Printf("Node %q taken offline\n", args[0])
		return nil
	},
}

var onlineCmd = &cobra.Command{
	Use:   "online <node-name>",
	Short: "Bring a node online",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		if err := client.ToggleNodeOffline(ctx, args[0], false, ""); err != nil {
			return err
		}
		fmt.Printf("Node %q brought online\n", args[0])
		return nil
	},
}

func init() {
	listCmd.Flags().Bool("json", false, "Output in JSON format")
	infoCmd.Flags().Bool("json", false, "Output in JSON format")
	offlineCmd.Flags().StringP("message", "m", "", "Offline reason message")

	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(infoCmd)
	Cmd.AddCommand(offlineCmd)
	Cmd.AddCommand(onlineCmd)
}
