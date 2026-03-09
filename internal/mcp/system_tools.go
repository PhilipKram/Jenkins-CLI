package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
)

func registerSystemTools(registry *ToolRegistry) error {
	return registry.Register(newSystemInfoTool(), systemInfoHandler)
}

func newSystemInfoTool() Tool {
	return Tool{
		Name:        "system_info",
		Title:       "System Info",
		Description: "Get Jenkins server information including version, mode, and current user details",
		InputSchema: NewJSONSchema("object", map[string]interface{}{}, nil),
	}
}

func systemInfoHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	sysInfo, err := client.GetSystemInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get system info: %w", err)
	}

	whoAmI, err := client.WhoAmI(ctx)
	if err != nil {
		// Non-fatal: include system info even if whoami fails
		whoAmI = nil
	}

	ver, err := client.GetVersion(ctx)
	if err != nil {
		ver = ""
	}

	result := map[string]interface{}{
		"system": sysInfo,
	}
	if whoAmI != nil {
		result["user"] = whoAmI
	}
	if ver != "" {
		result["version"] = ver
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal system info: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}
