package jenkins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	jenkinserrors "github.com/PhilipKram/jenkins-cli/internal/errors"
)

// Credential represents basic credential information from Jenkins
type Credential struct {
	ID          string `json:"id"`
	Type        string `json:"typeName"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// CredentialDetail represents detailed credential information
type CredentialDetail struct {
	Credential
	Scope  string `json:"scope"`
	Domain string `json:"domain,omitempty"`
}

// CredentialCreateRequest represents a request to create a new credential
type CredentialCreateRequest struct {
	Credentials CredentialPayload `json:"credentials"`
}

// CredentialPayload represents the credential data for creation
type CredentialPayload struct {
	Scope       string            `json:"scope"`
	ID          string            `json:"id"`
	Username    string            `json:"username,omitempty"`
	Password    string            `json:"password,omitempty"`
	Secret      string            `json:"secret,omitempty"`
	PrivateKey  string            `json:"privateKey,omitempty"`
	Passphrase  string            `json:"passphrase,omitempty"`
	Description string            `json:"description,omitempty"`
	Class       string            `json:"stapler-class"`
	TypeName    string            `json:"$class"`
}

type credentialListResponse struct {
	Credentials []Credential `json:"credentials"`
}

// credentialStorePaths returns the paths to try for the credential store, in order.
// Newer Jenkins versions (2.�+) require the /manage/ prefix.
func credentialStorePaths(suffix string) []string {
	return []string{
		"/credentials/store/system/" + suffix,
		"/manage/credentials/store/system/" + suffix,
	}
}

// ListCredentials lists all credentials in the specified domain.
// If domain is empty, "_" (global domain) is used.
func (c *Client) ListCredentials(ctx context.Context, domain string) ([]Credential, error) {
	if domain == "" {
		domain = "_"
	}

	suffix := fmt.Sprintf("domain/%s/api/json?tree=credentials[id,typeName,displayName,description,fingerprint]", url.PathEscape(domain))

	var resp credentialListResponse
	var lastErr error
	for _, path := range credentialStorePaths(suffix) {
		lastErr = c.get(ctx, path, &resp)
		if lastErr == nil {
			return resp.Credentials, nil
		}
		if !jenkinserrors.IsNotFound(lastErr) {
			return nil, fmt.Errorf("listing credentials: %w", lastErr)
		}
	}
	return nil, fmt.Errorf("listing credentials: %w", lastErr)
}

// GetCredential retrieves detailed information about a specific credential.
// If domain is empty, "_" (global domain) is used.
func (c *Client) GetCredential(ctx context.Context, id, domain string) (*CredentialDetail, error) {
	if domain == "" {
		domain = "_"
	}

	suffix := fmt.Sprintf("domain/%s/credential/%s/api/json",
		url.PathEscape(domain), url.PathEscape(id))

	var cred CredentialDetail
	var lastErr error
	for _, path := range credentialStorePaths(suffix) {
		lastErr = c.get(ctx, path, &cred)
		if lastErr == nil {
			cred.Domain = domain
			return &cred, nil
		}
		if !jenkinserrors.IsNotFound(lastErr) {
			return nil, c.wrapNotFoundError(lastErr, id)
		}
	}
	return nil, c.wrapNotFoundError(lastErr, id)
}

// CreateCredential creates a new credential in Jenkins.
// credType must be one of: username-password, secret-text, ssh-key, certificate
// If domain is empty, "_" (global domain) is used.
func (c *Client) CreateCredential(ctx context.Context, credType, domain string, payload CredentialPayload) error {
	if domain == "" {
		domain = "_"
	}

	// Set the credential type class based on credType
	switch credType {
	case "username-password":
		payload.Class = "com.cloudbees.plugins.credentials.impl.UsernamePasswordCredentialsImpl"
		payload.TypeName = "com.cloudbees.plugins.credentials.impl.UsernamePasswordCredentialsImpl"
	case "secret-text":
		payload.Class = "org.jenkinsci.plugins.plaincredentials.impl.StringCredentialsImpl"
		payload.TypeName = "org.jenkinsci.plugins.plaincredentials.impl.StringCredentialsImpl"
	case "ssh-key":
		payload.Class = "com.cloudbees.jenkins.plugins.sshcredentials.impl.BasicSSHUserPrivateKey"
		payload.TypeName = "com.cloudbees.jenkins.plugins.sshcredentials.impl.BasicSSHUserPrivateKey"
	case "certificate":
		payload.Class = "com.cloudbees.plugins.credentials.impl.CertificateCredentialsImpl"
		payload.TypeName = "com.cloudbees.plugins.credentials.impl.CertificateCredentialsImpl"
	default:
		return fmt.Errorf("unsupported credential type: %s", credType)
	}

	// Default scope to GLOBAL if not set
	if payload.Scope == "" {
		payload.Scope = "GLOBAL"
	}

	// Create the JSON payload
	reqBody := map[string]interface{}{
		"": "0",
		"credentials": payload,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling credential payload: %w", err)
	}

	suffix := fmt.Sprintf("domain/%s/createCredentials", url.PathEscape(domain))

	// Use postFormWithCrumb for credential creation as Jenkins expects form data
	// Try multiple credential store paths for compatibility with different Jenkins versions
	var lastErr error
	for _, path := range credentialStorePaths(suffix) {
		lastErr = c.postFormWithCrumb(ctx, path, bytes.NewReader(jsonData))
		if lastErr == nil {
			return nil
		}
		if !jenkinserrors.IsNotFound(lastErr) {
			return fmt.Errorf("creating credential: %w", lastErr)
		}
	}
	return fmt.Errorf("creating credential: %w", lastErr)
}

// DeleteCredential removes a credential from Jenkins.
// If domain is empty, "_" (global domain) is used.
func (c *Client) DeleteCredential(ctx context.Context, id, domain string) error {
	if domain == "" {
		domain = "_"
	}

	suffix := fmt.Sprintf("domain/%s/credential/%s/doDelete",
		url.PathEscape(domain), url.PathEscape(id))

	var lastErr error
	for _, path := range credentialStorePaths(suffix) {
		lastErr = c.postWithCrumb(ctx, path, nil)
		if lastErr == nil {
			return nil
		}
		if !jenkinserrors.IsNotFound(lastErr) {
			return c.wrapNotFoundError(lastErr, id)
		}
	}
	return c.wrapNotFoundError(lastErr, id)
}
