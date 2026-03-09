package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
)

func registerJobTools(registry *ToolRegistry) error {
	tools := []struct {
		tool    Tool
		handler ToolHandler
	}{
		{newJobListTool(), jobListHandler},
		{newJobViewTool(), jobViewHandler},
		{newJobBuildTool(), jobBuildHandler},
		{newJobEnableTool(), jobEnableHandler},
		{newJobDisableTool(), jobDisableHandler},
	}
	for _, t := range tools {
		if err := registry.Register(t.tool, t.handler); err != nil {
			return fmt.Errorf("failed to register %s: %w", t.tool.Name, err)
		}
	}
	return nil
}

func newJobListTool() Tool {
	return Tool{
		Name:        "job_list",
		Title:       "List Jobs",
		Description: "List Jenkins jobs, optionally filtered by folder path",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"folder": NewStringProperty("Optional folder path to list jobs from (e.g. 'my-folder' or 'parent/child')"),
		}, nil),
	}
}

func newJobViewTool() Tool {
	return Tool{
		Name:        "job_view",
		Title:       "View Job Details",
		Description: "View detailed information about a Jenkins job including health, last build, and queue status",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"name": NewStringProperty("Job name or path (e.g. 'my-job' or 'folder/my-job')"),
		}, []string{"name"}),
	}
}

func newJobBuildTool() Tool {
	return Tool{
		Name:        "job_build",
		Title:       "Trigger Build",
		Description: "Trigger a new build of a Jenkins job with optional parameters",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"name":       NewStringProperty("Job name or path (e.g. 'my-job' or 'folder/my-job')"),
			"parameters": NewStringProperty("Optional build parameters as JSON object (e.g. '{\"BRANCH\":\"main\",\"DEPLOY\":\"true\"}')"),
		}, []string{"name"}),
	}
}

func newJobEnableTool() Tool {
	return Tool{
		Name:        "job_enable",
		Title:       "Enable Job",
		Description: "Enable a disabled Jenkins job",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"name": NewStringProperty("Job name or path"),
		}, []string{"name"}),
	}
}

func newJobDisableTool() Tool {
	return Tool{
		Name:        "job_disable",
		Title:       "Disable Job",
		Description: "Disable a Jenkins job to prevent new builds",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"name": NewStringProperty("Job name or path"),
		}, []string{"name"}),
	}
}

func jobListHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	folder, _ := args["folder"].(string)

	jobs, err := client.ListJobs(ctx, folder)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal jobs: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func jobViewHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("missing required parameter: name")
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	job, err := client.GetJob(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func jobBuildHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("missing required parameter: name")
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	var params map[string]string
	if paramsStr, ok := args["parameters"].(string); ok && paramsStr != "" {
		if err := json.Unmarshal([]byte(paramsStr), &params); err != nil {
			return nil, fmt.Errorf("invalid parameters JSON: %w", err)
		}
	}

	if err := client.BuildJob(ctx, name, params); err != nil {
		return nil, fmt.Errorf("failed to trigger build: %w", err)
	}

	result := map[string]string{
		"status":  "triggered",
		"job":     name,
		"message": fmt.Sprintf("Build triggered for job '%s'", name),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func jobEnableHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("missing required parameter: name")
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	if err := client.EnableJob(ctx, name); err != nil {
		return nil, fmt.Errorf("failed to enable job: %w", err)
	}

	result := map[string]string{
		"status":  "enabled",
		"job":     name,
		"message": fmt.Sprintf("Job '%s' has been enabled", name),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func jobDisableHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("missing required parameter: name")
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	if err := client.DisableJob(ctx, name); err != nil {
		return nil, fmt.Errorf("failed to disable job: %w", err)
	}

	result := map[string]string{
		"status":  "disabled",
		"job":     name,
		"message": fmt.Sprintf("Job '%s' has been disabled", name),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}
