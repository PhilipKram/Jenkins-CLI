package jenkins

import (
	"context"
	"fmt"
	"io"
)

// ScanMultibranchPipeline triggers a branch indexing scan for a multibranch pipeline.
// For multibranch projects, the /build endpoint triggers a scan rather than a build.
func (c *Client) ScanMultibranchPipeline(ctx context.Context, name string) error {
	path := "/job/" + encodeJobPath(name) + "/build"
	return c.wrapNotFoundError(c.postWithCrumb(ctx, path, nil), name)
}

// GetScanLog retrieves the branch indexing log for a multibranch pipeline.
func (c *Client) GetScanLog(ctx context.Context, name string) (string, error) {
	ctx, cancel := c.withSlowTimeout(ctx)
	defer cancel()

	path := "/job/" + encodeJobPath(name) + "/indexing/consoleText"
	stream, err := c.getStream(ctx, path)
	if err != nil {
		return "", c.wrapNotFoundError(fmt.Errorf("getting scan log: %w", err), name)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("reading scan log: %w", err)
	}
	return string(data), nil
}
