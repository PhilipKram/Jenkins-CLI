package buildwatch

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/jenkins"
	"github.com/PhilipKram/jenkins-cli/internal/output"
)

// ExitError represents a non-zero exit code from a build result.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("build finished with exit code %d", e.Code)
}

// BuildExitCode returns nil for SUCCESS or an ExitError for other statuses.
// 0 = SUCCESS, 1 = FAILURE, 2 = UNSTABLE, 3 = ABORTED
func BuildExitCode(status string) error {
	switch status {
	case "SUCCESS":
		return nil
	case "UNSTABLE":
		return &ExitError{Code: 2}
	case "ABORTED":
		return &ExitError{Code: 3}
	default: // FAILURE and unknown
		return &ExitError{Code: 1}
	}
}

// WatchBuild polls a build until it completes, displaying progress updates.
// If suppressProgress is true, progress display is skipped (e.g., when streaming logs).
func WatchBuild(ctx context.Context, client *jenkins.Client, jobName string, buildNumber int, jsonOut bool, suppressProgress bool) (*jenkins.BuildDetail, error) {
	pollInterval := 2 * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var lastStatus string

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			build, err := client.GetBuild(ctx, jobName, buildNumber)
			if err != nil {
				return nil, fmt.Errorf("polling build status: %w", err)
			}

			if !jsonOut && !suppressProgress {
				currentStatus := build.Status()
				if currentStatus != lastStatus || build.Building {
					DisplayProgress(build)
					lastStatus = currentStatus
				}
			}

			if !build.Building {
				if jsonOut {
					return build, output.PrintJSON(os.Stdout, build)
				}
				return build, nil
			}
		}
	}
}

// DisplayProgress prints the current build progress to stdout.
func DisplayProgress(build *jenkins.BuildDetail) {
	duration := build.DurationStr()
	status := output.StatusColor(build.Status())
	fmt.Printf("\rBuild #%d: %s | Duration: %s", build.Number, status, duration)
	os.Stdout.Sync()
}

// WaitForBuild polls until the specified build number appears in Jenkins.
func WaitForBuild(ctx context.Context, client *jenkins.Client, jobName string, buildNumber int) (*jenkins.BuildDetail, error) {
	pollInterval := 2 * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			build, err := client.GetBuild(ctx, jobName, buildNumber)
			if err != nil {
				continue // Build doesn't exist yet
			}
			return build, nil
		}
	}
}
