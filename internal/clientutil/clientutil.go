package clientutil

import (
	"fmt"
	"os"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/config"
	"github.com/PhilipKram/jenkins-cli/internal/jenkins"
)

// NewClient creates a Jenkins client based on the current configuration.
func NewClient(timeout time.Duration, maxRetries int) (*jenkins.Client, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load Jenkins configuration: %w\n\nSuggestion:\n  - Run 'jenkins-cli configure' to set up your Jenkins connection", err)
	}
	return ClientFromConfig(cfg, timeout, maxRetries)
}

// ClientFromConfig builds a Jenkins client from the given config,
// selecting the appropriate authentication method.
func ClientFromConfig(cfg *config.Config, timeout time.Duration, maxRetries int) (*jenkins.Client, error) {
	auth, err := AuthFromConfig(cfg)
	if err != nil {
		return nil, err
	}

	if cfg.Insecure {
		fmt.Fprintf(os.Stderr, "WARNING: TLS certificate verification is disabled. Your credentials are vulnerable to man-in-the-middle attacks.\n")
		fmt.Fprintf(os.Stderr, "Suggestion: Use proper certificates or remove the --insecure flag for production environments.\n")
	}

	return jenkins.NewClientWithAuth(cfg.URL, auth, cfg.Insecure, timeout, maxRetries), nil
}

// AuthFromConfig returns the appropriate AuthMethod for the config.
func AuthFromConfig(cfg *config.Config) (jenkins.AuthMethod, error) {
	switch cfg.EffectiveAuthType() {
	case config.AuthBasic:
		return &jenkins.BasicAuth{User: cfg.User, Token: cfg.Token}, nil
	case config.AuthBearer:
		if cfg.BearerToken == "" {
			return nil, fmt.Errorf("bearer authentication is configured but no token is set\n\nSuggestions:\n  - Run 'jenkins-cli auth token' to set a bearer token\n  - Run 'jenkins-cli configure' to change authentication method")
		}
		return &jenkins.BearerTokenAuth{Token: cfg.BearerToken}, nil
	case config.AuthOAuth:
		if cfg.OAuth == nil || cfg.OAuth.AccessToken == "" {
			return nil, fmt.Errorf("OAuth authentication is configured but no access token is available\n\nSuggestions:\n  - Run 'jenkins-cli auth login' to authenticate via OAuth\n  - Run 'jenkins-cli configure' to change authentication method")
		}
		return &jenkins.BearerTokenAuth{Token: cfg.OAuth.AccessToken}, nil
	default:
		return nil, fmt.Errorf("unknown authentication type: '%s'\n\nSuggestions:\n  - Run 'jenkins-cli configure' to set a valid authentication method\n  - Supported methods: basic, bearer, oauth", cfg.EffectiveAuthType())
	}
}
