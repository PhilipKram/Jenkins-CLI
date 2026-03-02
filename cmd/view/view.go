package view

import (
	"bufio"
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

// confirmPrompt displays a yes/no confirmation prompt and returns the user's choice.
// Returns true if the user confirms (y/yes), false otherwise (n/no or empty).
func confirmPrompt(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", message)

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "y" || input == "yes"
}

var Cmd = &cobra.Command{
	Use:   "view",
	Short: "Manage Jenkins views",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all views",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		views, err := client.ListViews(ctx)
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, views)
		}

		headers := []string{"NAME", "TYPE", "JOBS"}
		rows := make([][]string, len(views))
		for i, v := range views {
			typeName := shortClassName(v.Class)
			jobCount := fmt.Sprintf("%d", len(v.Jobs))
			rows[i] = []string{v.Name, typeName, jobCount}
		}
		output.PrintTable(os.Stdout, headers, rows)
		return nil
	},
}

var infoCmd = &cobra.Command{
	Use:   "info <view-name>",
	Short: "Show detailed view information",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		view, err := client.GetView(ctx, args[0])
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, view)
		}

		fmt.Printf("Name:        %s\n", view.Name)
		fmt.Printf("URL:         %s\n", view.URL)
		fmt.Printf("Type:        %s\n", shortClassName(view.Class))
		if view.Description != "" {
			fmt.Printf("Description: %s\n", view.Description)
		}
		fmt.Printf("Jobs:        %d\n", len(view.Jobs))
		if len(view.Jobs) > 0 {
			fmt.Println("\nContained Jobs:")
			for _, job := range view.Jobs {
				status := jenkins.ColorToStatus(job.Color)
				fmt.Printf("  - %s [%s]\n", job.Name, output.StatusColor(status))
			}
		}
		return nil
	},
}

func shortClassName(class string) string {
	parts := strings.Split(class, ".")
	name := parts[len(parts)-1]
	switch name {
	case "ListView":
		return "List"
	case "MyView":
		return "My"
	case "AllView":
		return "All"
	default:
		return name
	}
}

var createCmd = &cobra.Command{
	Use:   "create <view-name>",
	Short: "Create a new view",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		configFile, _ := cmd.Flags().GetString("config")
		var configXML string

		if configFile != "" {
			// Read config from file
			data, err := os.ReadFile(configFile)
			if err != nil {
				return fmt.Errorf("reading config file: %w", err)
			}
			configXML = string(data)
		} else {
			// Use default ListView template
			configXML = `<?xml version="1.0" encoding="UTF-8"?>
<hudson.model.ListView>
  <name>` + args[0] + `</name>
  <description></description>
  <filterExecutors>false</filterExecutors>
  <filterQueue>false</filterQueue>
  <properties class="hudson.model.View$PropertyList"/>
  <jobNames>
    <comparator class="hudson.util.CaseInsensitiveComparator"/>
  </jobNames>
  <jobFilters/>
  <columns>
    <hudson.views.StatusColumn/>
    <hudson.views.WeatherColumn/>
    <hudson.views.JobColumn/>
    <hudson.views.LastSuccessColumn/>
    <hudson.views.LastFailureColumn/>
    <hudson.views.LastDurationColumn/>
    <hudson.views.BuildButtonColumn/>
  </columns>
</hudson.model.ListView>`
		}

		if err := client.CreateView(ctx, args[0], configXML); err != nil {
			return err
		}
		fmt.Printf("View %q created\n", args[0])
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <view-name>",
	Short: "Delete a view",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		// Check if confirmation should be skipped
		skipConfirm, _ := cmd.Flags().GetBool("yes")

		if !skipConfirm {
			if !confirmPrompt(fmt.Sprintf("Are you sure you want to delete view %q?", args[0])) {
				fmt.Println("Deletion cancelled")
				return nil
			}
		}

		client, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := client.DeleteView(ctx, args[0]); err != nil {
			return err
		}
		fmt.Printf("View %q deleted\n", args[0])
		return nil
	},
}

var addJobCmd = &cobra.Command{
	Use:   "add-job <view-name> <job-name>",
	Short: "Add a job to a view",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := client.AddJobToView(ctx, args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("Job %q added to view %q\n", args[1], args[0])
		return nil
	},
}

var removeJobCmd = &cobra.Command{
	Use:   "remove-job <view-name> <job-name>",
	Short: "Remove a job from a view",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := client.RemoveJobFromView(ctx, args[0], args[1]); err != nil {
			return err
		}
		fmt.Printf("Job %q removed from view %q\n", args[1], args[0])
		return nil
	},
}

func init() {
	listCmd.Flags().Bool("json", false, "Output in JSON format")
	infoCmd.Flags().Bool("json", false, "Output in JSON format")
	createCmd.Flags().StringP("config", "c", "", "Path to XML config file (optional, defaults to ListView)")
	deleteCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(infoCmd)
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(deleteCmd)
	Cmd.AddCommand(addJobCmd)
	Cmd.AddCommand(removeJobCmd)
}
