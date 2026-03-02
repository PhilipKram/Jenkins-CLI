package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// AuthType represents the authentication method used.
type AuthType string

const (
	AuthBasic  AuthType = "basic"
	AuthOAuth  AuthType = "oauth"
	AuthBearer AuthType = "bearer"
)

// OAuthConfig holds OAuth2 provider settings.
type OAuthConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
	AuthURL      string `json:"auth_url"`
	TokenURL     string `json:"token_url"`
	Scopes       []string `json:"scopes,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

type Config struct {
	URL         string       `json:"url"`
	User        string       `json:"user,omitempty"`
	Token       string       `json:"token,omitempty"`
	Insecure    bool         `json:"insecure"`
	AuthType    AuthType     `json:"auth_type,omitempty"`
	BearerToken string       `json:"bearer_token,omitempty"`
	OAuth       *OAuthConfig `json:"oauth,omitempty"`
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

func Load() (*Config, error) {
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

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
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

func Save(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}
