package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
	"github.com/PhilipKram/jenkins-cli/internal/config"
)

func registerConfigTools(registry *ToolRegistry) error {
	if err := registry.Register(newConfigStatusTool(), configStatusHandler); err != nil {
		return err
	}
	if err := registry.Register(newConfigSetTool(), configSetHandler); err != nil {
		return err
	}
	if err := registry.Register(newProfileListTool(), profileListHandler); err != nil {
		return err
	}
	if err := registry.Register(newProfileSwitchTool(), profileSwitchHandler); err != nil {
		return err
	}
	if err := registry.Register(newProfileDeleteTool(), profileDeleteHandler); err != nil {
		return err
	}
	return nil
}

// -- config_status: check configuration and connectivity --

func newConfigStatusTool() Tool {
	return Tool{
		Name:  "config_status",
		Title: "Configuration Status",
		Description: "Check Jenkins CLI configuration status and connectivity. " +
			"Returns the current profile, configured URL, auth type, and whether the connection is working. " +
			"Call this first if any Jenkins operation fails, to diagnose configuration issues.",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"profile": NewStringProperty("Profile name to check (optional, uses active profile if not specified)."),
		}, nil),
	}
}

func configStatusHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	// Temporarily override profile if requested
	if profile, ok := args["profile"].(string); ok && profile != "" {
		orig := config.ActiveProfile
		config.ActiveProfile = profile
		defer func() { config.ActiveProfile = orig }()
	}

	result := map[string]interface{}{}

	// Get current profile name
	profileName, err := config.CurrentProfileName()
	if err != nil {
		result["configured"] = false
		result["error"] = err.Error()
		result["setup_instructions"] = setupInstructions()
		return marshalResult(result)
	}
	result["profile"] = profileName

	// Load config
	cfg, err := config.Load()
	if err != nil {
		result["configured"] = false
		result["error"] = err.Error()
		result["setup_instructions"] = setupInstructions()
		return marshalResult(result)
	}

	result["configured"] = true
	result["url"] = cfg.URL
	result["auth_type"] = string(cfg.EffectiveAuthType())
	if cfg.User != "" {
		result["user"] = cfg.User
	}

	// Test connectivity
	client, err := clientutil.NewClient(10*time.Second, 1)
	if err != nil {
		result["connected"] = false
		result["connection_error"] = err.Error()
		return marshalResult(result)
	}

	ver, err := client.GetVersion(ctx)
	if err != nil {
		result["connected"] = false
		result["connection_error"] = err.Error()
	} else {
		result["connected"] = true
		result["jenkins_version"] = ver
	}

	return marshalResult(result)
}

// -- config_set: configure a profile --

func newConfigSetTool() Tool {
	return Tool{
		Name:  "config_set",
		Title: "Configure Profile",
		Description: "Create or update a Jenkins CLI profile with connection details. " +
			"Supports basic auth (user+token), bearer token, and OAuth. " +
			"Use this to set up Jenkins connectivity for the first time or to add additional server profiles.",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"profile":      NewStringProperty("Profile name (default: 'default'). Use different names to manage multiple Jenkins servers."),
			"url":          NewStringProperty("Jenkins server URL (e.g., https://jenkins.example.com)."),
			"user":         NewStringProperty("Jenkins username (for basic auth)."),
			"token":        NewStringProperty("Jenkins API token (for basic auth)."),
			"bearer_token": NewStringProperty("Bearer token (alternative to basic auth)."),
			"insecure":     NewBooleanProperty("Skip TLS certificate verification."),
		}, []string{"url"}),
	}
}

func configSetHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	profileName := "default"
	if p, ok := args["profile"].(string); ok && p != "" {
		profileName = p
	}

	url, _ := args["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}

	cfg := config.Config{
		URL: url,
	}

	if user, ok := args["user"].(string); ok && user != "" {
		cfg.User = user
		cfg.AuthType = config.AuthBasic
	}
	if token, ok := args["token"].(string); ok && token != "" {
		cfg.Token = token
	}
	if bearer, ok := args["bearer_token"].(string); ok && bearer != "" {
		cfg.BearerToken = bearer
		cfg.AuthType = config.AuthBearer
	}
	if insecure, ok := args["insecure"].(bool); ok {
		cfg.Insecure = insecure
	}

	// Set the active profile so Save() writes to the right profile
	orig := config.ActiveProfile
	config.ActiveProfile = profileName
	defer func() { config.ActiveProfile = orig }()

	if err := config.Save(&cfg); err != nil {
		return nil, fmt.Errorf("failed to save configuration: %w", err)
	}

	result := map[string]interface{}{
		"status":    "saved",
		"profile":   profileName,
		"url":       cfg.URL,
		"auth_type": string(cfg.EffectiveAuthType()),
	}

	// Test the new configuration
	client, err := clientutil.NewClient(10*time.Second, 1)
	if err != nil {
		result["connected"] = false
		result["connection_error"] = err.Error()
		return marshalResult(result)
	}

	ver, err := client.GetVersion(ctx)
	if err != nil {
		result["connected"] = false
		result["connection_error"] = err.Error()
	} else {
		result["connected"] = true
		result["jenkins_version"] = ver
	}

	return marshalResult(result)
}

// -- profile_list: list all profiles --

func newProfileListTool() Tool {
	return Tool{
		Name:        "profile_list",
		Title:       "List Profiles",
		Description: "List all configured Jenkins CLI profiles with their URLs and auth types.",
		InputSchema: NewJSONSchema("object", map[string]interface{}{}, nil),
	}
}

func profileListHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	mc, err := config.LoadMulti()
	if err != nil {
		result := map[string]interface{}{
			"profiles":           []interface{}{},
			"setup_instructions": setupInstructions(),
		}
		return marshalResult(result)
	}

	currentProfile, _ := config.CurrentProfileName()
	profiles := make([]map[string]interface{}, 0, len(mc.Profiles))

	for _, name := range config.ProfileNames(mc) {
		cfg := mc.Profiles[name]
		p := map[string]interface{}{
			"name":      name,
			"url":       cfg.URL,
			"auth_type": string(cfg.EffectiveAuthType()),
			"active":    name == currentProfile,
		}
		if cfg.User != "" {
			p["user"] = cfg.User
		}
		profiles = append(profiles, p)
	}

	result := map[string]interface{}{
		"current_profile": currentProfile,
		"profiles":        profiles,
	}
	return marshalResult(result)
}

// -- profile_switch: switch active profile --

func newProfileSwitchTool() Tool {
	return Tool{
		Name:        "profile_switch",
		Title:       "Switch Profile",
		Description: "Switch the active Jenkins CLI profile. Use this to change which Jenkins server subsequent commands target.",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"profile": NewStringProperty("Profile name to switch to."),
		}, []string{"profile"}),
	}
}

func profileSwitchHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	profileName, _ := args["profile"].(string)
	if profileName == "" {
		return nil, fmt.Errorf("profile name is required")
	}

	mc, err := config.LoadMulti()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if _, ok := mc.Profiles[profileName]; !ok {
		available := config.ProfileNames(mc)
		return nil, fmt.Errorf("profile %q not found. Available: %v", profileName, available)
	}

	mc.CurrentProfile = profileName
	if err := config.SaveMulti(mc); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	result := map[string]interface{}{
		"status":  "switched",
		"profile": profileName,
		"url":     mc.Profiles[profileName].URL,
	}
	return marshalResult(result)
}

// -- profile_delete: delete a profile --

func newProfileDeleteTool() Tool {
	return Tool{
		Name:        "profile_delete",
		Title:       "Delete Profile",
		Description: "Delete a Jenkins CLI profile. Cannot delete the last remaining profile.",
		InputSchema: NewJSONSchema("object", map[string]interface{}{
			"profile": NewStringProperty("Profile name to delete."),
		}, []string{"profile"}),
	}
}

func profileDeleteHandler(ctx context.Context, args map[string]interface{}) ([]Content, error) {
	profileName, _ := args["profile"].(string)
	if profileName == "" {
		return nil, fmt.Errorf("profile name is required")
	}

	mc, err := config.LoadMulti()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if _, ok := mc.Profiles[profileName]; !ok {
		available := config.ProfileNames(mc)
		return nil, fmt.Errorf("profile %q not found. Available: %v", profileName, available)
	}

	if len(mc.Profiles) <= 1 {
		return nil, fmt.Errorf("cannot delete the last profile")
	}

	delete(mc.Profiles, profileName)

	// If we deleted the current profile, switch to the first available
	if mc.CurrentProfile == profileName {
		names := config.ProfileNames(mc)
		mc.CurrentProfile = names[0]
	}

	if err := config.SaveMulti(mc); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	result := map[string]interface{}{
		"status":          "deleted",
		"deleted_profile": profileName,
		"current_profile": mc.CurrentProfile,
	}
	return marshalResult(result)
}

// -- helpers --

func setupInstructions() map[string]interface{} {
	return map[string]interface{}{
		"message": "Jenkins CLI is not configured. Use the config_set tool to configure a connection, or set environment variables.",
		"option_1_tool": map[string]interface{}{
			"tool": "config_set",
			"args": map[string]string{
				"url":   "https://jenkins.example.com",
				"user":  "your-username",
				"token": "your-api-token",
			},
		},
		"option_2_env_vars": map[string]string{
			"JENKINS_URL":   "https://jenkins.example.com",
			"JENKINS_USER":  "your-username",
			"JENKINS_TOKEN": "your-api-token",
		},
		"option_3_cli": "jenkins-cli configure",
	}
}

func marshalResult(result map[string]interface{}) ([]Content, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return []Content{NewTextContent(string(data))}, nil
}
