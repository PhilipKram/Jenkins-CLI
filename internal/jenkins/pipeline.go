package jenkins

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ValidatePipeline validates a Jenkinsfile via the Jenkins pipeline model converter API.
// Returns the validation result text. An empty or success message indicates a valid pipeline.
func (c *Client) ValidatePipeline(ctx context.Context, jenkinsfile string) (string, error) {
	ctx, cancel := c.withStandardTimeout(ctx)
	defer cancel()

	path := "/pipeline-model-converter/validate"
	crumb, _ := c.getCrumb(ctx)

	body := url.Values{"jenkinsfile": {jenkinsfile}}.Encode()
	req, err := http.NewRequestWithContext(ctx, "POST", c.buildURL(path), strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
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
		return "", fmt.Errorf("validating pipeline: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading validation response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", c.classifyHTTPError(resp.StatusCode, c.buildURL(path), string(respBody))
	}

	return strings.TrimSpace(string(respBody)), nil
}
