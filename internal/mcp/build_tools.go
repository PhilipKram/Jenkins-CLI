package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
)

const maxLogSize = 1 << 20 // 1 MiB

func registerBuildTools(registry *ToolRegistry) error {
	tools := []struct {
		tool    Tool
		handler ToolHandler
	}{
		{newBuildListTool(), buildListHandler},
		{newBuildViewTool(), buildViewHandler},
		{newBuildLogTool(), buildLogHandler},
		{newBuildLastTool(), buildLastHandler},
		{newBuildStopTool(), buildStopHandler},
		{newBuildArtifactsTool(), buildArtifactsHandler},
	}
	for _, t := range tools {
		if err := registry.Register(t.tool, t.handler); err != nil {
			return fmt.Errorf("failed to register %s: %w", t.tool.Name, err)
		}
	}
	return nil
}

func newBuildListTool() Tool {
	return Tool{
		Name:        "build_list",
		Title:       "List Builds",
		Description: "List builds for a Jenkins job with optional limit",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"job":   NewStringProperty("Job name or path"),
			"limit": NewNumberProperty("Maximum number of builds to return (default: 10)"),
		}, []string{"job"}),
	}
}

func newBuildViewTool() Tool {
	return Tool{
		Name:        "build_view",
		Title:       "View Build Details",
		Description: "View detailed information about a specific build including artifacts, changeset, and actions",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"job":    NewStringProperty("Job name or path"),
			"number": NewNumberProperty("Build number"),
		}, []string{"job", "number"}),
	}
}

func newBuildLogTool() Tool {
	return Tool{
		Name:        "build_log",
		Title:       "Get Build Log",
		Description: "Get the console output of a build (capped at 1 MiB)",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"job":    NewStringProperty("Job name or path"),
			"number": NewNumberProperty("Build number"),
		}, []string{"job", "number"}),
	}
}

func newBuildLastTool() Tool {
	return Tool{
		Name:        "build_last",
		Title:       "Get Last Build",
		Description: "Get information about the last build of a job",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"job": NewStringProperty("Job name or path"),
		}, []string{"job"}),
	}
}

func newBuildStopTool() Tool {
	return Tool{
		Name:        "build_stop",
		Title:       "Stop Build",
		Description: "Stop a running build",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"job":    NewStringProperty("Job name or path"),
			"number": NewNumberProperty("Build number to stop"),
		}, []string{"job", "number"}),
	}
}

func newBuildArtifactsTool() Tool {
	return Tool{
		Name:        "build_artifacts",
		Title:       "List Build Artifacts",
		Description: "List artifacts produced by a build",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"job":    NewStringProperty("Job name or path"),
			"number": NewNumberProperty("Build number"),
		}, []string{"job", "number"}),
	}
}

func buildListHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	job, ok := args["job"].(string)
	if !ok || job == "" {
		return nil, fmt.Errorf("missing required parameter: job")
	}

	limit := 10
	if l, ok := args["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	builds, err := client.ListBuilds(ctx, job, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list builds: %w", err)
	}

	data, err := json.MarshalIndent(builds, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal builds: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func buildViewHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
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

	build, err := client.GetBuild(ctx, job, int(number))
	if err != nil {
		return nil, fmt.Errorf("failed to get build: %w", err)
	}

	data, err := json.MarshalIndent(build, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal build: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func buildLogHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	job, ok := args["job"].(string)
	if !ok || job == "" {
		return nil, fmt.Errorf("missing required parameter: job")
	}

	number, ok := args["number"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing required parameter: number")
	}

	client, err := clientutil.NewClient(60*time.Second, 3)
	if err != nil {
		return nil, err
	}

	rc, err := client.GetBuildLog(ctx, job, int(number))
	if err != nil {
		return nil, fmt.Errorf("failed to get build log: %w", err)
	}
	defer rc.Close()

	logData, err := io.ReadAll(io.LimitReader(rc, maxLogSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read build log: %w", err)
	}

	text := string(logData)
	if len(logData) == maxLogSize {
		text += "\n\n[Output truncated at 1 MiB]"
	}

	return []Content{NewTextContent(text)}, nil
}

func buildLastHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	job, ok := args["job"].(string)
	if !ok || job == "" {
		return nil, fmt.Errorf("missing required parameter: job")
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	build, err := client.GetLastBuild(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("failed to get last build: %w", err)
	}

	data, err := json.MarshalIndent(build, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal build: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func buildStopHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
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

	if err := client.StopBuild(ctx, job, int(number)); err != nil {
		return nil, fmt.Errorf("failed to stop build: %w", err)
	}

	result := map[string]interface{}{
		"status":  "stopped",
		"job":     job,
		"number":  int(number),
		"message": fmt.Sprintf("Build #%d of '%s' has been stopped", int(number), job),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func buildArtifactsHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
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

	artifacts, err := client.GetBuildArtifacts(ctx, job, int(number))
	if err != nil {
		return nil, fmt.Errorf("failed to get build artifacts: %w", err)
	}

	data, err := json.MarshalIndent(artifacts, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal artifacts: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}
