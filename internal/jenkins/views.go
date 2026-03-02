package jenkins

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// View represents a Jenkins view with basic information.
type View struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Class       string `json:"_class"`
	Jobs        []Job  `json:"jobs"`
}

// ViewDetail represents a detailed Jenkins view including its jobs.
type ViewDetail struct {
	View
	Jobs     []Job  `json:"jobs"`
	Property []any  `json:"property"`
}

// viewListResponse is the response structure from Jenkins API for listing views.
type viewListResponse struct {
	Views []View `json:"views"`
}

// ListViews returns all views from Jenkins.
func (c *Client) ListViews(ctx context.Context) ([]View, error) {
	path := "/api/json?tree=views[name,url,description,_class,jobs[name]]"

	var resp viewListResponse
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("listing views: %w", err)
	}
	return resp.Views, nil
}

// GetView returns detailed information about a specific view.
func (c *Client) GetView(ctx context.Context, name string) (*ViewDetail, error) {
	path := "/view/" + url.PathEscape(name) + "/api/json"
	var view ViewDetail
	if err := c.get(ctx, path, &view); err != nil {
		return nil, c.wrapNotFoundError(err, name)
	}
	return &view, nil
}

// CreateView creates a new view with the given name and XML configuration.
func (c *Client) CreateView(ctx context.Context, name string, configXML string) error {
	path := "/createView?name=" + url.QueryEscape(name)
	body := strings.NewReader(configXML)
	return c.postWithCrumb(ctx, path, body)
}

// DeleteView deletes the specified view.
func (c *Client) DeleteView(ctx context.Context, name string) error {
	path := "/view/" + url.PathEscape(name) + "/doDelete"
	return c.wrapNotFoundError(c.postWithCrumb(ctx, path, nil), name)
}

// AddJobToView adds a job to the specified view.
func (c *Client) AddJobToView(ctx context.Context, viewName, jobName string) error {
	path := "/view/" + url.PathEscape(viewName) + "/addJobToView?name=" + url.QueryEscape(jobName)
	return c.wrapNotFoundError(c.postWithCrumb(ctx, path, nil), viewName)
}

// RemoveJobFromView removes a job from the specified view.
func (c *Client) RemoveJobFromView(ctx context.Context, viewName, jobName string) error {
	path := "/view/" + url.PathEscape(viewName) + "/removeJobFromView?name=" + url.QueryEscape(jobName)
	return c.wrapNotFoundError(c.postWithCrumb(ctx, path, nil), viewName)
}
