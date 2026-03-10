package jenkins

import (
	"context"
	"crypto/tls"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/errors"
)

// Per-operation timeout constants.
// These define appropriate timeout durations for different categories of Jenkins operations.
const (
	// FastTimeout is used for lightweight operations that should complete quickly.
	// Examples: getCrumb, TestConnection
	FastTimeout = 5 * time.Second

	// StandardTimeout is used for typical API operations.
	// Examples: ListBuilds, GetBuild, GetJob, ListPlugins
	StandardTimeout = 30 * time.Second

	// SlowTimeout is used for operations that may take longer to complete.
	// Examples: DownloadArtifact, large data transfers
	SlowTimeout = 60 * time.Second
)

// AuthMethod applies authentication to an outgoing HTTP request.
type AuthMethod interface {
	Apply(req *http.Request)
	String() string
}

// BasicAuth authenticates via HTTP Basic (username + API token).
type BasicAuth struct {
	User  string
	Token string
}

func (a *BasicAuth) Apply(req *http.Request) {
	if a.User != "" && a.Token != "" {
		req.SetBasicAuth(a.User, a.Token)
	}
}

func (a *BasicAuth) String() string { return "basic" }

// BearerTokenAuth authenticates via an Authorization: Bearer header.
type BearerTokenAuth struct {
	Token string
}

func (a *BearerTokenAuth) Apply(req *http.Request) {
	if a.Token != "" {
		req.Header.Set("Authorization", "Bearer "+a.Token)
	}
}

func (a *BearerTokenAuth) String() string { return "bearer" }

type Client struct {
	BaseURL    string
	Auth       AuthMethod
	HTTPClient *http.Client
	MaxRetries int
	Timeout    time.Duration
}

// NewClient creates a client using HTTP Basic authentication (backward compatible).
func NewClient(baseURL, user, token string, insecure bool, timeout time.Duration, maxRetries int) *Client {
	return NewClientWithAuth(baseURL, &BasicAuth{User: user, Token: token}, insecure, timeout, maxRetries)
}

// NewClientWithAuth creates a client with the given authentication method.
func NewClientWithAuth(baseURL string, auth AuthMethod, insecure bool, timeout time.Duration, maxRetries int) *Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
	}
	return &Client{
		BaseURL:    stripJobPath(strings.TrimRight(baseURL, "/")),
		Auth:       auth,
		MaxRetries: maxRetries,
		Timeout:    timeout,
		HTTPClient: &http.Client{
			Transport: transport,
		},
	}
}

// stripJobPath removes any /job/... suffix from a Jenkins URL so that the
// BaseURL always points to the Jenkins root. Users sometimes configure a
// job-specific URL (e.g. copied from a browser), but all API methods already
// prepend /job/ when constructing paths.
func stripJobPath(u string) string {
	if idx := strings.Index(u, "/job/"); idx != -1 {
		return u[:idx]
	}
	return u
}

func (c *Client) buildURL(path string) string {
	return c.BaseURL + path
}

// withFastTimeout wraps the given context with a fast operation timeout (5s).
// Use for lightweight operations like getCrumb, TestConnection.
// The caller must call the returned cancel function when done.
func (c *Client) withFastTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, FastTimeout)
}

// withStandardTimeout wraps the given context with a standard operation timeout (30s).
// Use for typical API operations like ListBuilds, GetBuild, GetJob, ListPlugins.
// The caller must call the returned cancel function when done.
func (c *Client) withStandardTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, StandardTimeout)
}

// withSlowTimeout wraps the given context with a slow operation timeout (60s).
// Use for operations that may take longer like DownloadArtifact, large data transfers.
// The caller must call the returned cancel function when done.
func (c *Client) withSlowTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, SlowTimeout)
}

// isTransientError determines if an error is transient and should be retried.
// Transient errors include: connection refused, timeouts, and HTTP 502/503/504.
func isTransientError(err error, statusCode int) bool {
	if err != nil {
		// Check for network errors (connection refused, timeout, etc.)
		var netErr net.Error
		if stderrors.As(err, &netErr) {
			return true // Network errors are transient
		}
		// Check for connection refused
		var opErr *net.OpError
		if stderrors.As(err, &opErr) {
			return true
		}
	}

	// Check for transient HTTP status codes
	if statusCode == http.StatusBadGateway || // 502
		statusCode == http.StatusServiceUnavailable || // 503
		statusCode == http.StatusGatewayTimeout { // 504
		return true
	}

	return false
}

// retryWithBackoff executes a function with exponential backoff retry logic.
// It retries up to maxRetries times for transient errors, with delays of 1s, 2s, 4s, etc.
// It respects context cancellation and will abort retries if the context is cancelled.
func retryWithBackoff(ctx context.Context, maxRetries int, operation func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Check if context is cancelled before attempting
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		resp, lastErr = operation()

		// Success - return immediately
		if lastErr == nil && resp != nil && resp.StatusCode < 500 {
			return resp, nil
		}

		// Determine status code for transient check
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}

		// Check if error is transient
		if !isTransientError(lastErr, statusCode) {
			// Non-transient error - don't retry
			return resp, lastErr
		}

		// Don't sleep after the last attempt
		if attempt < maxRetries {
			// Exponential backoff: 1s, 2s, 4s, 8s, etc.
			backoffDuration := time.Duration(1<<uint(attempt)) * time.Second

			// Log retry attempt to stderr
			if lastErr != nil {
				fmt.Fprintf(os.Stderr, "Request failed (attempt %d/%d): %v. Retrying in %v...\n",
					attempt+1, maxRetries+1, lastErr, backoffDuration)
			} else if statusCode >= 500 {
				fmt.Fprintf(os.Stderr, "Request failed with HTTP %d (attempt %d/%d). Retrying in %v...\n",
					statusCode, attempt+1, maxRetries+1, backoffDuration)
			}

			// Use context-aware sleep
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDuration):
				// Continue to next attempt
			}
		}
	}

	return resp, lastErr
}

// classifyHTTPError converts HTTP status codes into structured errors with helpful suggestions.
func (c *Client) classifyHTTPError(statusCode int, url, body string) error {
	authMethod := "unknown"
	if c.Auth != nil {
		authMethod = c.Auth.String()
	}

	switch statusCode {
	case http.StatusUnauthorized:
		suggestions := []string{
			"Check that your API token is valid and not expired",
			"Verify that your username is correct",
		}
		if authMethod == "basic" {
			suggestions = append(suggestions, "Generate a new API token in Jenkins under User > Configure > API Token")
		} else if authMethod == "bearer" {
			suggestions = append(suggestions, "Verify that your bearer token is valid")
		}
		return &errors.AuthenticationError{
			URL:         url,
			AuthMethod:  authMethod,
			StatusCode:  statusCode,
			Err:         fmt.Errorf("%s", body),
			Suggestions: suggestions,
		}

	case http.StatusForbidden:
		// Check if it's a permission error
		if strings.Contains(strings.ToLower(body), "permission") ||
			strings.Contains(strings.ToLower(body), "anonymous") {
			suggestions := []string{
				"Check that your user has the required permissions in Jenkins",
				"Contact your Jenkins administrator to grant necessary permissions",
			}
			if strings.Contains(strings.ToLower(body), "anonymous") {
				suggestions = append(suggestions, "Anonymous access may be disabled - provide authentication credentials")
			}
			return &errors.PermissionError{
				URL:         url,
				AuthMethod:  authMethod,
				Err:         fmt.Errorf("%s", body),
				Suggestions: suggestions,
			}
		}
		// Otherwise treat as authentication error
		return &errors.AuthenticationError{
			URL:         url,
			AuthMethod:  authMethod,
			StatusCode:  statusCode,
			Err:         fmt.Errorf("%s", body),
			Suggestions: []string{
				"Check that your credentials are correct",
				"Verify that your user account is enabled",
			},
		}

	case http.StatusNotFound:
		suggestions := []string{
			"Verify that the resource exists in Jenkins",
			"Check that the URL is correct: " + url,
		}
		return &errors.NotFoundError{
			URL:         url,
			Err:         fmt.Errorf("%s", body),
			Suggestions: suggestions,
		}

	default:
		// For other errors, return a generic error
		return fmt.Errorf("HTTP %d: %s", statusCode, body)
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.buildURL(path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.Auth.Apply(req)

	// Use retry logic with configured maxRetries
	resp, err := retryWithBackoff(ctx, c.MaxRetries, func() (*http.Response, error) {
		return c.HTTPClient.Do(req)
	})
	if err != nil {
		// Wrap network errors as ConnectionError with helpful suggestions
		suggestions := []string{}

		if errors.IsConnectionRefused(err) {
			suggestions = append(suggestions, "Check that Jenkins is running and accessible at "+c.BaseURL)
			suggestions = append(suggestions, "Verify the URL is correct (e.g., http://localhost:8080)")
		} else if errors.IsDNSError(err) {
			suggestions = append(suggestions, "Check that the hostname is correct")
			suggestions = append(suggestions, "Verify your network connection")
		} else if errors.IsTimeoutError(err) {
			suggestions = append(suggestions, "Check your network connection")
			suggestions = append(suggestions, "Increase the timeout if Jenkins is slow to respond")
		} else if errors.IsTLSError(err) {
			suggestions = append(suggestions, "Try using --insecure if the certificate is self-signed")
			suggestions = append(suggestions, "Verify the Jenkins URL scheme (http:// vs https://)")
		} else {
			suggestions = append(suggestions, "Check your network connection")
			suggestions = append(suggestions, "Verify the Jenkins URL is correct: "+c.BaseURL)
		}

		return nil, &errors.ConnectionError{
			URL:         url,
			Err:         err,
			Suggestions: suggestions,
		}
	}

	return resp, nil
}

func (c *Client) get(ctx context.Context, path string, v any) error {
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return c.classifyHTTPError(resp.StatusCode, c.buildURL(path), string(bodyBytes))
	}

	if v != nil {
		return json.NewDecoder(resp.Body).Decode(v)
	}
	return nil
}

type crumbResponse struct {
	Crumb             string `json:"crumb"`
	CrumbRequestField string `json:"crumbRequestField"`
}

func (c *Client) getCrumb(ctx context.Context) (*crumbResponse, error) {
	// Use fast timeout for lightweight crumb operation
	ctx, cancel := c.withFastTimeout(ctx)
	defer cancel()

	var crumb crumbResponse
	err := c.get(ctx, "/crumbIssuer/api/json", &crumb)
	if err != nil {
		return nil, err
	}
	return &crumb, nil
}

func (c *Client) doRequestWithCrumb(ctx context.Context, method, path string, body io.Reader, crumb *crumbResponse) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.buildURL(path), body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	c.Auth.Apply(req)

	if crumb != nil {
		req.Header.Set(crumb.CrumbRequestField, crumb.Crumb)
	}

	// Add retry logic (same as doRequest)
	resp, err := retryWithBackoff(ctx, c.MaxRetries, func() (*http.Response, error) {
		return c.HTTPClient.Do(req)
	})
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}

	return resp, nil
}

func (c *Client) postWithCrumb(ctx context.Context, path string, body io.Reader) error {
	crumb, _ := c.getCrumb(ctx)

	var resp *http.Response
	var err error

	if crumb != nil {
		resp, err = c.doRequestWithCrumb(ctx, "POST", path, body, crumb)
	} else {
		resp, err = c.doRequest(ctx, "POST", path, body)
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return c.classifyHTTPError(resp.StatusCode, c.buildURL(path), string(bodyBytes))
	}
	return nil
}

func (c *Client) postFormWithCrumb(ctx context.Context, path string, body io.Reader) error {
	crumb, _ := c.getCrumb(ctx)

	req, err := http.NewRequestWithContext(ctx, "POST", c.buildURL(path), body)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	c.Auth.Apply(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if crumb != nil {
		req.Header.Set(crumb.CrumbRequestField, crumb.Crumb)
	}

	resp, err := retryWithBackoff(ctx, c.MaxRetries, func() (*http.Response, error) {
		return c.HTTPClient.Do(req)
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return c.classifyHTTPError(resp.StatusCode, c.buildURL(path), string(bodyBytes))
	}
	return nil
}

func (c *Client) getStream(ctx context.Context, path string) (io.ReadCloser, error) {
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, c.classifyHTTPError(resp.StatusCode, c.buildURL(path), string(bodyBytes))
	}

	return resp.Body, nil
}

// TestConnection verifies connectivity to the Jenkins server.
func (c *Client) TestConnection(ctx context.Context) error {
	// Use fast timeout for lightweight connection test
	ctx, cancel := c.withFastTimeout(ctx)
	defer cancel()

	resp, err := c.doRequest(ctx, "HEAD", "/", nil)
	if err != nil {
		// Connection errors are already wrapped with helpful suggestions
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// For TestConnection, we want to provide more specific error handling
		// Read the response body for error details (though HEAD typically has no body)
		bodyBytes, _ := io.ReadAll(resp.Body)
		return c.classifyHTTPError(resp.StatusCode, c.BaseURL, string(bodyBytes))
	}
	return nil
}
