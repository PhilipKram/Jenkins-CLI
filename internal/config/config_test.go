package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// setupTestHome creates a temp dir and sets HOME to it, resetting ActiveProfile.
func setupTestHome(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	// Clear env vars that could interfere
	t.Setenv("JENKINS_URL", "")
	t.Setenv("JENKINS_USER", "")
	t.Setenv("JENKINS_TOKEN", "")
	t.Setenv("JENKINS_BEARER_TOKEN", "")
	t.Setenv("JENKINS_PROFILE", "")
	// Reset global state
	ActiveProfile = ""
	return tmpDir
}

func TestSaveAndLoad(t *testing.T) {
	setupTestHome(t)

	cfg := &Config{
		URL:      "http://localhost:8080",
		User:     "admin",
		Token:    "secret-token",
		Insecure: true,
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.URL != cfg.URL {
		t.Errorf("URL mismatch: got %s, want %s", loaded.URL, cfg.URL)
	}
	if loaded.User != cfg.User {
		t.Errorf("User mismatch: got %s, want %s", loaded.User, cfg.User)
	}
	if loaded.Token != cfg.Token {
		t.Errorf("Token mismatch: got %s, want %s", loaded.Token, cfg.Token)
	}
	if loaded.Insecure != cfg.Insecure {
		t.Errorf("Insecure mismatch: got %v, want %v", loaded.Insecure, cfg.Insecure)
	}
}

func TestSaveAndLoadCreatesMultiConfig(t *testing.T) {
	tmpDir := setupTestHome(t)

	cfg := &Config{
		URL:  "http://localhost:8080",
		User: "admin",
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the file is in multi-config format
	path := filepath.Join(tmpDir, ".jenkins-cli", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var mc MultiConfig
	if err := json.Unmarshal(data, &mc); err != nil {
		t.Fatalf("unmarshal multi config: %v", err)
	}
	if mc.CurrentProfile != DefaultProfileName {
		t.Errorf("expected current_profile %q, got %q", DefaultProfileName, mc.CurrentProfile)
	}
	if _, ok := mc.Profiles[DefaultProfileName]; !ok {
		t.Error("expected default profile to exist")
	}
}

func TestFilePermissions(t *testing.T) {
	tmpDir := setupTestHome(t)

	cfg := &Config{URL: "http://localhost:8080"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	path := filepath.Join(tmpDir, ".jenkins-cli", "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected file permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestLoadNotConfigured(t *testing.T) {
	setupTestHome(t)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when not configured")
	}
}

func TestEnvOverrides(t *testing.T) {
	setupTestHome(t)

	cfg := &Config{URL: "http://original:8080", User: "original-user", Token: "original-token"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	t.Setenv("JENKINS_URL", "http://override:9090")
	t.Setenv("JENKINS_USER", "env-user")
	t.Setenv("JENKINS_TOKEN", "env-token")

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.URL != "http://override:9090" {
		t.Errorf("expected env URL override, got %s", loaded.URL)
	}
	if loaded.User != "env-user" {
		t.Errorf("expected env User override, got %s", loaded.User)
	}
	if loaded.Token != "env-token" {
		t.Errorf("expected env Token override, got %s", loaded.Token)
	}
}

func TestBearerTokenEnvOverride(t *testing.T) {
	setupTestHome(t)

	cfg := &Config{URL: "http://localhost:8080", User: "admin", Token: "token"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	t.Setenv("JENKINS_BEARER_TOKEN", "my-bearer")

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.BearerToken != "my-bearer" {
		t.Errorf("expected bearer token override, got %s", loaded.BearerToken)
	}
	if loaded.AuthType != AuthBearer {
		t.Errorf("expected auth type bearer, got %s", loaded.AuthType)
	}
}

func TestConfigJSON(t *testing.T) {
	cfg := Config{
		URL:      "http://localhost:8080",
		User:     "admin",
		Token:    "token",
		Insecure: false,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if decoded != cfg {
		t.Errorf("round-trip mismatch: %+v != %+v", decoded, cfg)
	}
}

// --- Multi-profile tests ---

func TestMultipleProfiles(t *testing.T) {
	setupTestHome(t)

	// Save config to "images" profile
	ActiveProfile = "images"
	if err := Save(&Config{URL: "http://images:8080", User: "img-user", Token: "img-token"}); err != nil {
		t.Fatalf("Save images: %v", err)
	}

	// Save config to "helm" profile
	ActiveProfile = "helm"
	if err := Save(&Config{URL: "http://helm:8080", User: "helm-user", Token: "helm-token"}); err != nil {
		t.Fatalf("Save helm: %v", err)
	}

	// Load images profile
	ActiveProfile = "images"
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load images: %v", err)
	}
	if loaded.URL != "http://images:8080" {
		t.Errorf("images URL: got %s, want http://images:8080", loaded.URL)
	}
	if loaded.User != "img-user" {
		t.Errorf("images User: got %s, want img-user", loaded.User)
	}

	// Load helm profile
	ActiveProfile = "helm"
	loaded, err = Load()
	if err != nil {
		t.Fatalf("Load helm: %v", err)
	}
	if loaded.URL != "http://helm:8080" {
		t.Errorf("helm URL: got %s, want http://helm:8080", loaded.URL)
	}
	if loaded.User != "helm-user" {
		t.Errorf("helm User: got %s, want helm-user", loaded.User)
	}
}

func TestProfileNotFound(t *testing.T) {
	setupTestHome(t)

	// Create a default config
	ActiveProfile = ""
	if err := Save(&Config{URL: "http://localhost:8080"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Try loading a non-existent profile
	ActiveProfile = "nonexistent"
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for nonexistent profile")
	}
}

func TestCurrentProfileSwitching(t *testing.T) {
	setupTestHome(t)

	// Create two profiles
	ActiveProfile = "alpha"
	if err := Save(&Config{URL: "http://alpha:8080"}); err != nil {
		t.Fatalf("Save alpha: %v", err)
	}
	ActiveProfile = "beta"
	if err := Save(&Config{URL: "http://beta:8080"}); err != nil {
		t.Fatalf("Save beta: %v", err)
	}

	// Switch current profile to alpha
	mc, err := LoadMulti()
	if err != nil {
		t.Fatalf("LoadMulti: %v", err)
	}
	mc.CurrentProfile = "alpha"
	if err := SaveMulti(mc); err != nil {
		t.Fatalf("SaveMulti: %v", err)
	}

	// With no ActiveProfile override, should load alpha
	ActiveProfile = ""
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.URL != "http://alpha:8080" {
		t.Errorf("expected alpha URL, got %s", loaded.URL)
	}

	// ActiveProfile flag should override current_profile
	ActiveProfile = "beta"
	loaded, err = Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.URL != "http://beta:8080" {
		t.Errorf("expected beta URL, got %s", loaded.URL)
	}
}

func TestJenkinsProfileEnvVar(t *testing.T) {
	setupTestHome(t)

	// Create two profiles
	ActiveProfile = "prod"
	if err := Save(&Config{URL: "http://prod:8080"}); err != nil {
		t.Fatalf("Save prod: %v", err)
	}
	ActiveProfile = "staging"
	if err := Save(&Config{URL: "http://staging:8080"}); err != nil {
		t.Fatalf("Save staging: %v", err)
	}

	// JENKINS_PROFILE env var should select the profile
	ActiveProfile = ""
	t.Setenv("JENKINS_PROFILE", "staging")

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.URL != "http://staging:8080" {
		t.Errorf("expected staging URL, got %s", loaded.URL)
	}
}

func TestActiveProfileOverridesEnvVar(t *testing.T) {
	setupTestHome(t)

	ActiveProfile = "one"
	if err := Save(&Config{URL: "http://one:8080"}); err != nil {
		t.Fatalf("Save one: %v", err)
	}
	ActiveProfile = "two"
	if err := Save(&Config{URL: "http://two:8080"}); err != nil {
		t.Fatalf("Save two: %v", err)
	}

	// --profile flag (ActiveProfile) should take precedence over JENKINS_PROFILE
	t.Setenv("JENKINS_PROFILE", "two")
	ActiveProfile = "one"

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.URL != "http://one:8080" {
		t.Errorf("expected one URL, got %s", loaded.URL)
	}
}

func TestBackwardCompatMigration(t *testing.T) {
	tmpDir := setupTestHome(t)

	// Write a legacy flat config
	dir := filepath.Join(tmpDir, ".jenkins-cli")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := Config{
		URL:   "http://legacy:8080",
		User:  "legacy-user",
		Token: "legacy-token",
	}
	data, _ := json.Marshal(legacy)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), data, 0600); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	// Load should auto-migrate
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.URL != "http://legacy:8080" {
		t.Errorf("expected legacy URL, got %s", loaded.URL)
	}
	if loaded.User != "legacy-user" {
		t.Errorf("expected legacy user, got %s", loaded.User)
	}

	// Verify the file was migrated to multi-config format
	rawData, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var mc MultiConfig
	if err := json.Unmarshal(rawData, &mc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if mc.CurrentProfile != DefaultProfileName {
		t.Errorf("expected current_profile %q after migration, got %q", DefaultProfileName, mc.CurrentProfile)
	}
	if _, ok := mc.Profiles[DefaultProfileName]; !ok {
		t.Error("expected default profile after migration")
	}
}

func TestProfileNames(t *testing.T) {
	mc := &MultiConfig{
		Profiles: map[string]Config{
			"zebra": {URL: "http://z"},
			"alpha": {URL: "http://a"},
			"mid":   {URL: "http://m"},
		},
	}
	names := ProfileNames(mc)
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	// Should be sorted
	if names[0] != "alpha" || names[1] != "mid" || names[2] != "zebra" {
		t.Errorf("expected sorted names [alpha mid zebra], got %v", names)
	}
}

func TestSaveToNewProfileCreatesIt(t *testing.T) {
	setupTestHome(t)

	// Start with no config at all — Save should create everything
	ActiveProfile = "fresh"
	if err := Save(&Config{URL: "http://fresh:8080"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	mc, err := LoadMulti()
	if err != nil {
		t.Fatalf("LoadMulti: %v", err)
	}
	if _, ok := mc.Profiles["fresh"]; !ok {
		t.Error("expected fresh profile to exist")
	}
}

func TestEffectiveAuthType(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		expected AuthType
	}{
		{"explicit basic", Config{AuthType: AuthBasic}, AuthBasic},
		{"explicit bearer", Config{AuthType: AuthBearer}, AuthBearer},
		{"explicit oauth", Config{AuthType: AuthOAuth}, AuthOAuth},
		{"infer bearer", Config{BearerToken: "tok"}, AuthBearer},
		{"infer oauth", Config{OAuth: &OAuthConfig{AccessToken: "tok"}}, AuthOAuth},
		{"infer basic", Config{User: "u", Token: "t"}, AuthBasic},
		{"empty defaults to basic", Config{}, AuthBasic},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.EffectiveAuthType()
			if got != tt.expected {
				t.Errorf("got %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestSaveUpdatesExistingProfile(t *testing.T) {
	setupTestHome(t)

	ActiveProfile = "myprofile"
	if err := Save(&Config{URL: "http://v1:8080", User: "user1"}); err != nil {
		t.Fatalf("Save v1: %v", err)
	}

	// Update the same profile
	if err := Save(&Config{URL: "http://v2:8080", User: "user2"}); err != nil {
		t.Fatalf("Save v2: %v", err)
	}

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.URL != "http://v2:8080" {
		t.Errorf("expected updated URL, got %s", loaded.URL)
	}
	if loaded.User != "user2" {
		t.Errorf("expected updated user, got %s", loaded.User)
	}

	// Should still have only one profile
	mc, err := LoadMulti()
	if err != nil {
		t.Fatalf("LoadMulti: %v", err)
	}
	if len(mc.Profiles) != 1 {
		t.Errorf("expected 1 profile, got %d", len(mc.Profiles))
	}
}

func TestEnvOverridesWithProfiles(t *testing.T) {
	setupTestHome(t)

	ActiveProfile = "myenv"
	if err := Save(&Config{URL: "http://original:8080", User: "orig"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	t.Setenv("JENKINS_URL", "http://env-override:9090")

	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.URL != "http://env-override:9090" {
		t.Errorf("expected env URL override with profiles, got %s", loaded.URL)
	}
}
