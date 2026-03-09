package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/PhilipKram/jenkins-cli/cmd/api"
	"github.com/PhilipKram/jenkins-cli/cmd/auth"
	"github.com/PhilipKram/jenkins-cli/cmd/builds"
	"github.com/PhilipKram/jenkins-cli/cmd/configure"
	"github.com/PhilipKram/jenkins-cli/cmd/credentials"
	"github.com/PhilipKram/jenkins-cli/cmd/jobs"
	mcpcmd "github.com/PhilipKram/jenkins-cli/cmd/mcp"
	"github.com/PhilipKram/jenkins-cli/cmd/nodes"
	"github.com/PhilipKram/jenkins-cli/cmd/open"
	"github.com/PhilipKram/jenkins-cli/cmd/pipeline"
	"github.com/PhilipKram/jenkins-cli/cmd/plugins"
	"github.com/PhilipKram/jenkins-cli/cmd/queue"
	"github.com/PhilipKram/jenkins-cli/cmd/system"
	"github.com/PhilipKram/jenkins-cli/cmd/upgrade"
	"github.com/PhilipKram/jenkins-cli/cmd/view"
	"github.com/PhilipKram/jenkins-cli/internal/buildwatch"
	"github.com/PhilipKram/jenkins-cli/internal/output"
	"github.com/PhilipKram/jenkins-cli/internal/update"
	"github.com/PhilipKram/jenkins-cli/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "jenkins-cli",
	Short:   "A CLI tool for controlling Jenkins from the terminal",
	Version: version.Version,
	Long: `Jenkins CLI provides command-line access to your Jenkins server.

Manage jobs, builds, nodes, queue, plugins, and more directly from your terminal.
Configure your Jenkins connection with 'jenkins-cli configure' to get started.`,
	SilenceErrors: true, // We handle errors in Execute()
	SilenceUsage:  true, // Don't show usage on errors
}

func Execute() {
	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	var updateResult chan *update.CheckResult

	if os.Getenv("JENKINS_CLI_NO_UPDATE_CHECK") == "" {
		updateResult = make(chan *update.CheckResult, 1)
		go func() {
			updateResult <- update.CheckWithCache(version.Version)
		}()
	}

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		// Handle build exit codes (e.g., UNSTABLE=2, ABORTED=3)
		var exitErr *buildwatch.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}

		// Determine output format from --json flag
		format := output.FormatTable
		for _, arg := range os.Args {
			if arg == "--json" || strings.HasPrefix(arg, "--json=") {
				format = output.FormatJSON
				break
			}
		}

		// Print structured error to stderr
		if printErr := output.PrintError(os.Stderr, err, format); printErr != nil {
			os.Stderr.WriteString("Error: " + err.Error() + "\n")
		}
		os.Exit(1)
	}

	if updateResult != nil {
		select {
		case result := <-updateResult:
			if result != nil {
				fmt.Fprintf(os.Stderr, "\nA new version of jenkins-cli is available: v%s -> v%s\n", result.CurrentVersion, result.LatestVersion)
				fmt.Fprintln(os.Stderr, "Run 'jenkins-cli upgrade' to update")
			}
		default:
		}
	}
}

func init() {
	// Add global --json flag for structured output
	rootCmd.PersistentFlags().Bool("json", false, "Output in JSON format")
	rootCmd.PersistentFlags().Duration("timeout", 30*time.Second, "Request timeout duration")
	rootCmd.PersistentFlags().Int("retries", 3, "Maximum number of retries (0 to disable)")

	rootCmd.AddCommand(api.Cmd)
	rootCmd.AddCommand(auth.Cmd)
	rootCmd.AddCommand(configure.Cmd)
	rootCmd.AddCommand(credentials.Cmd)
	rootCmd.AddCommand(jobs.Cmd)
	rootCmd.AddCommand(builds.Cmd)
	rootCmd.AddCommand(nodes.Cmd)
	rootCmd.AddCommand(queue.Cmd)
	rootCmd.AddCommand(pipeline.Cmd)
	rootCmd.AddCommand(plugins.Cmd)
	rootCmd.AddCommand(system.Cmd)
	rootCmd.AddCommand(open.Cmd)
	rootCmd.AddCommand(upgrade.Cmd)
	rootCmd.AddCommand(view.Cmd)
	rootCmd.AddCommand(mcpcmd.NewCmdMCP())
}
