package jenkins

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type SystemInfo struct {
	Mode            string `json:"mode"`
	NodeDescription string `json:"nodeDescription"`
	NodeName        string `json:"nodeName"`
	NumExecutors    int    `json:"numExecutors"`
	UseSecurity     bool   `json:"useSecurity"`
	QuietingDown    bool   `json:"quietingDown"`
}

type WhoAmI struct {
	Name          string   `json:"name"`
	Authorities   []string `json:"authorities"`
	Authenticated bool     `json:"authenticated"`
}

func (c *Client) GetSystemInfo(ctx context.Context) (*SystemInfo, error) {
	path := "/api/json?tree=mode,nodeDescription,nodeName,numExecutors,useSecurity,quietingDown"

	var info SystemInfo
	if err := c.get(ctx, path, &info); err != nil {
		return nil, fmt.Errorf("getting system info: %w", err)
	}
	return &info, nil
}

func (c *Client) WhoAmI(ctx context.Context) (*WhoAmI, error) {
	path := "/me/api/json"

	var who WhoAmI
	if err := c.get(ctx, path, &who); err != nil {
		return nil, fmt.Errorf("getting current user: %w", err)
	}
	return &who, nil
}

func (c *Client) Restart(ctx context.Context) error {
	return c.postWithCrumb(ctx, "/restart", nil)
}

func (c *Client) SafeRestart(ctx context.Context) error {
	return c.postWithCrumb(ctx, "/safeRestart", nil)
}

func (c *Client) QuietDown(ctx context.Context) error {
	return c.postWithCrumb(ctx, "/quietDown", nil)
}

func (c *Client) CancelQuietDown(ctx context.Context) error {
	return c.postWithCrumb(ctx, "/cancelQuietDown", nil)
}

// ExecuteGroovy executes a Groovy script on the Jenkins master and returns the output.
func (c *Client) ExecuteGroovy(ctx context.Context, script string) (string, error) {
	ctx, cancel := c.withSlowTimeout(ctx)
	defer cancel()

	path := "/scriptText"
	crumb, _ := c.getCrumb(ctx)

	body := url.Values{"script": {script}}.Encode()
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
		return "", fmt.Errorf("executing groovy script: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading script output: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", c.classifyHTTPError(resp.StatusCode, c.buildURL(path), string(respBody))
	}

	return string(respBody), nil
}

// GetSystemLog retrieves the Jenkins system log.
func (c *Client) GetSystemLog(ctx context.Context) (string, error) {
	ctx, cancel := c.withSlowTimeout(ctx)
	defer cancel()

	stream, err := c.getStream(ctx, "/log/all")
	if err != nil {
		return "", fmt.Errorf("getting system log: %w", err)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("reading system log: %w", err)
	}
	return string(data), nil
}

func (c *Client) GetVersion(ctx context.Context) (string, error) {
	resp, err := c.doRequest(ctx, "HEAD", "/", nil)
	if err != nil {
		return "", fmt.Errorf("getting version: %w", err)
	}
	defer resp.Body.Close()
	return resp.Header.Get("X-Jenkins"), nil
}
