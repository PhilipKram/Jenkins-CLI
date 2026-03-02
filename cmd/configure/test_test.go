package configure

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/config"
	"github.com/PhilipKram/jenkins-cli/internal/jenkins"
)

func TestTestCmd(t *testing.T) {
	if testCmd == nil {
		t.Fatal("testCmd should not be nil")
	}
	if testCmd.Use != "test" {
		t.Errorf("expected Use='test', got %s", testCmd.Use)
	}
	if testCmd.Short == "" {
		t.Error("Short description should not be empty")
	}
	if testCmd.RunE == nil {
		t.Error("RunE should not be nil")
	}
}

func TestRunTestSuccess(t *testing.T) {
	// Create a test server that responds successfully
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			t.Errorf("expected HEAD request, got %s", r.Method)
		}
		if r.URL.Path != "/" {
			t.Errorf("expected path /, got %s", r.URL.Path)
		}

		// Verify auth header
		user, pass, ok := r.BasicAuth()
		if !ok || user != "testuser" || pass != "testtoken" {
			t.Error("expected basic auth with testuser:testtoken")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a temporary home directory
	tmpHome := t.TempDir()
	jenkinsCliDir := filepath.Join(tmpHome, ".jenkins-cli")
	if err := os.MkdirAll(jenkinsCliDir, 0700); err != nil {
		t.Fatalf("failed to create jenkins-cli dir: %v", err)
	}
	configPath := filepath.Join(jenkinsCliDir, "config.json")

	cfg := &config.Config{
		URL:      server.URL,
		User:     "testuser",
		Token:    "testtoken",
		Insecure: false,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Set context on command
	testCmd.SetContext(context.Background())

	// Run the command
	err = runTest(testCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !bytes.Contains([]byte(output), []byte("Testing connection")) {
		t.Errorf("expected 'Testing connection' in output, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("Connection successful")) {
		t.Errorf("expected 'Connection successful' in output, got: %s", output)
	}
}

func TestRunTestConnectionFailed(t *testing.T) {
	// Create a test server that responds with error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("Unauthorized"))
	}))
	defer server.Close()

	// Create a temporary home directory
	tmpHome := t.TempDir()
	jenkinsCliDir := filepath.Join(tmpHome, ".jenkins-cli")
	if err := os.MkdirAll(jenkinsCliDir, 0700); err != nil {
		t.Fatalf("failed to create jenkins-cli dir: %v", err)
	}
	configPath := filepath.Join(jenkinsCliDir, "config.json")

	cfg := &config.Config{
		URL:      server.URL,
		User:     "testuser",
		Token:    "wrongtoken",
		Insecure: false,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Run the command
	err = runTest(testCmd, []string{})

	if err == nil {
		t.Fatal("expected error for failed connection, got nil")
	}

	// Error should mention connection test failed
	if !bytes.Contains([]byte(err.Error()), []byte("connection test failed")) {
		t.Errorf("expected 'connection test failed' in error, got: %v", err)
	}
}

func TestRunTestWithBearerAuth(t *testing.T) {
	// Create a test server that expects bearer auth
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-bearer-token" {
			t.Errorf("expected Bearer header, got %q", auth)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a temporary home directory
	tmpHome := t.TempDir()
	jenkinsCliDir := filepath.Join(tmpHome, ".jenkins-cli")
	if err := os.MkdirAll(jenkinsCliDir, 0700); err != nil {
		t.Fatalf("failed to create jenkins-cli dir: %v", err)
	}
	configPath := filepath.Join(jenkinsCliDir, "config.json")

	cfg := &config.Config{
		URL:         server.URL,
		AuthType:    config.AuthBearer,
		BearerToken: "test-bearer-token",
		Insecure:    false,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Set context on command
	testCmd.SetContext(context.Background())

	// Run the command
	err = runTest(testCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunTestNoConfig(t *testing.T) {
	// Create a temporary home directory without config
	tmpHome := t.TempDir()

	// Set HOME to temp directory (no .jenkins-cli directory)
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Run the command
	err := runTest(testCmd, []string{})

	if err == nil {
		t.Fatal("expected error for missing config, got nil")
	}

	// Error should mention loading config
	if !bytes.Contains([]byte(err.Error()), []byte("loading config")) {
		t.Errorf("expected 'loading config' in error, got: %v", err)
	}
}

func TestRunTestServerUnreachable(t *testing.T) {
	// Create a temporary config file with unreachable server
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := &config.Config{
		URL:      "http://localhost:1", // Invalid port
		User:     "testuser",
		Token:    "testtoken",
		Insecure: false,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Set config path env var
	oldConfigPath := os.Getenv("JENKINS_CLI_CONFIG")
	os.Setenv("JENKINS_CLI_CONFIG", configPath)
	defer os.Setenv("JENKINS_CLI_CONFIG", oldConfigPath)

	// Create client with very short timeout
	client := jenkins.NewClient("http://localhost:1", "testuser", "testtoken", false, 5*time.Second, 0)

	// Test connection should fail
	err = client.TestConnection(context.Background())

	if err == nil {
		t.Fatal("expected error for unreachable server, got nil")
	}
}

func TestRunTestInsecureConnection(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a temporary home directory
	tmpHome := t.TempDir()
	jenkinsCliDir := filepath.Join(tmpHome, ".jenkins-cli")
	if err := os.MkdirAll(jenkinsCliDir, 0700); err != nil {
		t.Fatalf("failed to create jenkins-cli dir: %v", err)
	}
	configPath := filepath.Join(jenkinsCliDir, "config.json")

	cfg := &config.Config{
		URL:      server.URL,
		User:     "testuser",
		Token:    "testtoken",
		Insecure: true,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Set HOME to temp directory
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", oldHome)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Set context on command
	testCmd.SetContext(context.Background())

	// Run the command
	err = runTest(testCmd, []string{})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
