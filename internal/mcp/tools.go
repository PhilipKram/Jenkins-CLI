package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// ToolHandler is a function that executes a tool with the given arguments.
type ToolHandler func(ctx context.Context, args map[string]interface{}) ([]Content, error)

// RegisteredTool combines a tool definition with its handler function.
type RegisteredTool struct {
	Tool    Tool
	Handler ToolHandler
}

// ToolRegistry manages the collection of available MCP tools.
type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]*RegisteredTool
}

// NewToolRegistry creates a new empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]*RegisteredTool),
	}
}

// Register adds a tool to the registry with its handler.
func (r *ToolRegistry) Register(tool Tool, handler ToolHandler) error {
	if tool.Name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if handler == nil {
		return fmt.Errorf("tool handler cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[tool.Name] = &RegisteredTool{
		Tool:    tool,
		Handler: handler,
	}

	return nil
}

// Get retrieves a registered tool by name.
func (r *ToolRegistry) Get(name string) *RegisteredTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// List returns all registered tools.
func (r *ToolRegistry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]Tool, 0, len(r.tools))
	for _, rt := range r.tools {
		tools = append(tools, rt.Tool)
	}
	return tools
}

// Execute runs a tool by name with the given arguments.
func (r *ToolRegistry) Execute(ctx context.Context, name string, args map[string]interface{}) ToolCallResult {
	rt := r.Get(name)
	if rt == nil {
		return ToolCallResult{
			Content: []Content{
				NewTextContent(fmt.Sprintf("Tool not found: %s", name)),
			},
			IsError: true,
		}
	}

	content, err := rt.Handler(ctx, args)
	if err != nil {
		return ToolCallResult{
			Content: []Content{
				NewTextContent(fmt.Sprintf("Tool execution failed: %v", err)),
			},
			IsError: true,
		}
	}

	return ToolCallResult{
		Content: content,
		IsError: false,
	}
}

// SetRegistry sets the tool registry for the server.
func (s *Server) SetRegistry(registry *ToolRegistry) {
	if registry == nil {
		return
	}
	s.handlersMu.Lock()
	defer s.handlersMu.Unlock()

	s.handlers["tools/list"] = func(req *Request) (map[string]interface{}, error) {
		tools := registry.List()
		result := ToolsListResult{Tools: tools}

		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tools list: %w", err)
		}

		var resultMap map[string]interface{}
		if err := json.Unmarshal(resultBytes, &resultMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal result map: %w", err)
		}

		return resultMap, nil
	}

	s.handlers["tools/call"] = func(req *Request) (map[string]interface{}, error) {
		var callReq ToolCallRequest
		if req.Params != nil {
			paramsBytes, err := json.Marshal(req.Params)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal params: %w", err)
			}
			if err := json.Unmarshal(paramsBytes, &callReq); err != nil {
				return nil, fmt.Errorf("failed to unmarshal tool call request: %w", err)
			}
		}

		result := registry.Execute(s.ctx, callReq.Name, callReq.Arguments)

		resultBytes, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal tool call result: %w", err)
		}

		var resultMap map[string]interface{}
		if err := json.Unmarshal(resultBytes, &resultMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal result map: %w", err)
		}

		return resultMap, nil
	}
}

// Schema helpers

// NewJSONSchema creates a JSON schema map for tool input parameters.
func NewJSONSchema(schemaType string, properties map[string]interface{}, required []string) map[string]interface{} {
	schema := map[string]interface{}{
		"type":       schemaType,
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// NewStringProperty creates a JSON schema property for a string parameter.
func NewStringProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "string",
		"description": description,
	}
}

// NewNumberProperty creates a JSON schema property for a number parameter.
func NewNumberProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "number",
		"description": description,
	}
}

// NewBooleanProperty creates a JSON schema property for a boolean parameter.
func NewBooleanProperty(description string) map[string]interface{} {
	return map[string]interface{}{
		"type":        "boolean",
		"description": description,
	}
}

// NewObjectProperty creates a JSON schema property for an object parameter.
func NewObjectProperty(description string, properties map[string]interface{}, required []string) map[string]interface{} {
	prop := map[string]interface{}{
		"type":        "object",
		"description": description,
		"properties":  properties,
	}
	if len(required) > 0 {
		prop["required"] = required
	}
	return prop
}

// RegisterDefaultTools registers all Jenkins MCP tools with the given registry.
func RegisterDefaultTools(registry *ToolRegistry) error {
	// Job Tools
	if err := registerJobTools(registry); err != nil {
		return err
	}
	// Build Tools
	if err := registerBuildTools(registry); err != nil {
		return err
	}
	// Pipeline Tools
	if err := registerPipelineTools(registry); err != nil {
		return err
	}
	// Node Tools
	if err := registerNodeTools(registry); err != nil {
		return err
	}
	// Queue Tools
	if err := registerQueueTools(registry); err != nil {
		return err
	}
	// Credential Tools
	if err := registerCredentialTools(registry); err != nil {
		return err
	}
	// View Tools
	if err := registerViewTools(registry); err != nil {
		return err
	}
	// System Tools
	if err := registerSystemTools(registry); err != nil {
		return err
	}
	// Multibranch Tools
	if err := registerMultibranchTools(registry); err != nil {
		return err
	}
	return nil
}
