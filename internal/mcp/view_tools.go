package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
)

func registerViewTools(registry *ToolRegistry) error {
	return registry.Register(newViewListTool(), viewListHandler)
}

func newViewListTool() Tool {
	return Tool{
		Name:        "view_list",
		Title:       "List Views",
		Description: "List Jenkins views and their associated jobs",
		InputSchema: NewJSONSchema("object", map[string]interface{}{}, nil),
	}
}

func viewListHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	views, err := client.ListViews(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list views: %w", err)
	}

	data, err := json.MarshalIndent(views, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal views: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}
