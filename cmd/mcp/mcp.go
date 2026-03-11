package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/PhilipKram/jenkins-cli/internal/mcp"
	"github.com/PhilipKram/jenkins-cli/internal/version"
)

// NewCmdMCP returns the top-level mcp command.
func NewCmdMCP() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Model Context Protocol server",
		Long:  "Start, install, or manage the Jenkins CLI MCP server for AI-powered development tools.",
	}

	cmd.AddCommand(newCmdServe())
	cmd.AddCommand(newCmdInstall())
	cmd.AddCommand(newCmdUninstall())
	cmd.AddCommand(newCmdStatus())

	return cmd
}

func newCmdServe() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start MCP server on stdio",
		Long: `Start an MCP (Model Context Protocol) server on stdio.

The server exposes Jenkins CLI capabilities as MCP tools that can be
invoked by AI agents and LLM-powered development tools.

The server communicates via JSON-RPC 2.0 over stdin/stdout.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			server := mcp.NewServer(
				"jenkins-cli-mcp",
				version.Version,
				"Jenkins CLI MCP server - exposes jenkins-cli commands as MCP tools",
			)

			registry := mcp.NewToolRegistry()
			if err := mcp.RegisterDefaultTools(registry); err != nil {
				return err
			}
			server.SetRegistry(registry)

			mcp.RegisterDefaultResources(server)
			mcp.RegisterDefaultPrompts(server)

			return server.Start()
		},
	}
}

func newCmdInstall() *cobra.Command {
	var scope string
	var client string
	var profileName string
	var serverName string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Register jenkins-cli as an MCP server in an AI client",
		Long: `Register jenkins-cli as an MCP server in an AI client configuration.

Supported clients:
  claude-code     - Claude Code CLI (default)
  claude-desktop  - Claude Desktop application

Supported scopes (claude-code only):
  user    - User-level configuration (default)
  local   - Local project configuration
  project - Project-level configuration

To register multiple Jenkins servers, use --profile and --name:
  jenkins-cli mcp install --profile images --name jenkins-images
  jenkins-cli mcp install --profile helm --name jenkins-helm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall(client, scope, profileName, serverName)
		},
	}

	cmd.Flags().StringVar(&scope, "scope", "user", "Configuration scope: user, local, or project (claude-code only)")
	cmd.Flags().StringVar(&client, "client", "claude-code", "AI client: claude-code or claude-desktop")
	cmd.Flags().StringVar(&profileName, "profile", "", "Jenkins CLI profile to use for this MCP server")
	cmd.Flags().StringVar(&serverName, "name", "jenkins-cli", "MCP server name (useful when registering multiple profiles)")

	return cmd
}

func newCmdUninstall() *cobra.Command {
	var scope string
	var client string

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove jenkins-cli MCP server from an AI client",
		Long: `Remove jenkins-cli as an MCP server from an AI client configuration.

Supported clients:
  claude-code     - Claude Code CLI (default)
  claude-desktop  - Claude Desktop application`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstall(client, scope)
		},
	}

	cmd.Flags().StringVar(&scope, "scope", "user", "Configuration scope: user, local, or project (claude-code only)")
	cmd.Flags().StringVar(&client, "client", "claude-code", "AI client: claude-code or claude-desktop")

	return cmd
}

func newCmdStatus() *cobra.Command {
	var client string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check if jenkins-cli is registered as an MCP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(client)
		},
	}

	cmd.Flags().StringVar(&client, "client", "claude-code", "AI client: claude-code")

	return cmd
}

func binaryPath() string {
	exe, err := os.Executable()
	if err != nil {
		return "jenkins-cli"
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe
	}
	return resolved
}

func mcpConfigJSON(binPath string, profileName string) (string, error) {
	config := map[string]interface{}{
		"command": binPath,
		"args":    []string{"mcp", "serve"},
	}
	if profileName != "" {
		config["env"] = map[string]string{
			"JENKINS_PROFILE": profileName,
		}
	}
	data, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal MCP config: %w", err)
	}
	return string(data), nil
}

func claudeDesktopConfigPath() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable not set")
		}
		return filepath.Join(appData, "Claude", "claude_desktop_config.json"), nil
	case "linux":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"), nil
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func findClaude() (string, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude CLI not found in PATH: %w", err)
	}
	return path, nil
}

func runInstall(client, scope, profileName, serverName string) error {
	switch client {
	case "claude-code":
		return installClaudeCode(scope, profileName, serverName)
	case "claude-desktop":
		return installClaudeDesktop(profileName, serverName)
	default:
		return fmt.Errorf("unsupported client: %s (supported: claude-code, claude-desktop)", client)
	}
}

func installClaudeCode(scope, profileName, serverName string) error {
	claudePath, err := findClaude()
	if err != nil {
		return err
	}

	binPath := binaryPath()
	configJSON, err := mcpConfigJSON(binPath, profileName)
	if err != nil {
		return err
	}

	//nolint:gosec // Arguments are constructed from trusted sources
	cmd := exec.Command(claudePath, "mcp", "add-json", "--scope", scope, serverName, configJSON)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to register %s in Claude Code: %w", serverName, err)
	}

	msg := fmt.Sprintf("Successfully registered %s as an MCP server in Claude Code (scope: %s)", serverName, scope)
	if profileName != "" {
		msg += fmt.Sprintf(" using profile %q", profileName)
	}
	fmt.Fprintln(os.Stderr, msg)
	return nil
}

func installClaudeDesktop(profileName, serverName string) error {
	configPath, err := claudeDesktopConfigPath()
	if err != nil {
		return err
	}

	binPath := binaryPath()

	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		config = make(map[string]interface{})
	} else {
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		mcpServers = make(map[string]interface{})
	}

	serverCfg := map[string]interface{}{
		"command": binPath,
		"args":    []string{"mcp", "serve"},
	}
	if profileName != "" {
		serverCfg["env"] = map[string]string{
			"JENKINS_PROFILE": profileName,
		}
	}

	mcpServers[serverName] = serverCfg
	config["mcpServers"] = mcpServers

	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(configPath, output, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	msg := fmt.Sprintf("Successfully registered %s as an MCP server in Claude Desktop", serverName)
	if profileName != "" {
		msg += fmt.Sprintf(" using profile %q", profileName)
	}
	fmt.Fprintln(os.Stderr, msg)
	fmt.Fprintf(os.Stderr, "Config file: %s\n", configPath)
	return nil
}

func runUninstall(client, scope string) error {
	switch client {
	case "claude-code":
		return uninstallClaudeCode(scope)
	case "claude-desktop":
		return uninstallClaudeDesktop()
	default:
		return fmt.Errorf("unsupported client: %s (supported: claude-code, claude-desktop)", client)
	}
}

func uninstallClaudeCode(scope string) error {
	claudePath, err := findClaude()
	if err != nil {
		return err
	}

	//nolint:gosec // Arguments are constructed from trusted sources
	cmd := exec.Command(claudePath, "mcp", "remove", "jenkins-cli", "--scope", scope)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove jenkins-cli from Claude Code: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Successfully removed jenkins-cli MCP server from Claude Code (scope: %s)\n", scope)
	return nil
}

func uninstallClaudeDesktop() error {
	configPath, err := claudeDesktopConfigPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config file not found: %s", configPath)
		}
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	mcpServers, ok := config["mcpServers"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("jenkins-cli is not registered as an MCP server in Claude Desktop")
	}

	if _, exists := mcpServers["jenkins-cli"]; !exists {
		return fmt.Errorf("jenkins-cli is not registered as an MCP server in Claude Desktop")
	}

	delete(mcpServers, "jenkins-cli")
	config["mcpServers"] = mcpServers

	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, output, 0o644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Successfully removed jenkins-cli MCP server from Claude Desktop\n")
	fmt.Fprintf(os.Stderr, "Config file: %s\n", configPath)
	return nil
}

func runStatus(client string) error {
	if client != "claude-code" {
		return fmt.Errorf("status is currently only supported for claude-code")
	}

	claudePath, err := findClaude()
	if err != nil {
		return err
	}

	//nolint:gosec // Arguments are constructed from trusted sources
	cmd := exec.Command(claudePath, "mcp", "get", "jenkins-cli")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("jenkins-cli is not registered as an MCP server in Claude Code (or failed to check): %w", err)
	}

	return nil
}
