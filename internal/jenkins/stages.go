package jenkins

import (
	"context"
	"fmt"
)

// PipelineRun represents the complete pipeline execution data from wfapi.
type PipelineRun struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Status           string  `json:"status"`
	StartTimeMillis  int64   `json:"startTimeMillis"`
	EndTimeMillis    int64   `json:"endTimeMillis"`
	DurationMillis   int64   `json:"durationMillis"`
	PauseDurationMillis int64 `json:"pauseDurationMillis,omitempty"`
	Stages           []Stage `json:"stages"`
}

// Stage represents a single pipeline stage.
type Stage struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	ExecNode            string          `json:"execNode"`
	Status              string          `json:"status"`
	StartTimeMillis     int64           `json:"startTimeMillis"`
	DurationMillis      int64           `json:"durationMillis"`
	PauseDurationMillis int64           `json:"pauseDurationMillis,omitempty"`
	StageFlowNodes      []StageFlowNode `json:"stageFlowNodes"`
	Error               *StageError     `json:"error,omitempty"`
}

// StageFlowNode represents an individual step within a stage.
type StageFlowNode struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Status               string   `json:"status"`
	StartTimeMillis      int64    `json:"startTimeMillis"`
	DurationMillis       int64    `json:"durationMillis"`
	PauseDurationMillis  int64    `json:"pauseDurationMillis,omitempty"`
	ParameterDescription string   `json:"parameterDescription,omitempty"`
	ParentNodes          []string `json:"parentNodes"`
	Error                *StageError `json:"error,omitempty"`
}

// StageError represents error information for a failed stage or step.
type StageError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// GetPipelineRun retrieves the complete pipeline execution data including all stages and steps.
// This uses the Jenkins Pipeline Stage View Plugin (wfapi) endpoint.
func (c *Client) GetPipelineRun(ctx context.Context, jobName string, buildNumber int) (*PipelineRun, error) {
	path := fmt.Sprintf("/job/%s/%d/wfapi/describe", encodeJobPath(jobName), buildNumber)

	var pipelineRun PipelineRun
	if err := c.get(ctx, path, &pipelineRun); err != nil {
		return nil, fmt.Errorf("getting pipeline run for build %d of %q: %w", buildNumber, jobName, err)
	}
	return &pipelineRun, nil
}

// StageLogResponse represents the log data returned from the wfapi log endpoint.
type StageLogResponse struct {
	NodeID     string `json:"nodeId"`
	NodeStatus string `json:"nodeStatus"`
	Length     int64  `json:"length"`
	HasMore    bool   `json:"hasMore"`
	Text       string `json:"text"`
}

// GetStageLog retrieves the log output for a specific stage.
// It fetches logs from all flow nodes within the stage and combines them.
func (c *Client) GetStageLog(ctx context.Context, jobName string, buildNumber int, stage *Stage) (string, error) {
	var combinedLogs string

	// If the stage has flow nodes, get logs for each one
	if len(stage.StageFlowNodes) > 0 {
		for _, flowNode := range stage.StageFlowNodes {
			path := fmt.Sprintf("/job/%s/%d/execution/node/%s/wfapi/log", encodeJobPath(jobName), buildNumber, flowNode.ID)

			var logResp StageLogResponse
			if err := c.get(ctx, path, &logResp); err != nil {
				// If we can't get logs for a specific node, continue with others
				continue
			}

			if logResp.Text != "" {
				if combinedLogs != "" {
					combinedLogs += "\n"
				}
				combinedLogs += logResp.Text
			}
		}
	} else {
		// If no flow nodes, try to get logs using the stage ID directly
		path := fmt.Sprintf("/job/%s/%d/execution/node/%s/wfapi/log", encodeJobPath(jobName), buildNumber, stage.ID)

		var logResp StageLogResponse
		if err := c.get(ctx, path, &logResp); err != nil {
			return "", fmt.Errorf("getting stage log: %w", err)
		}
		combinedLogs = logResp.Text
	}

	if combinedLogs == "" {
		return "", fmt.Errorf("no logs found for stage %q", stage.Name)
	}

	return combinedLogs, nil
}
