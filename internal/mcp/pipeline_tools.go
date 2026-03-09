package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
	"github.com/PhilipKram/jenkins-cli/internal/jenkins"
)

func registerPipelineTools(registry *ToolRegistry) error {
	tools := []struct {
		tool    Tool
		handler ToolHandler
	}{
		{newPipelineValidateTool(), pipelineValidateHandler},
		{newPipelineStagesTool(), pipelineStagesHandler},
		{newPipelineStageLogTool(), pipelineStageLogHandler},
	}
	for _, t := range tools {
		if err := registry.Register(t.tool, t.handler); err != nil {
			return fmt.Errorf("failed to register %s: %w", t.tool.Name, err)
		}
	}
	return nil
}

func newPipelineValidateTool() Tool {
	return Tool{
		Name:        "pipeline_validate",
		Title:       "Validate Jenkinsfile",
		Description: "Validate a Jenkinsfile (declarative pipeline) syntax against the Jenkins server",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"jenkinsfile": NewStringProperty("The Jenkinsfile content to validate"),
		}, []string{"jenkinsfile"}),
	}
}

func newPipelineStagesTool() Tool {
	return Tool{
		Name:        "pipeline_stages",
		Title:       "Get Pipeline Stages",
		Description: "Get pipeline stages and their status for a specific build",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"job":    NewStringProperty("Job name or path"),
			"number": NewNumberProperty("Build number"),
		}, []string{"job", "number"}),
	}
}

func newPipelineStageLogTool() Tool {
	return Tool{
		Name:        "pipeline_stage_log",
		Title:       "Get Pipeline Stage Log",
		Description: "Get the log output for a specific pipeline stage",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"job":    NewStringProperty("Job name or path"),
			"number": NewNumberProperty("Build number"),
			"stage":  NewStringProperty("Stage name"),
		}, []string{"job", "number", "stage"}),
	}
}

func pipelineValidateHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	jenkinsfile, ok := args["jenkinsfile"].(string)
	if !ok || jenkinsfile == "" {
		return nil, fmt.Errorf("missing required parameter: jenkinsfile")
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	result, err := client.ValidatePipeline(ctx, jenkinsfile)
	if err != nil {
		return nil, fmt.Errorf("failed to validate pipeline: %w", err)
	}

	output := map[string]string{
		"result": result,
	}
	if result == "Jenkinsfile successfully validated.\n" || result == "Jenkinsfile successfully validated." {
		output["valid"] = "true"
	} else {
		output["valid"] = "false"
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func pipelineStagesHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	job, ok := args["job"].(string)
	if !ok || job == "" {
		return nil, fmt.Errorf("missing required parameter: job")
	}

	number, ok := args["number"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing required parameter: number")
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	run, err := client.GetPipelineRun(ctx, job, int(number))
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline stages: %w", err)
	}

	data, err := json.MarshalIndent(run, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal pipeline run: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func pipelineStageLogHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	job, ok := args["job"].(string)
	if !ok || job == "" {
		return nil, fmt.Errorf("missing required parameter: job")
	}

	number, ok := args["number"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing required parameter: number")
	}

	stageName, ok := args["stage"].(string)
	if !ok || stageName == "" {
		return nil, fmt.Errorf("missing required parameter: stage")
	}

	client, err := clientutil.NewClient(60*time.Second, 3)
	if err != nil {
		return nil, err
	}

	// Get pipeline run to find the stage
	run, err := client.GetPipelineRun(ctx, job, int(number))
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline run: %w", err)
	}

	var targetStage *jenkins.Stage
	for i := range run.Stages {
		if run.Stages[i].Name == stageName {
			targetStage = &run.Stages[i]
			break
		}
	}

	if targetStage == nil {
		return nil, fmt.Errorf("stage '%s' not found in build #%d", stageName, int(number))
	}

	log, err := client.GetStageLog(ctx, job, int(number), targetStage)
	if err != nil {
		return nil, fmt.Errorf("failed to get stage log: %w", err)
	}

	return []Content{NewTextContent(log)}, nil
}
