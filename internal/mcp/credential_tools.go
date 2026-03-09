package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
)

func registerCredentialTools(registry *ToolRegistry) error {
	tools := []struct {
		tool    Tool
		handler ToolHandler
	}{
		{newCredentialListTool(), credentialListHandler},
		{newCredentialViewTool(), credentialViewHandler},
	}
	for _, t := range tools {
		if err := registry.Register(t.tool, t.handler); err != nil {
			return fmt.Errorf("failed to register %s: %w", t.tool.Name, err)
		}
	}
	return nil
}

func newCredentialListTool() Tool {
	return Tool{
		Name:        "credential_list",
		Title:       "List Credentials",
		Description: "List credentials stored in the Jenkins credential store (read-only, no secrets exposed)",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"domain": NewStringProperty("Optional credential domain (default: global '_')"),
		}, nil),
	}
}

func newCredentialViewTool() Tool {
	return Tool{
		Name:        "credential_view",
		Title:       "View Credential",
		Description: "View metadata about a specific credential (no secrets exposed)",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"id":     NewStringProperty("Credential ID"),
			"domain": NewStringProperty("Optional credential domain (default: global '_')"),
		}, []string{"id"}),
	}
}

func credentialListHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	domain, _ := args["domain"].(string)
	if domain == "" {
		domain = "_"
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	creds, err := client.ListCredentials(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to list credentials: %w", err)
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credentials: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}

func credentialViewHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	id, ok := args["id"].(string)
	if !ok || id == "" {
		return nil, fmt.Errorf("missing required parameter: id")
	}

	domain, _ := args["domain"].(string)
	if domain == "" {
		domain = "_"
	}

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	cred, err := client.GetCredential(ctx, id, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	data, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal credential: %w", err)
	}

	return []Content{NewTextContent(string(data))}, nil
}
