package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// AuthType represents the authentication method used.
type AuthType string

const (
	AuthBasic  AuthType = "basic"
	AuthOAuth  AuthType = "oauth"
	AuthBearer AuthType = "bearer"
)

// DefaultProfileName is the name used for the default profile.
const DefaultProfileName = "default"

// ActiveProfile is set by the root command's --profile flag.
// If empty, the current_profile from the config file is used.
var ActiveProfile string

// OAuthConfig holds OAuth2 provider settings.
type OAuthConfig struct {
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret,omitempty"`
	AuthURL      string   `json:"auth_url"`
	TokenURL     string   `json:"token_url"`
	Scopes       []string `json:"scopes,omitempty"`
	AccessToken  string   `json:"access_token,omitempty"`
	RefreshToken string   `json:"refresh_token,omitempty"`
}

// Config represents the configuration for a single Jenkins server.
type Config struct {
	URL         string       `json:"url"`
	User        string       `json:"user,omitempty"`
	Token       string       `json:"token,omitempty"`
	Insecure    bool         `json:"insecure"`
	AuthType    AuthType     `json:"auth_type,omitempty"`
	BearerToken string       `json:"bearer_token,omitempty"`
	OAuth       *OAuthConfig `json:"oauth,omitempty"`
}

// MultiConfig is the top-level config file format supporting multiple profiles.
type MultiConfig struct {
	CurrentProfile string            `json:"current_profile"`
	Profiles       map[string]Config `json:"profiles"`
}

func (c *Config) EffectiveAuthType() AuthType {
	if c.AuthType != "" {
		return c.AuthType
	}
	// Infer from fields for backward compatibility
	if c.BearerToken != "" {
		return AuthBearer
	}
	if c.OAuth != nil && c.OAuth.AccessToken != "" {
		return AuthOAuth
	}
	return AuthBasic
}

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}
	return filepath.Join(home, ".jenkins-cli"), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// resolveProfileName returns the profile name to use, considering
// the ActiveProfile override (--profile flag), JENKINS_PROFILE env var,
// the config file's current_profile, and the default.
func resolveProfileName(mc *MultiConfig) string {
	if ActiveProfile != "" {
		return ActiveProfile
	}
	if v := os.Getenv("JENKINS_PROFILE"); v != "" {
		return v
	}
	if mc.CurrentProfile != "" {
		return mc.CurrentProfile
	}
	return DefaultProfileName
}

// loadMultiConfig reads and parses the config file.
// If the file uses the old flat format, it auto-migrates to multi-profile format.
func loadMultiConfig() (*MultiConfig, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not configured. Run 'jenkins-cli configure' first")
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	// Try parsing as multi-config first
	var mc MultiConfig
	if err := json.Unmarshal(data, &mc); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// If profiles map exists, it's the new format
	if mc.Profiles != nil {
		return &mc, nil
	}

	// Otherwise, it's the old flat format — migrate it
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	mc = MultiConfig{
		CurrentProfile: DefaultProfileName,
		Profiles: map[string]Config{
			DefaultProfileName: cfg,
		},
	}

	// Auto-save the migrated format
	if saveErr := saveMultiConfig(&mc); saveErr != nil {
		// Non-fatal: we can still use the in-memory version
		fmt.Fprintf(os.Stderr, "Warning: could not migrate config to multi-profile format: %v\n", saveErr)
	}

	return &mc, nil
}

// saveMultiConfig writes the multi-config to disk.
func saveMultiConfig(mc *MultiConfig) error {
	dir, err := Dir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(mc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// Load returns the Config for the active profile.
// The active profile is determined by: --profile flag > JENKINS_PROFILE env var > current_profile in config > "default".
// Environment variables override the loaded profile's fields.
func Load() (*Config, error) {
	mc, err := loadMultiConfig()
	if err != nil {
		return nil, err
	}

	name := resolveProfileName(mc)
	cfg, ok := mc.Profiles[name]
	if !ok {
		available := ProfileNames(mc)
		return nil, fmt.Errorf("profile %q not found. Available profiles: %v\n\nSuggestion:\n  - Run 'jenkins-cli configure --profile %s' to create it\n  - Run 'jenkins-cli profile list' to see available profiles", name, available, name)
	}

	// Environment variables override config file
	if v := os.Getenv("JENKINS_URL"); v != "" {
		cfg.URL = v
	}
	if v := os.Getenv("JENKINS_USER"); v != "" {
		cfg.User = v
	}
	if v := os.Getenv("JENKINS_TOKEN"); v != "" {
		cfg.Token = v
	}
	if v := os.Getenv("JENKINS_BEARER_TOKEN"); v != "" {
		cfg.BearerToken = v
		cfg.AuthType = AuthBearer
	}

	return &cfg, nil
}

// Save writes the given Config to the active profile.
func Save(cfg *Config) error {
	mc, err := loadMultiConfig()
	if err != nil {
		// Only create a new multi-config if the config file does not exist.
		path, pathErr := Path()
		if pathErr != nil {
			return pathErr
		}
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			mc = &MultiConfig{
				CurrentProfile: DefaultProfileName,
				Profiles:       make(map[string]Config),
			}
		} else {
			// For other errors (e.g., parse/permission), return the error to avoid data loss.
			return err
		}
	}

	name := resolveProfileName(mc)

	// Ensure CurrentProfile points to an existing profile. On first save, or if
	// CurrentProfile refers to a missing profile, set it to the profile being saved.
	if _, ok := mc.Profiles[mc.CurrentProfile]; !ok {
		mc.CurrentProfile = name
	}

	mc.Profiles[name] = *cfg

	return saveMultiConfig(mc)
}

// LoadMulti returns the full multi-config. Used by profile management commands.
func LoadMulti() (*MultiConfig, error) {
	return loadMultiConfig()
}

// SaveMulti writes the full multi-config. Used by profile management commands.
func SaveMulti(mc *MultiConfig) error {
	return saveMultiConfig(mc)
}

// ProfileNames returns sorted profile names.
func ProfileNames(mc *MultiConfig) []string {
	names := make([]string, 0, len(mc.Profiles))
	for name := range mc.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// CurrentProfileName returns the effective current profile name.
func CurrentProfileName() (string, error) {
	mc, err := loadMultiConfig()
	if err != nil {
		return "", err
	}
	return resolveProfileName(mc), nil
}
