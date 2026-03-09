package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
)

func registerQueueTools(registry *ToolRegistry) error {
	tools := []struct {
		tool    Tool
		handler ToolHandler
	}{
		{newQueueListTool(), queueListHandler},
		{newQueueCancelTool(), queueCancelHandler},
	}
	for _, t := range tools {
		if err := registry.Register(t.tool, t.handler); err != nil {
			return fmt.Errorf("failed to register %s: %w", t.tool.Name, err)
		}
	}
	return nil
}

func newQueueListTool() Tool {
	return Tool{
		Name:        "queue_list",
		Title:       "List Queue",
		Description: "List items currently in the Jenkins build queue",
		InputSchema: NewJSONSchema("object", map[string]interface{}{}, nil),
	}
}

func newQueueCancelTool() Tool {
	return Tool{
		Name:        "queue_cancel",
		Title:       "Cancel Queue Item",
		Description: "Cancel a queued build by its queue item ID",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"id": NewNumberProperty("Queue item ID to cancel"),
		}, []string{"id"}),
	}
}

func queueListHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	items, err := client.GetQueue(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list queue: %w", err)
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal queue: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func queueCancelHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	id, ok := args["id"].(float64)
	if !ok {
		return nil, fmt.Errorf("missing required parameter: id")
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	if err := client.CancelQueueItem(ctx, int(id)); err != nil {
		return nil, fmt.Errorf("failed to cancel queue item: %w", err)
	}

	result := map[string]interface{}{
		"status":  "cancelled",
		"id":      int(id),
		"message": fmt.Sprintf("Queue item #%d has been cancelled", int(id)),
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}
