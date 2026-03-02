package jenkins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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

// ListCredentials lists all credentials in the specified domain.
// If domain is empty, "_" (global domain) is used.
func (c *Client) ListCredentials(ctx context.Context, domain string) ([]Credential, error) {
	if domain == "" {
		domain = "_"
	}

	path := fmt.Sprintf("/credentials/store/system/domain/%s/api/json?tree=credentials[id,typeName,displayName,description,fingerprint]", url.PathEscape(domain))

	var resp credentialListResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("listing credentials: %w", err)
	}
	return resp.Credentials, nil
}

// GetCredential retrieves detailed information about a specific credential.
// If domain is empty, "_" (global domain) is used.
func (c *Client) GetCredential(ctx context.Context, id, domain string) (*CredentialDetail, error) {
	if domain == "" {
		domain = "_"
	}

	path := fmt.Sprintf("/credentials/store/system/domain/%s/credential/%s/api/json",
		url.PathEscape(domain), url.PathEscape(id))

	var cred CredentialDetail
	if err := c.get(ctx, path, &cred); err != nil {
		return nil, c.wrapNotFoundError(err, id)
	}
	cred.Domain = domain
	return &cred, nil
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

	path := fmt.Sprintf("/credentials/store/system/domain/%s/createCredentials", url.PathEscape(domain))

	// Use postFormWithCrumb for credential creation as Jenkins expects form data
	if err := c.postFormWithCrumb(ctx, path, bytes.NewReader(jsonData)); err != nil {
		return fmt.Errorf("creating credential: %w", err)
	}

	return nil
}

// DeleteCredential removes a credential from Jenkins.
// If domain is empty, "_" (global domain) is used.
func (c *Client) DeleteCredential(ctx context.Context, id, domain string) error {
	if domain == "" {
		domain = "_"
	}

	path := fmt.Sprintf("/credentials/store/system/domain/%s/credential/%s/doDelete",
		url.PathEscape(domain), url.PathEscape(id))

	return c.wrapNotFoundError(c.postWithCrumb(ctx, path, nil), id)
}
