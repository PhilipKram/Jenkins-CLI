package jenkins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestListCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/credentials/store/system/domain/_/api/json"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(credentialListResponse{
			Credentials: []Credential{
				{
					ID:          "cred-1",
					Type:        "Username with password",
					DisplayName: "Test User",
					Description: "Test credential",
				},
				{
					ID:          "cred-2",
					Type:        "Secret text",
					DisplayName: "API Token",
					Description: "API token for service",
				},
			},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	creds, err := c.ListCredentials(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(creds) != 2 {
		t.Fatalf("expected 2 credentials, got %d", len(creds))
	}
	if creds[0].ID != "cred-1" {
		t.Errorf("expected cred-1, got %s", creds[0].ID)
	}
	if creds[1].ID != "cred-2" {
		t.Errorf("expected cred-2, got %s", creds[1].ID)
	}
}

func TestListCredentialsCustomDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/credentials/store/system/domain/custom-domain/api/json"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(credentialListResponse{
			Credentials: []Credential{
				{ID: "domain-cred", Type: "SSH Key"},
			},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	creds, err := c.ListCredentials(context.Background(), "custom-domain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(creds) != 1 || creds[0].ID != "domain-cred" {
		t.Errorf("unexpected credentials: %v", creds)
	}
}

func TestGetCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/credentials/store/system/domain/_/credential/my-cred/api/json"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CredentialDetail{
			Credential: Credential{
				ID:          "my-cred",
				Type:        "Username with password",
				DisplayName: "My Credential",
				Description: "Test credential detail",
			},
			Scope: "GLOBAL",
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	cred, err := c.GetCredential(context.Background(), "my-cred", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cred.ID != "my-cred" {
		t.Errorf("expected my-cred, got %s", cred.ID)
	}
	if cred.Scope != "GLOBAL" {
		t.Errorf("expected GLOBAL scope, got %s", cred.Scope)
	}
	if cred.Domain != "_" {
		t.Errorf("expected _ domain, got %s", cred.Domain)
	}
}

func TestGetCredentialCustomDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/credentials/store/system/domain/my-domain/credential/test-cred/api/json"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CredentialDetail{
			Credential: Credential{
				ID:   "test-cred",
				Type: "Secret text",
			},
			Scope: "GLOBAL",
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	cred, err := c.GetCredential(context.Background(), "test-cred", "my-domain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cred.Domain != "my-domain" {
		t.Errorf("expected my-domain, got %s", cred.Domain)
	}
}

func TestGetCredentialNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	_, err := c.GetCredential(context.Background(), "nonexistent", "")
	if err == nil {
		t.Fatal("expected error for non-existent credential")
	}
}

func TestCreateCredentialUsernamePassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/credentials/store/system/domain/_/createCredentials":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	payload := CredentialPayload{
		ID:          "test-user",
		Username:    "testuser",
		Password:    "testpass",
		Description: "Test username/password credential",
	}
	err := c.CreateCredential(context.Background(), "username-password", "", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateCredentialSecretText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/credentials/store/system/domain/_/createCredentials":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	payload := CredentialPayload{
		ID:          "api-token",
		Secret:      "secret-value",
		Description: "API token",
	}
	err := c.CreateCredential(context.Background(), "secret-text", "", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateCredentialSSHKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/credentials/store/system/domain/_/createCredentials":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	payload := CredentialPayload{
		ID:          "ssh-key",
		Username:    "git",
		PrivateKey:  "-----BEGIN PRIVATE KEY-----",
		Passphrase:  "key-passphrase",
		Description: "SSH key for git",
	}
	err := c.CreateCredential(context.Background(), "ssh-key", "", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateCredentialCertificate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/credentials/store/system/domain/_/createCredentials":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	payload := CredentialPayload{
		ID:          "cert",
		Description: "Certificate credential",
	}
	err := c.CreateCredential(context.Background(), "certificate", "", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateCredentialCustomDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/credentials/store/system/domain/custom/createCredentials":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	payload := CredentialPayload{
		ID:       "test-cred",
		Username: "user",
		Password: "pass",
	}
	err := c.CreateCredential(context.Background(), "username-password", "custom", payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateCredentialUnsupportedType(t *testing.T) {
	c := NewClient("http://localhost:8080", "admin", "token", false, 30*time.Second, 3)
	payload := CredentialPayload{
		ID: "test",
	}
	err := c.CreateCredential(context.Background(), "invalid-type", "", payload)
	if err == nil {
		t.Fatal("expected error for unsupported credential type")
	}
	if err.Error() != "unsupported credential type: invalid-type" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDeleteCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/credentials/store/system/domain/_/credential/my-cred/doDelete":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.DeleteCredential(context.Background(), "my-cred", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteCredentialCustomDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/credentials/store/system/domain/my-domain/credential/test-cred/doDelete":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.DeleteCredential(context.Background(), "test-cred", "my-domain")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteCredentialNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/credentials/store/system/domain/_/credential/nonexistent/doDelete",
			"/manage/credentials/store/system/domain/_/credential/nonexistent/doDelete":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.DeleteCredential(context.Background(), "nonexistent", "")
	if err == nil {
		t.Fatal("expected error for non-existent credential")
	}
}
