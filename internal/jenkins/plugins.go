package jenkins

import (
	"context"
	"fmt"
)

type Plugin struct {
	ShortName    string `json:"shortName"`
	LongName     string `json:"longName"`
	Version      string `json:"version"`
	Active       bool   `json:"active"`
	Enabled      bool   `json:"enabled"`
	HasUpdate    bool   `json:"hasUpdate"`
	URL          string `json:"url"`
}

type pluginResponse struct {
	Plugins []Plugin `json:"plugins"`
}

func (c *Client) ListPlugins(ctx context.Context) ([]Plugin, error) {
	path := "/pluginManager/api/json?tree=plugins[shortName,longName,version,active,enabled,hasUpdate,url]&depth=1"

	var resp pluginResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("listing plugins: %w", err)
	}
	return resp.Plugins, nil
}
