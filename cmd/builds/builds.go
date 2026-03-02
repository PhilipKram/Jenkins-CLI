package builds

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

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
	Use:   "builds",
	Short: "Manage Jenkins builds",
}

// jobArgs returns a cobra.PositionalArgs validator that accounts for --job flag.
// extraArgs is the number of positional args expected beyond the job name (e.g. 0 for list, 1 for info).
func jobArgs(extraArgs int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		job, _ := cmd.Flags().GetString("job")
		if job != "" {
			if len(args) != extraArgs {
				return fmt.Errorf("accepts %d arg(s) when --job is used, received %d", extraArgs, len(args))
			}
			return nil
		}
		expected := extraArgs + 1
		if len(args) != expected {
			return fmt.Errorf("accepts %d arg(s), received %d", expected, len(args))
		}
		return nil
	}
}

// resolveJobName gets the job name from --job flag or first positional arg.
// Returns the job name and remaining args after extracting the job name.
func resolveJobName(cmd *cobra.Command, args []string) (string, []string, error) {
	job, _ := cmd.Flags().GetString("job")
	if job != "" {
		return job, args, nil
	}
	if len(args) == 0 {
		return "", nil, fmt.Errorf("job name required: provide as argument or use --job flag")
	}
	return args[0], args[1:], nil
}

// resolveBuildNumber resolves a build number string (numeric or alias like "last", "lastSuccessful")
// to an actual build number.
func resolveBuildNumber(ctx context.Context, client *jenkins.Client, jobName, buildStr string) (int, error) {
	switch buildStr {
	case "last":
		build, err := client.GetLastBuild(ctx, jobName)
		if err != nil {
			return 0, err
		}
		return build.Number, nil
	case "lastSuccessful":
		build, err := client.GetLastSuccessfulBuild(ctx, jobName)
		if err != nil {
			return 0, err
		}
		return build.Number, nil
	default:
		number, err := strconv.Atoi(buildStr)
		if err != nil {
			return 0, fmt.Errorf("invalid build number: %s (must be a number, 'last', or 'lastSuccessful')", buildStr)
		}
		return number, nil
	}
}

var listCmd = &cobra.Command{
	Use:   "list [job-name]",
	Short: "List builds for a job",
	Example: `  jenkins-cli builds list my-pipeline
  jenkins-cli builds list --job my-pipeline
  jenkins-cli builds list -j "folder/my-pipeline" -n 20`,
	Args: jobArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		jobName, _, err := resolveJobName(cmd, args)
		if err != nil {
			return err
		}

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		builds, err := client.ListBuilds(ctx, jobName, limit)
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, builds)
		}

		headers := []string{"NUMBER", "STATUS", "DURATION", "STARTED"}
		rows := make([][]string, len(builds))
		for i, b := range builds {
			rows[i] = []string{
				fmt.Sprintf("#%d", b.Number),
				output.StatusColor(b.Status()),
				b.DurationStr(),
				b.StartTime().Format("2006-01-02 15:04:05"),
			}
		}
		output.PrintTable(os.Stdout, headers, rows)
		return nil
	},
}

var infoCmd = &cobra.Command{
	Use:   "info [job-name] <build-number>",
	Short: "Show detailed build information",
	Example: `  jenkins-cli builds info my-pipeline 42
  jenkins-cli builds info --job my-pipeline 42`,
	Args: jobArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		jobName, rest, err := resolveJobName(cmd, args)
		if err != nil {
			return err
		}

		number, err := strconv.Atoi(rest[0])
		if err != nil {
			return fmt.Errorf("invalid build number: %s", rest[0])
		}

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		build, err := client.GetBuild(ctx, jobName, number)
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, build)
		}

		fmt.Printf("Build:       #%d\n", build.Number)
		fmt.Printf("Name:        %s\n", build.FullName)
		fmt.Printf("Status:      %s\n", output.StatusColor(build.Status()))
		fmt.Printf("Duration:    %s\n", build.DurationStr())
		fmt.Printf("Started:     %s\n", build.StartTime().Format("2006-01-02 15:04:05"))
		fmt.Printf("URL:         %s\n", build.URL)

		if build.Description != "" {
			fmt.Printf("Description: %s\n", build.Description)
		}

		for _, a := range build.Actions {
			for _, cause := range a.Causes {
				fmt.Printf("Cause:       %s\n", cause.ShortDescription)
			}
		}

		if len(build.Artifacts) > 0 {
			fmt.Println("\nArtifacts:")
			for _, a := range build.Artifacts {
				fmt.Printf("  - %s\n", a.FileName)
			}
		}

		if len(build.ChangeSet.Items) > 0 {
			fmt.Printf("\nChanges (%s):\n", build.ChangeSet.Kind)
			for _, c := range build.ChangeSet.Items {
				fmt.Printf("  - %s (%s)\n", c.Message, c.Author.FullName)
			}
		}

		return nil
	},
}

var logCmd = &cobra.Command{
	Use:   "log [job-name] <build-number>",
	Short: "Show console output for a build",
	Example: `  jenkins-cli builds log my-pipeline 42
  jenkins-cli builds log --job my-pipeline 42
  jenkins-cli builds log my-pipeline 42 --follow`,
	Args: jobArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		jobName, rest, err := resolveJobName(cmd, args)
		if err != nil {
			return err
		}

		number, err := strconv.Atoi(rest[0])
		if err != nil {
			return fmt.Errorf("invalid build number: %s", rest[0])
		}

		follow, _ := cmd.Flags().GetBool("follow")
		jsonOut, _ := cmd.Flags().GetBool("json")

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		// Set up signal handling for graceful shutdown when following
		if follow {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-sigCh
				cancel()
			}()

			// Stream logs in real-time
			// Use buffered I/O to reduce syscall overhead
			writer := bufio.NewWriterSize(os.Stdout, 32*1024) // 32KB buffer
			if err := client.StreamBuildLog(ctx, jobName, number, writer, 0); err != nil {
				writer.Flush() // Ensure any buffered content is written
				// Check if error is due to context cancellation (graceful exit)
				if errors.Is(err, context.Canceled) {
					return nil
				}
				return err
			}
			// Flush the buffer to ensure all data is written
			if err := writer.Flush(); err != nil {
				return fmt.Errorf("flushing output buffer: %w", err)
			}

			// Fetch final build status after streaming completes
			build, err := client.GetBuild(ctx, jobName, number)
			if err != nil {
				return fmt.Errorf("fetching final build status: %w", err)
			}

			// Display final build result (unless --json is used)
			if !jsonOut {
				fmt.Printf("\nBuild completed: %s\n", output.StatusColor(build.Status()))
			}

			// Set exit code based on build result
			status := build.Status()
			if status == "SUCCESS" {
				return nil
			}
			return fmt.Errorf("build failed with status: %s", status)
		}

		reader, err := client.GetBuildLog(ctx, jobName, number)
		if err != nil {
			return err
		}
		defer reader.Close()

		// Use buffered I/O to reduce syscall overhead
		writer := bufio.NewWriterSize(os.Stdout, 32*1024) // 32KB buffer
		_, err = io.Copy(writer, reader)
		if err != nil {
			return err
		}

		// Flush the buffer to ensure all data is written
		return writer.Flush()
	},
}

var lastCmd = &cobra.Command{
	Use:   "last [job-name]",
	Short: "Show the last build for a job",
	Example: `  jenkins-cli builds last my-pipeline
  jenkins-cli builds last --job my-pipeline`,
	Args: jobArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		jobName, _, err := resolveJobName(cmd, args)
		if err != nil {
			return err
		}

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		build, err := client.GetLastBuild(ctx, jobName)
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, build)
		}

		fmt.Printf("Build:    #%d\n", build.Number)
		fmt.Printf("Status:   %s\n", output.StatusColor(build.Status()))
		fmt.Printf("Duration: %s\n", build.DurationStr())
		fmt.Printf("Started:  %s\n", build.StartTime().Format("2006-01-02 15:04:05"))
		fmt.Printf("URL:      %s\n", build.URL)
		return nil
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop [job-name] <build-number>",
	Short: "Stop a running build",
	Example: `  jenkins-cli builds stop my-pipeline 42
  jenkins-cli builds stop --job my-pipeline 42`,
	Args: jobArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		jobName, rest, err := resolveJobName(cmd, args)
		if err != nil {
			return err
		}

		number, err := strconv.Atoi(rest[0])
		if err != nil {
			return fmt.Errorf("invalid build number: %s", rest[0])
		}

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		if err := client.StopBuild(ctx, jobName, number); err != nil {
			return err
		}

		fmt.Printf("Build #%d of %q stopped\n", number, jobName)
		return nil
	},
}

var killCmd = &cobra.Command{
	Use:   "kill [job-name] <build-number>",
	Short: "Force kill a running build",
	Long: `Force kill a running build immediately.

WARNING: This forcefully terminates the build process and may leave resources
in an inconsistent state (locks not released, temporary files not cleaned up,
post-build actions not executed, etc.).

Use 'jenkins builds stop' for graceful termination. Only use 'kill' when:
  - A build is stuck with blocking input steps
  - The build doesn't respond to the normal stop command
  - You need immediate termination during an incident

This command requires confirmation unless --yes flag is used.`,
	Example: `  jenkins-cli builds kill my-pipeline 42
  jenkins-cli builds kill --job my-pipeline 42
  jenkins-cli builds kill my-pipeline 42 --yes`,
	Args: jobArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		jobName, rest, err := resolveJobName(cmd, args)
		if err != nil {
			return err
		}

		number, err := strconv.Atoi(rest[0])
		if err != nil {
			return fmt.Errorf("invalid build number: %s", rest[0])
		}

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		// Check if confirmation should be skipped
		skipConfirm, _ := cmd.Flags().GetBool("yes")

		if !skipConfirm {
			fmt.Println("\nWARNING: Force-killing a build may leave resources in an inconsistent state.")
			fmt.Println("         Use 'jenkins builds stop' for graceful termination when possible.")

			if !confirmPrompt(fmt.Sprintf("Force-kill build #%d of %q?", number, jobName)) {
				fmt.Println("Operation cancelled.")
				return nil
			}
		}

		// Verify build is actually running
		build, err := client.GetBuild(ctx, jobName, number)
		if err != nil {
			return fmt.Errorf("failed to get build info: %w", err)
		}

		if !build.Building {
			return fmt.Errorf("cannot kill build #%d: build is not running (status: %s)",
				number, build.Status())
		}

		if err := client.KillBuild(ctx, jobName, number); err != nil {
			return err
		}

		fmt.Printf("Build #%d of %q killed\n", number, jobName)
		return nil
	},
}

var artifactsCmd = &cobra.Command{
	Use:   "artifacts [job-name] <build-number>",
	Short: "List artifacts for a build",
	Example: `  jenkins-cli builds artifacts my-pipeline 42
  jenkins-cli builds artifacts --job my-pipeline 42
  jenkins-cli builds artifacts my-pipeline 42 --json
  jenkins-cli builds artifacts my-pipeline 42 --download
  jenkins-cli builds artifacts my-pipeline 42 --filter '*.jar'
  jenkins-cli builds artifacts my-pipeline 42 --download --output-dir ./artifacts
  jenkins-cli builds artifacts my-pipeline last
  jenkins-cli builds artifacts my-pipeline lastSuccessful`,
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

		number, err := resolveBuildNumber(ctx, client, jobName, rest[0])
		if err != nil {
			return err
		}

		artifacts, err := client.GetBuildArtifacts(ctx, jobName, number)
		if err != nil {
			return err
		}

		// Apply filter if specified
		filter, _ := cmd.Flags().GetString("filter")
		if filter != "" {
			filtered := []jenkins.Artifact{}
			for _, a := range artifacts {
				matched, err := filepath.Match(filter, a.FileName)
				if err != nil {
					return fmt.Errorf("invalid filter pattern: %w", err)
				}
				if matched {
					filtered = append(filtered, a)
				}
			}
			artifacts = filtered
		}

		download, _ := cmd.Flags().GetBool("download")
		if download {
			outputDir, _ := cmd.Flags().GetString("output-dir")
			return downloadArtifacts(ctx, client, jobName, number, artifacts, outputDir)
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, artifacts)
		}

		if len(artifacts) == 0 {
			fmt.Println("No artifacts found")
			return nil
		}

		headers := []string{"FILENAME", "SIZE", "PATH"}
		rows := make([][]string, len(artifacts))
		for i, a := range artifacts {
			rows[i] = []string{
				a.FileName,
				formatSize(a.Size),
				a.RelativePath,
			}
		}
		output.PrintTable(os.Stdout, headers, rows)
		return nil
	},
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func downloadArtifacts(ctx context.Context, client *jenkins.Client, jobName string, buildNumber int, artifacts []jenkins.Artifact, outputDir string) error {
	if len(artifacts) == 0 {
		fmt.Println("No artifacts to download")
		return nil
	}

	// Use current directory if outputDir is not specified
	if outputDir == "" {
		outputDir = "."
	}

	// Resolve output directory to absolute path
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("resolving output directory: %w", err)
	}

	fmt.Printf("Downloading %d artifact(s)...\n", len(artifacts))

	for _, artifact := range artifacts {
		// Join output directory with artifact relative path
		targetPath := filepath.Join(absOutputDir, artifact.RelativePath)

		// Validate path is within output directory (prevent path traversal)
		absTargetPath, err := filepath.Abs(targetPath)
		if err != nil {
			return fmt.Errorf("resolving target path: %w", err)
		}

		relPath, err := filepath.Rel(absOutputDir, absTargetPath)
		if err != nil || strings.HasPrefix(relPath, "..") || filepath.IsAbs(relPath) {
			return fmt.Errorf("invalid artifact path (path traversal detected): %s", artifact.RelativePath)
		}

		// Create directory structure for nested paths
		dir := filepath.Dir(absTargetPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}

		// Open file for writing
		file, err := os.Create(absTargetPath)
		if err != nil {
			return fmt.Errorf("creating file %s: %w", absTargetPath, err)
		}

		// Show progress for files > 1MB
		const oneMB = 1024 * 1024
		var progressCallback jenkins.ProgressCallback
		if artifact.Size > oneMB {
			fmt.Printf("Downloading %s (%s)...\n", absTargetPath, formatSize(artifact.Size))
			lastPercent := -1
			progressCallback = func(downloaded, total int64) {
				if total > 0 {
					percent := int(100 * downloaded / total)
					if percent != lastPercent && percent%10 == 0 {
						fmt.Printf("  %d%% (%s / %s)\n", percent, formatSize(downloaded), formatSize(total))
						lastPercent = percent
					}
				}
			}
		} else {
			fmt.Printf("Downloading %s...\n", absTargetPath)
		}

		// Download artifact
		err = client.DownloadArtifact(ctx, jobName, buildNumber, artifact.RelativePath, file, progressCallback)
		file.Close()

		if err != nil {
			os.Remove(absTargetPath)
			return fmt.Errorf("downloading %s: %w", absTargetPath, err)
		}

		fmt.Printf("  ✓ Saved to %s\n", absTargetPath)
	}

	fmt.Println("All artifacts downloaded successfully")
	return nil
}

func init() {
	for _, cmd := range []*cobra.Command{listCmd, infoCmd, logCmd, lastCmd, stopCmd, killCmd, artifactsCmd, watchCmd} {
		cmd.Flags().StringP("job", "j", "", "Job name (alternative to positional argument)")
	}

	listCmd.Flags().IntP("limit", "n", 10, "Number of builds to show")
	listCmd.Flags().Bool("json", false, "Output in JSON format")
	infoCmd.Flags().Bool("json", false, "Output in JSON format")
	logCmd.Flags().BoolP("follow", "f", false, "Follow log output in real-time")
	logCmd.Flags().Bool("json", false, "Output in JSON format")
	lastCmd.Flags().Bool("json", false, "Output in JSON format")
	artifactsCmd.Flags().Bool("json", false, "Output in JSON format")
	artifactsCmd.Flags().Bool("download", false, "Download artifacts to current directory")
	artifactsCmd.Flags().String("filter", "", "Filter artifacts by glob pattern (e.g., '*.jar', 'app-*.war')")
	artifactsCmd.Flags().String("output-dir", "", "Directory to download artifacts to (default: current directory)")
	watchCmd.Flags().Duration("timeout", 0, "Maximum time to wait for build completion (e.g., 30m, 1h)")
	watchCmd.Flags().Bool("json", false, "Output in JSON format")

	killCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(infoCmd)
	Cmd.AddCommand(logCmd)
	Cmd.AddCommand(lastCmd)
	Cmd.AddCommand(stopCmd)
	Cmd.AddCommand(killCmd)
	Cmd.AddCommand(artifactsCmd)
	Cmd.AddCommand(watchCmd)
}
