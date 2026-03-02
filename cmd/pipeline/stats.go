package pipeline

import (
	"fmt"
	"os"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/output"
	"github.com/spf13/cobra"
)

type pipelineStats struct {
	TotalBuilds int            `json:"totalBuilds"`
	Counts      map[string]int `json:"counts"`
	Rates       map[string]string `json:"rates"`
	Duration    durationStats  `json:"duration"`
	Trend       string         `json:"trend"`
}

type durationStats struct {
	Average string `json:"average"`
	Min     string `json:"min"`
	Max     string `json:"max"`
}

var statsCmd = &cobra.Command{
	Use:   "stats [job-name]",
	Short: "Show pipeline build statistics",
	Long:  `Analyze recent builds to show success/failure rates, duration trends, and more.`,
	Example: `  jenkins-cli pipeline stats my-pipeline
  jenkins-cli pipeline stats my-pipeline -n 50
  jenkins-cli pipeline stats --job my-pipeline --json`,
	Args: jobArgsOptional(0, 0),
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

		numBuilds, _ := cmd.Flags().GetInt("builds")

		builds, err := client.ListBuilds(ctx, jobName, numBuilds)
		if err != nil {
			return err
		}

		if len(builds) == 0 {
			fmt.Println("No builds found")
			return nil
		}

		// Count results
		counts := map[string]int{}
		var durations []int64
		for _, b := range builds {
			status := b.Status()
			counts[status]++
			if !b.Building && b.Duration > 0 {
				durations = append(durations, b.Duration)
			}
		}

		// Calculate rates
		total := len(builds)
		rates := map[string]string{}
		for status, count := range counts {
			rates[status] = fmt.Sprintf("%.1f%%", float64(count)/float64(total)*100)
		}

		// Duration stats
		var avgDur, minDur, maxDur int64
		if len(durations) > 0 {
			minDur = durations[0]
			maxDur = durations[0]
			var sum int64
			for _, d := range durations {
				sum += d
				if d < minDur {
					minDur = d
				}
				if d > maxDur {
					maxDur = d
				}
			}
			avgDur = sum / int64(len(durations))
		}

		// Trend: compare first half vs second half average duration
		trend := "stable"
		if len(durations) >= 4 {
			mid := len(durations) / 2
			var firstSum, secondSum int64
			for _, d := range durations[:mid] {
				firstSum += d
			}
			for _, d := range durations[mid:] {
				secondSum += d
			}
			firstAvg := firstSum / int64(mid)
			secondAvg := secondSum / int64(len(durations)-mid)
			// Note: builds are ordered newest first, so "first half" = recent builds
			diff := float64(firstAvg-secondAvg) / float64(secondAvg)
			if diff > 0.15 {
				trend = "degrading"
			} else if diff < -0.15 {
				trend = "improving"
			}
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			stats := pipelineStats{
				TotalBuilds: total,
				Counts:      counts,
				Rates:       rates,
				Duration: durationStats{
					Average: fmtMs(avgDur),
					Min:     fmtMs(minDur),
					Max:     fmtMs(maxDur),
				},
				Trend: trend,
			}
			return output.PrintJSON(os.Stdout, stats)
		}

		fmt.Printf("Pipeline: %s (%d builds)\n\n", jobName, total)

		// Result breakdown
		fmt.Println("Results:")
		for _, status := range []string{"SUCCESS", "FAILURE", "UNSTABLE", "ABORTED"} {
			if count, ok := counts[status]; ok {
				fmt.Printf("  %-12s %d (%s)\n", status, count, rates[status])
			}
		}
		// Print any other statuses not in the standard list
		for status, count := range counts {
			switch status {
			case "SUCCESS", "FAILURE", "UNSTABLE", "ABORTED", "BUILDING":
			default:
				fmt.Printf("  %-12s %d (%s)\n", status, count, rates[status])
			}
		}

		if len(durations) > 0 {
			fmt.Printf("\nDuration:\n")
			fmt.Printf("  Average:  %s\n", fmtMs(avgDur))
			fmt.Printf("  Min:      %s\n", fmtMs(minDur))
			fmt.Printf("  Max:      %s\n", fmtMs(maxDur))
			fmt.Printf("  Trend:    %s\n", trend)
		}

		return nil
	},
}

func fmtMs(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func init() {
	statsCmd.Flags().StringP("job", "j", "", "Job name (alternative to positional argument)")
	statsCmd.Flags().IntP("builds", "n", 20, "Number of builds to analyze")
	statsCmd.Flags().Bool("json", false, "Output in JSON format")

	Cmd.AddCommand(statsCmd)
}
