package multibranch

import (
	"fmt"
	"os"
	"strings"

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

// Cmd is the root multibranch command.
var Cmd = &cobra.Command{
	Use:   "multibranch",
	Short: "Manage multibranch pipelines",
}

var branchesCmd = &cobra.Command{
	Use:   "branches <pipeline>",
	Short: "List branches in a multibranch pipeline",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		branches, err := client.ListJobs(ctx, args[0])
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, branches)
		}

		headers := []string{"BRANCH", "STATUS", "TYPE"}
		rows := make([][]string, len(branches))
		for i, b := range branches {
			status := jenkins.ColorToStatus(b.Color)
			typeName := shortClassName(b.Class)
			rows[i] = []string{b.Name, output.StatusColor(status), typeName}
		}
		output.PrintTable(os.Stdout, headers, rows)
		return nil
	},
}

var scanCmd = &cobra.Command{
	Use:   "scan <pipeline>",
	Short: "Trigger a branch indexing scan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		name := args[0]
		if err := client.ScanMultibranchPipeline(ctx, name); err != nil {
			return err
		}

		fmt.Printf("Branch scan triggered for %q\n", name)

		follow, _ := cmd.Flags().GetBool("follow")
		if follow {
			log, err := client.GetScanLog(ctx, name)
			if err != nil {
				return err
			}
			fmt.Print(log)
		}

		return nil
	},
}

var scanLogCmd = &cobra.Command{
	Use:   "scan-log <pipeline>",
	Short: "View the branch indexing log",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		log, err := client.GetScanLog(ctx, args[0])
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, map[string]string{
				"pipeline": args[0],
				"log":      log,
			})
		}

		fmt.Print(log)
		return nil
	},
}

func init() {
	branchesCmd.Flags().Bool("json", false, "Output in JSON format")
	scanCmd.Flags().Bool("follow", false, "Stream scan log after triggering")
	scanLogCmd.Flags().Bool("json", false, "Output in JSON format")

	Cmd.AddCommand(branchesCmd)
	Cmd.AddCommand(scanCmd)
	Cmd.AddCommand(scanLogCmd)
}

func shortClassName(class string) string {
	parts := strings.Split(class, ".")
	name := parts[len(parts)-1]
	switch name {
	case "FreeStyleProject":
		return "Freestyle"
	case "WorkflowJob":
		return "Pipeline"
	case "WorkflowMultiBranchProject":
		return "Multibranch"
	case "Folder":
		return "Folder"
	case "OrganizationFolder":
		return "OrgFolder"
	case "MatrixProject":
		return "Matrix"
	default:
		return name
	}
}
