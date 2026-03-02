package clientutil

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/config"
	"github.com/PhilipKram/jenkins-cli/internal/jenkins"
)

func TestAuthFromConfigBasic(t *testing.T) {
	cfg := &config.Config{
		URL:      "http://localhost:8080",
		User:     "admin",
		Token:    "api-token",
		AuthType: config.AuthBasic,
	}

	auth, err := AuthFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	basic, ok := auth.(*jenkins.BasicAuth)
	if !ok {
		t.Fatalf("expected BasicAuth, got %T", auth)
	}
	if basic.User != "admin" {
		t.Errorf("expected admin, got %s", basic.User)
	}
	if basic.Token != "api-token" {
		t.Errorf("expected api-token, got %s", basic.Token)
	}
}

func TestAuthFromConfigBearer(t *testing.T) {
	cfg := &config.Config{
		URL:         "http://localhost:8080",
		AuthType:    config.AuthBearer,
		BearerToken: "my-bearer-token",
	}

	auth, err := AuthFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bearer, ok := auth.(*jenkins.BearerTokenAuth)
	if !ok {
		t.Fatalf("expected BearerTokenAuth, got %T", auth)
	}
	if bearer.Token != "my-bearer-token" {
		t.Errorf("expected my-bearer-token, got %s", bearer.Token)
	}
}

func TestAuthFromConfigOAuth(t *testing.T) {
	cfg := &config.Config{
		URL:      "http://localhost:8080",
		AuthType: config.AuthOAuth,
		OAuth: &config.OAuthConfig{
			ClientID:    "client-id",
			AccessToken: "oauth-access-token",
		},
	}

	auth, err := AuthFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bearer, ok := auth.(*jenkins.BearerTokenAuth)
	if !ok {
		t.Fatalf("expected BearerTokenAuth (for OAuth), got %T", auth)
	}
	if bearer.Token != "oauth-access-token" {
		t.Errorf("expected oauth-access-token, got %s", bearer.Token)
	}
}

func TestAuthFromConfigBearerEmpty(t *testing.T) {
	cfg := &config.Config{
		URL:         "http://localhost:8080",
		AuthType:    config.AuthBearer,
		BearerToken: "",
	}

	_, err := AuthFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for empty bearer token")
	}
}

func TestAuthFromConfigOAuthNoToken(t *testing.T) {
	cfg := &config.Config{
		URL:      "http://localhost:8080",
		AuthType: config.AuthOAuth,
		OAuth:    &config.OAuthConfig{ClientID: "id"},
	}

	_, err := AuthFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error for OAuth with no access token")
	}
}

func TestAuthFromConfigInferBearer(t *testing.T) {
	// No explicit AuthType, but BearerToken is set — should infer bearer
	cfg := &config.Config{
		URL:         "http://localhost:8080",
		BearerToken: "inferred-token",
	}

	auth, err := AuthFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bearer, ok := auth.(*jenkins.BearerTokenAuth)
	if !ok {
		t.Fatalf("expected BearerTokenAuth, got %T", auth)
	}
	if bearer.Token != "inferred-token" {
		t.Errorf("expected inferred-token, got %s", bearer.Token)
	}
}

func TestAuthFromConfigInferOAuth(t *testing.T) {
	// No explicit AuthType, but OAuth access token is set
	cfg := &config.Config{
		URL: "http://localhost:8080",
		OAuth: &config.OAuthConfig{
			AccessToken: "inferred-oauth",
		},
	}

	auth, err := AuthFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bearer, ok := auth.(*jenkins.BearerTokenAuth)
	if !ok {
		t.Fatalf("expected BearerTokenAuth, got %T", auth)
	}
	if bearer.Token != "inferred-oauth" {
		t.Errorf("expected inferred-oauth, got %s", bearer.Token)
	}
}

func TestClientFromConfig(t *testing.T) {
	cfg := &config.Config{
		URL:      "http://localhost:8080",
		User:     "admin",
		Token:    "token",
		AuthType: config.AuthBasic,
	}

	timeout := 30 * time.Second
	maxRetries := 3
	client, err := ClientFromConfig(cfg, timeout, maxRetries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.BaseURL != "http://localhost:8080" {
		t.Errorf("expected base URL, got %s", client.BaseURL)
	}
	if client.Auth.String() != "basic" {
		t.Errorf("expected basic auth, got %s", client.Auth.String())
	}
	if client.Timeout != timeout {
		t.Errorf("expected timeout %v, got %v", timeout, client.Timeout)
	}
	if client.MaxRetries != maxRetries {
		t.Errorf("expected maxRetries %d, got %d", maxRetries, client.MaxRetries)
	}
}

func TestClientFromConfigInsecureWarning(t *testing.T) {
	// Capture stderr output
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w

	cfg := &config.Config{
		URL:      "http://localhost:8080",
		User:     "admin",
		Token:    "token",
		AuthType: config.AuthBasic,
		Insecure: true,
	}

	timeout := 30 * time.Second
	maxRetries := 3
	client, err := ClientFromConfig(cfg, timeout, maxRetries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close writer and read captured output
	w.Close()
	captured, _ := io.ReadAll(r)
	output := string(captured)

	// Verify client was created successfully
	if client == nil {
		t.Fatal("expected client to be created")
	}

	// Verify warning message appears
	if !strings.Contains(output, "WARNING: TLS certificate verification is disabled") {
		t.Errorf("expected TLS warning in stderr, got: %s", output)
	}
	if !strings.Contains(output, "man-in-the-middle attacks") {
		t.Errorf("expected MITM risk warning in stderr, got: %s", output)
	}
	if !strings.Contains(output, "proper certificates") {
		t.Errorf("expected certificate suggestion in stderr, got: %s", output)
	}
}
