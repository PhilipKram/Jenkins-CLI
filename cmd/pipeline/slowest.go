package pipeline

import (
	"fmt"
	"os"
	"sort"

	"github.com/PhilipKram/jenkins-cli/internal/jenkins"
	"github.com/PhilipKram/jenkins-cli/internal/output"
	"github.com/spf13/cobra"
)

type slowStage struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Duration string `json:"duration"`
	DurationMs int64 `json:"durationMs"`
}

var slowestCmd = &cobra.Command{
	Use:   "slowest [job-name] [build-number]",
	Short: "Show slowest pipeline stages",
	Long:  `Show the slowest pipeline stages for a build, sorted by duration.`,
	Example: `  jenkins-cli pipeline slowest my-pipeline
  jenkins-cli pipeline slowest my-pipeline 42
  jenkins-cli pipeline slowest --job my-pipeline --json`,
	Args: jobArgsOptional(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		jobName, rest, err := resolveJobName(cmd, args)
		if err != nil {
			return err
		}

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

		if len(pipelineRun.Stages) == 0 {
			fmt.Println("No stages found")
			return nil
		}

		// Sort stages by duration descending
		stages := make([]jenkins.Stage, len(pipelineRun.Stages))
		copy(stages, pipelineRun.Stages)
		sort.Slice(stages, func(i, j int) bool {
			return stages[i].DurationMillis > stages[j].DurationMillis
		})

		// Show top 5
		limit := 5
		if len(stages) < limit {
			limit = len(stages)
		}
		stages = stages[:limit]

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			result := make([]slowStage, len(stages))
			for i, s := range stages {
				result[i] = slowStage{
					Name:       s.Name,
					Status:     s.Status,
					Duration:   formatDuration(s.DurationMillis),
					DurationMs: s.DurationMillis,
				}
			}
			return output.PrintJSON(os.Stdout, result)
		}

		fmt.Printf("Slowest stages for %s #%d:\n\n", jobName, buildNumber)

		headers := []string{"RANK", "NAME", "STATUS", "DURATION"}
		rows := make([][]string, len(stages))
		for i, s := range stages {
			rows[i] = []string{
				fmt.Sprintf("#%d", i+1),
				s.Name,
				output.StatusColor(s.Status),
				formatDuration(s.DurationMillis),
			}
		}
		output.PrintTable(os.Stdout, headers, rows)
		return nil
	},
}

func init() {
	slowestCmd.Flags().StringP("job", "j", "", "Job name (alternative to positional argument)")
	slowestCmd.Flags().Bool("json", false, "Output in JSON format")

	Cmd.AddCommand(slowestCmd)
}
