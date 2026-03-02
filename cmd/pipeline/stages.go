package pipeline

import (
	"fmt"
	"os"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/jenkins"
	"github.com/PhilipKram/jenkins-cli/internal/output"
	"github.com/spf13/cobra"
)

// formatDuration formats a duration in milliseconds to a human-readable string
func formatDuration(durationMillis int64) string {
	d := time.Duration(durationMillis) * time.Millisecond
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// parallelGroup represents a group of stages that execute in parallel
type parallelGroup struct {
	startIndex int
	endIndex   int
}

// detectParallelStages analyzes stages and returns groups of parallel stages.
// Stages are considered parallel if they have start times within 1 second of each other.
func detectParallelStages(stages []jenkins.Stage) []parallelGroup {
	if len(stages) <= 1 {
		return nil
	}

	var groups []parallelGroup
	const parallelThreshold = 1000 // 1 second in milliseconds

	i := 0
	for i < len(stages) {
		groupStart := i
		groupEnd := i
		baseStartTime := stages[i].StartTimeMillis

		// Look ahead to find stages that start within the threshold
		for j := i + 1; j < len(stages); j++ {
			timeDiff := stages[j].StartTimeMillis - baseStartTime
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}

			if timeDiff <= parallelThreshold {
				groupEnd = j
			} else {
				break
			}
		}

		// If we found at least 2 stages in the group, record it
		if groupEnd > groupStart {
			groups = append(groups, parallelGroup{
				startIndex: groupStart,
				endIndex:   groupEnd,
			})
			i = groupEnd + 1
		} else {
			i++
		}
	}

	return groups
}

// getStagePrefix returns the prefix for a stage based on parallel grouping
func getStagePrefix(index int, groups []parallelGroup) string {
	for _, group := range groups {
		if index >= group.startIndex && index <= group.endIndex {
			if index == group.endIndex {
				return "└─ "
			}
			return "├─ "
		}
	}
	return ""
}

var stagesCmd = &cobra.Command{
	Use:   "stages [job-name] [build-number]",
	Short: "Show pipeline stages for a build",
	Example: `  jenkins-cli pipeline stages my-pipeline 42
  jenkins-cli pipeline stages my-pipeline
  jenkins-cli pipeline stages --job my-pipeline 42
  jenkins-cli pipeline stages -j my-pipeline
  jenkins-cli pipeline stages my-pipeline 42 --logs "Build"
  jenkins-cli pipeline stages my-pipeline last
  jenkins-cli pipeline stages my-pipeline lastSuccessful`,
	Args: jobArgsOptional(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		jobName, rest, err := resolveJobName(cmd, args)
		if err != nil {
			return err
		}

		// Default to "last" if no build number provided
		buildStr := "last"
		if len(rest) > 0 {
			buildStr = rest[0]
		}

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		buildNumber, err := resolveBuildNumber(ctx, client, jobName, buildStr)
		if err != nil {
			return err
		}

		pipelineRun, err := client.GetPipelineRun(ctx, jobName, buildNumber)
		if err != nil {
			return err
		}

		// Handle --logs flag
		logsFlag, _ := cmd.Flags().GetString("logs")
		if logsFlag != "" {
			// Find the stage by name
			var targetStage *jenkins.Stage
			for i := range pipelineRun.Stages {
				if pipelineRun.Stages[i].Name == logsFlag {
					targetStage = &pipelineRun.Stages[i]
					break
				}
			}

			if targetStage == nil {
				return fmt.Errorf("stage %q not found", logsFlag)
			}

			// Fetch and display stage logs
			logs, err := client.GetStageLog(ctx, jobName, buildNumber, targetStage)
			if err != nil {
				return err
			}

			fmt.Print(logs)
			return nil
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, pipelineRun)
		}

		// Detect parallel stages
		parallelGroups := detectParallelStages(pipelineRun.Stages)

		headers := []string{"NAME", "STATUS", "DURATION"}
		rows := make([][]string, len(pipelineRun.Stages))
		for i, stage := range pipelineRun.Stages {
			// Add prefix for parallel stages
			prefix := getStagePrefix(i, parallelGroups)
			stageName := prefix + stage.Name

			rows[i] = []string{
				stageName,
				output.StatusColor(stage.Status),
				formatDuration(stage.DurationMillis),
			}
		}
		output.PrintTable(os.Stdout, headers, rows)
		return nil
	},
}

func init() {
	stagesCmd.Flags().StringP("job", "j", "", "Job name (alternative to positional argument)")
	stagesCmd.Flags().Bool("json", false, "Output in JSON format")
	stagesCmd.Flags().String("logs", "", "Show logs for the specified stage name")

	Cmd.AddCommand(stagesCmd)
}
