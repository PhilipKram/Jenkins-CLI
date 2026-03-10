package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
)

func registerMultibranchTools(registry *ToolRegistry) error {
	tools := []struct {
		tool    Tool
		handler ToolHandler
	}{
		{newMultibranchBranchesTool(), multibranchBranchesHandler},
		{newMultibranchScanTool(), multibranchScanHandler},
		{newMultibranchScanLogTool(), multibranchScanLogHandler},
	}
	for _, t := range tools {
		if err := registry.Register(t.tool, t.handler); err != nil {
			return fmt.Errorf("failed to register %s: %w", t.tool.Name, err)
		}
	}
	return nil
}

func newMultibranchBranchesTool() Tool {
	return Tool{
		Name:        "multibranch_branches",
		Title:       "List Multibranch Branches",
		Description: "List branches in a multibranch pipeline project",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"name": NewStringProperty("Multibranch pipeline name or path (e.g. 'my-pipeline' or 'folder/my-pipeline')"),
		}, []string{"name"}),
	}
}

func newMultibranchScanTool() Tool {
	return Tool{
		Name:        "multibranch_scan",
		Title:       "Scan Multibranch Pipeline",
		Description: "Trigger a branch indexing scan for a multibranch pipeline to discover new branches",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"name": NewStringProperty("Multibranch pipeline name or path (e.g. 'my-pipeline' or 'folder/my-pipeline')"),
		}, []string{"name"}),
	}
}

func newMultibranchScanLogTool() Tool {
	return Tool{
		Name:        "multibranch_scan_log",
		Title:       "View Scan Log",
		Description: "Retrieve the branch indexing log for a multibranch pipeline",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"name": NewStringProperty("Multibranch pipeline name or path (e.g. 'my-pipeline' or 'folder/my-pipeline')"),
		}, []string{"name"}),
	}
}

func multibranchBranchesHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("missing required parameter: name")
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	branches, err := client.ListJobs(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	data, err := json.MarshalIndent(branches, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal branches: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func multibranchScanHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("missing required parameter: name")
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	if err := client.ScanMultibranchPipeline(ctx, name); err != nil {
		return nil, fmt.Errorf("failed to trigger scan: %w", err)
	}

	result := map[string]string{
		"status":  "triggered",
		"pipeline": name,
		"message": fmt.Sprintf("Branch scan triggered for '%s'", name),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func multibranchScanLogHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("missing required parameter: name")
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	log, err := client.GetScanLog(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get scan log: %w", err)
	}

	return []Content{NewTextContent(log)}, nil
}
