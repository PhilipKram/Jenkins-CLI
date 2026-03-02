package jenkins

import (
	"context"
	"fmt"
	"net/url"
)

type Node struct {
	DisplayName     string       `json:"displayName"`
	Description     string       `json:"description"`
	Idle            bool         `json:"idle"`
	Offline         bool         `json:"offline"`
	OfflineCause    *OfflineCause `json:"offlineCause"`
	TemporarilyOffline bool      `json:"temporarilyOffline"`
	NumExecutors    int          `json:"numExecutors"`
	MonitorData     map[string]any `json:"monitorData"`
}

type OfflineCause struct {
	Class  string `json:"_class"`
	Reason string `json:"description"`
}

type nodeListResponse struct {
	Computer []Node `json:"computer"`
}

func (c *Client) ListNodes(ctx context.Context) ([]Node, error) {
	path := "/computer/api/json?tree=computer[displayName,description,idle,offline,offlineCause[_class,description],temporarilyOffline,numExecutors]"

	var resp nodeListResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("listing nodes: %w", err)
	}
	return resp.Computer, nil
}

func (c *Client) GetNode(ctx context.Context, name string) (*Node, error) {
	displayName := name
	if name == "master" || name == "built-in" {
		name = "(built-in)"
	}

	nodes, err := c.ListNodes(ctx)
	if err != nil {
		return nil, err
	}

	for _, n := range nodes {
		if n.DisplayName == name || n.DisplayName == displayName {
			return &n, nil
		}
	}
	return nil, fmt.Errorf("node %q not found", name)
}

func (c *Client) ToggleNodeOffline(ctx context.Context, name string, offline bool, message string) error {
	if name == "master" || name == "built-in" {
		name = "(built-in)"
	}

	action := "toggleOffline"
	path := fmt.Sprintf("/computer/%s/%s?offlineMessage=%s", name, action, url.QueryEscape(message))
	return c.postWithCrumb(ctx, path, nil)
}

func (n *Node) StatusStr() string {
	if n.Offline {
		if n.TemporarilyOffline {
			return "TEMPORARILY_OFFLINE"
		}
		return "OFFLINE"
	}
	if n.Idle {
		return "IDLE"
	}
	return "BUSY"
}
