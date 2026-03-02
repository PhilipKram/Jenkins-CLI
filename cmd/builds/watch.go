package builds

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/PhilipKram/jenkins-cli/internal/buildwatch"
	"github.com/PhilipKram/jenkins-cli/internal/notification"
	"github.com/PhilipKram/jenkins-cli/internal/output"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch [job-name] <build-number>",
	Short: "Watch a build and get notified on completion",
	Example: `  jenkins-cli builds watch my-pipeline 42
  jenkins-cli builds watch --job my-pipeline last
  jenkins-cli builds watch my-pipeline lastSuccessful
  jenkins-cli builds watch my-pipeline last --timeout 30m`,
	Args: jobArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		jobName, rest, err := resolveJobName(cmd, args)
		if err != nil {
			return err
		}

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		buildNumber, err := resolveBuildNumber(ctx, client, jobName, rest[0])
		if err != nil {
			return err
		}

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

		jsonOut, _ := cmd.Flags().GetBool("json")
		build, err := buildwatch.WatchBuild(ctx, client, jobName, buildNumber, jsonOut, false)
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
		if err := notification.SendBuildComplete(jobName, buildNumber, status); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send notification: %v\n", err)
		}

		if !jsonOut {
			fmt.Printf("\nBuild completed: %s\n", output.StatusColor(status))
		}

		return buildwatch.BuildExitCode(status)
	},
}
