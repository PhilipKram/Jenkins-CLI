package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
)

func registerNodeTools(registry *ToolRegistry) error {
	tools := []struct {
		tool    Tool
		handler ToolHandler
	}{
		{newNodeListTool(), nodeListHandler},
		{newNodeViewTool(), nodeViewHandler},
	}
	for _, t := range tools {
		if err := registry.Register(t.tool, t.handler); err != nil {
			return fmt.Errorf("failed to register %s: %w", t.tool.Name, err)
		}
	}
	return nil
}

func newNodeListTool() Tool {
	return Tool{
		Name:        "node_list",
		Title:       "List Nodes",
		Description: "List Jenkins nodes/agents with their status and executor information",
		InputSchema: NewJSONSchema("object", map[string]interface{}{}, nil),
	}
}

func newNodeViewTool() Tool {
	return Tool{
		Name:        "node_view",
		Title:       "View Node Details",
		Description: "View detailed information about a specific Jenkins node/agent",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"name": NewStringProperty("Node name (use 'master' or 'built-in' for the controller node)"),
		}, []string{"name"}),
	}
}

func nodeListHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	nodes, err := client.ListNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	data, err := json.MarshalIndent(nodes, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal nodes: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func nodeViewHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return nil, fmt.Errorf("missing required parameter: name")
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	node, err := client.GetNode(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get node: %w", err)
	}

	data, err := json.MarshalIndent(node, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal node: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}
