package jenkins

import (
	"context"
	"fmt"
)

type QueueItem struct {
	ID        int    `json:"id"`
	Task      Task   `json:"task"`
	Why       string `json:"why"`
	Stuck     bool   `json:"stuck"`
	Buildable bool   `json:"buildable"`
	Blocked   bool   `json:"blocked"`
	InQueueSince int64 `json:"inQueueSince"`
}

type Task struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Color string `json:"color"`
}

type queueResponse struct {
	Items []QueueItem `json:"items"`
}

func (c *Client) GetQueue(ctx context.Context) ([]QueueItem, error) {
	path := "/queue/api/json?tree=items[id,task[name,url,color],why,stuck,buildable,blocked,inQueueSince]"

	var resp queueResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("getting queue: %w", err)
	}
	return resp.Items, nil
}

func (c *Client) CancelQueueItem(ctx context.Context, id int) error {
	path := fmt.Sprintf("/queue/cancelItem?id=%d", id)
	return c.postWithCrumb(ctx, path, nil)
}
