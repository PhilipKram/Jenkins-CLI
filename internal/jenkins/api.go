package jenkins

import (
	"context"
	"io"
	"net/http"
)

// RawRequest performs an authenticated HTTP request to any Jenkins endpoint.
// For POST/PUT/DELETE methods, a CSRF crumb is automatically fetched and included.
// The caller is responsible for closing the response body.
func (c *Client) RawRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	if method == "GET" || method == "HEAD" {
		return c.doRequest(ctx, method, path, body)
	}

	// For mutating methods, include CSRF crumb
	crumb, _ := c.getCrumb(ctx)
	if crumb != nil {
		return c.doRequestWithCrumb(ctx, method, path, body, crumb)
	}
	return c.doRequest(ctx, method, path, body)
}
