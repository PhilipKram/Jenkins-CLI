package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	// Use a temp directory
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	cfg := &Config{
		URL:      "http://localhost:8080",
		User:     "admin",
		Token:    "secret-token",
		Insecure: true,
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists with correct permissions
	path := filepath.Join(tmpDir, ".jenkins-cli", "config.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("config file not found: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected file permissions 0600, got %o", info.Mode().Perm())
	}

	// Load and verify
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

func TestLoadNotConfigured(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when not configured")
	}
}

func TestEnvOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// Save a base config
	cfg := &Config{URL: "http://original:8080", User: "original-user", Token: "original-token"}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Set env overrides
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
