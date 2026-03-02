package jobs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/PhilipKram/jenkins-cli/internal/buildwatch"
	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
	"github.com/PhilipKram/jenkins-cli/internal/jenkins"
	"github.com/PhilipKram/jenkins-cli/internal/notification"
	"github.com/PhilipKram/jenkins-cli/internal/output"
	"github.com/spf13/cobra"
)

func newClient(cmd *cobra.Command) (*jenkins.Client, error) {
	timeout, _ := cmd.Root().Flags().GetDuration("timeout")
	retries, _ := cmd.Root().Flags().GetInt("retries")
	return clientutil.NewClient(timeout, retries)
}

var Cmd = &cobra.Command{
	Use:   "jobs",
	Short: "Manage Jenkins jobs",
}

var listCmd = &cobra.Command{
	Use:   "list [folder]",
	Short: "List all jobs",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		folder := ""
		if len(args) > 0 {
			folder = args[0]
		}

		jobs, err := client.ListJobs(ctx, folder)
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, jobs)
		}

		headers := []string{"NAME", "STATUS", "TYPE"}
		rows := make([][]string, len(jobs))
		for i, j := range jobs {
			status := jenkins.ColorToStatus(j.Color)
			typeName := shortClassName(j.Class)
			rows[i] = []string{j.Name, output.StatusColor(status), typeName}
		}
		output.PrintTable(os.Stdout, headers, rows)
		return nil
	},
}

var infoCmd = &cobra.Command{
	Use:   "info <job-name>",
	Short: "Show detailed job information",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		job, err := client.GetJob(ctx, args[0])
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, job)
		}

		fmt.Printf("Name:        %s\n", job.FullName)
		fmt.Printf("URL:         %s\n", job.URL)
		fmt.Printf("Status:      %s\n", output.StatusColor(jenkins.ColorToStatus(job.Color)))
		fmt.Printf("Buildable:   %v\n", job.Buildable)
		fmt.Printf("In Queue:    %v\n", job.InQueue)
		if job.Description != "" {
			fmt.Printf("Description: %s\n", job.Description)
		}
		if job.LastBuild != nil {
			fmt.Printf("Last Build:  #%d\n", job.LastBuild.Number)
		}
		if job.NextBuildNumber > 0 {
			fmt.Printf("Next Build:  #%d\n", job.NextBuildNumber)
		}
		for _, h := range job.HealthReport {
			fmt.Printf("Health:      %s (score: %d%%)\n", h.Description, h.Score)
		}
		return nil
	},
}

var buildCmd = &cobra.Command{
	Use:   "build <job-name>",
	Short: "Trigger a build for a job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		jobName := args[0]
		params, _ := cmd.Flags().GetStringToString("param")
		wait, _ := cmd.Flags().GetBool("wait")
		follow, _ := cmd.Flags().GetBool("follow")

		// Get job info to determine next build number before triggering
		var nextBuildNumber int
		if wait {
			job, err := client.GetJob(ctx, jobName)
			if err != nil {
				return err
			}
			nextBuildNumber = job.NextBuildNumber
		}

		// Trigger the build
		if err := client.BuildJob(ctx, jobName, params); err != nil {
			return err
		}

		fmt.Printf("Build triggered for %q\n", jobName)

		if !wait {
			return nil
		}

		// Wait for build to appear and complete
		fmt.Printf("Waiting for build #%d to start...\n", nextBuildNumber)

		// Handle timeout flag
		timeoutDur, _ := cmd.Flags().GetDuration("timeout")
		if timeoutDur > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeoutDur)
			defer cancel()
		}

		// Set up signal handling for graceful shutdown (Ctrl+C)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Fprintln(os.Stderr, "\nInterrupted. Exiting watch (build continues in Jenkins)...")
			cancel()
		}()

		// Wait for build to start (it may be queued)
		build, err := buildwatch.WaitForBuild(ctx, client, jobName, nextBuildNumber)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("timeout reached while waiting for build to start")
			}
			return err
		}

		fmt.Printf("Build #%d started\n", build.Number)

		// Stream logs if --follow is set
		if follow {
			if err := client.StreamBuildLog(ctx, jobName, build.Number, os.Stdout, 0); err != nil {
				if !errors.Is(err, context.Canceled) {
					fmt.Fprintf(os.Stderr, "Error streaming logs: %v\n", err)
				}
			}
		}

		// Watch the build until completion
		jsonOut, _ := cmd.Flags().GetBool("json")
		build, err = buildwatch.WatchBuild(ctx, client, jobName, build.Number, jsonOut, follow)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			if errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("timeout reached while waiting for build completion")
			}
			return err
		}

		// Send desktop notification on completion
		status := build.Status()
		if err := notification.SendBuildComplete(jobName, build.Number, status); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send notification: %v\n", err)
		}

		if !jsonOut {
			fmt.Printf("\nBuild completed: %s\n", output.StatusColor(status))
		}

		return buildwatch.BuildExitCode(status)
	},
}

var enableCmd = &cobra.Command{
	Use:   "enable <job-name>",
	Short: "Enable a job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := client.EnableJob(ctx, args[0]); err != nil {
			return err
		}
		fmt.Printf("Job %q enabled\n", args[0])
		return nil
	},
}

var disableCmd = &cobra.Command{
	Use:   "disable <job-name>",
	Short: "Disable a job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := client.DisableJob(ctx, args[0]); err != nil {
			return err
		}
		fmt.Printf("Job %q disabled\n", args[0])
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete <job-name>",
	Short: "Delete a job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}
		if err := client.DeleteJob(ctx, args[0]); err != nil {
			return err
		}
		fmt.Printf("Job %q deleted\n", args[0])
		return nil
	},
}

func init() {
	listCmd.Flags().Bool("json", false, "Output in JSON format")
	infoCmd.Flags().Bool("json", false, "Output in JSON format")
	buildCmd.Flags().StringToStringP("param", "p", nil, "Build parameters (key=value)")
	buildCmd.Flags().Bool("wait", false, "Wait for build to complete")
	buildCmd.Flags().Bool("follow", false, "Stream build logs (requires --wait)")
	buildCmd.Flags().Duration("timeout", 0, "Timeout for waiting (e.g., 30m, 1h)")
	buildCmd.Flags().Bool("json", false, "Output final build result in JSON format")

	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(infoCmd)
	Cmd.AddCommand(buildCmd)
	Cmd.AddCommand(enableCmd)
	Cmd.AddCommand(disableCmd)
	Cmd.AddCommand(deleteCmd)
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
