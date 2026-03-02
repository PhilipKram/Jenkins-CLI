package pipeline

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
	"github.com/PhilipKram/jenkins-cli/internal/jenkins"
	"github.com/spf13/cobra"
)

func newClient(cmd *cobra.Command) (*jenkins.Client, error) {
	timeout, _ := cmd.Root().Flags().GetDuration("timeout")
	retries, _ := cmd.Root().Flags().GetInt("retries")
	return clientutil.NewClient(timeout, retries)
}

var Cmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Manage Jenkins pipelines",
}

// jobArgs returns a cobra.PositionalArgs validator that accounts for --job flag.
// extraArgs is the number of positional args expected beyond the job name (e.g. 1 for replay requires build-number).
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

// jobArgsOptional returns a cobra.PositionalArgs validator for optional arguments beyond the job name.
// minArgs and maxArgs specify the range of extra args allowed (e.g. 0, 1 means build number is optional).
func jobArgsOptional(minArgs, maxArgs int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		job, _ := cmd.Flags().GetString("job")
		if job != "" {
			if len(args) < minArgs || len(args) > maxArgs {
				return fmt.Errorf("accepts %d to %d arg(s) when --job is used, received %d", minArgs, maxArgs, len(args))
			}
			return nil
		}
		// When job is not provided via flag, first arg is job name
		expectedMin := minArgs + 1
		expectedMax := maxArgs + 1
		if len(args) < expectedMin || len(args) > expectedMax {
			return fmt.Errorf("accepts %d to %d arg(s), received %d", expectedMin, expectedMax, len(args))
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

var replayCmd = &cobra.Command{
	Use:   "replay [job-name] <build-number>",
	Short: "Replay a pipeline build",
	Long: `Replay a pipeline build with optional modifications.

The replay command allows you to re-execute a pipeline build with the same or modified
Jenkinsfile without committing changes to the repository. This is useful for testing
pipeline changes and debugging build issues.

You can provide a modified Jenkinsfile in three ways:
  1. Using the --script flag to read from a file
  2. Piping the Jenkinsfile content via stdin
  3. Without modifications to replay with the original Jenkinsfile`,
	Example: `  jenkins-cli pipeline replay my-pipeline 42
  jenkins-cli pipeline replay --job my-pipeline 42
  jenkins-cli pipeline replay my-pipeline 42 --script Jenkinsfile.test
  jenkins-cli pipeline replay my-pipeline 42 --follow
  cat Jenkinsfile | jenkins-cli pipeline replay my-pipeline 42`,
	Args: jobArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobName, rest, err := resolveJobName(cmd, args)
		if err != nil {
			return err
		}

		buildNumber, err := strconv.Atoi(rest[0])
		if err != nil {
			return fmt.Errorf("invalid build number: %s", rest[0])
		}

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		// Read script from file, stdin, or use empty string for original
		var script string
		scriptFile, _ := cmd.Flags().GetString("script")

		if scriptFile != "" {
			// Read from file
			data, err := os.ReadFile(scriptFile)
			if err != nil {
				return fmt.Errorf("failed to read script file: %w", err)
			}
			script = string(data)
		} else {
			// Check if stdin has data (piped input)
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				// stdin is piped
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read from stdin: %w", err)
				}
				script = string(data)
			}
			// If neither file nor stdin, script remains empty and original build will be replayed
		}

		ctx := cmd.Context()
		newBuildNumber, err := client.ReplayBuild(ctx, jobName, buildNumber, script)
		if err != nil {
			return err
		}

		fmt.Printf("Build replayed successfully. New build number: #%d\n", newBuildNumber)

		follow, _ := cmd.Flags().GetBool("follow")
		if follow {
			fmt.Println("Streaming build log...")
			reader, err := client.GetBuildLog(ctx, jobName, newBuildNumber)
			if err != nil {
				return err
			}
			defer reader.Close()
			_, err = io.Copy(os.Stdout, reader)
			return err
		}

		return nil
	},
}

func init() {
	replayCmd.Flags().StringP("job", "j", "", "Job name (alternative to positional argument)")
	replayCmd.Flags().StringP("script", "s", "", "Path to modified Jenkinsfile")
	replayCmd.Flags().BoolP("follow", "f", false, "Stream the build log after replay")
	replayCmd.Flags().Bool("json", false, "Output in JSON format")

	Cmd.AddCommand(replayCmd)
}
