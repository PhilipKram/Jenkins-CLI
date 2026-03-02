package jenkins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestListViews(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(viewListResponse{
			Views: []View{
				{Name: "All", URL: "http://jenkins/view/All/", Description: "All jobs", Class: "hudson.model.AllView"},
				{Name: "My View", URL: "http://jenkins/view/My%20View/", Description: "Custom view", Class: "hudson.model.ListView"},
			},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	views, err := c.ListViews(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(views) != 2 {
		t.Fatalf("expected 2 views, got %d", len(views))
	}
	if views[0].Name != "All" {
		t.Errorf("expected All, got %s", views[0].Name)
	}
	if views[1].Name != "My View" {
		t.Errorf("expected My View, got %s", views[1].Name)
	}
}

func TestGetView(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/view/My View/api/json"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ViewDetail{
			View: View{
				Name:        "My View",
				URL:         "http://jenkins/view/My%20View/",
				Description: "Test view",
				Class:       "hudson.model.ListView",
			},
			Jobs: []Job{
				{Name: "job-1", Color: "blue", Class: "hudson.model.FreeStyleProject"},
				{Name: "job-2", Color: "red", Class: "org.jenkinsci.plugins.workflow.job.WorkflowJob"},
			},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	view, err := c.GetView(context.Background(), "My View")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if view.Name != "My View" {
		t.Errorf("expected My View, got %s", view.Name)
	}
	if len(view.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(view.Jobs))
	}
	if view.Jobs[0].Name != "job-1" {
		t.Errorf("expected job-1, got %s", view.Jobs[0].Name)
	}
}

func TestCreateView(t *testing.T) {
	configXML := `<?xml version="1.0" encoding="UTF-8"?>
<hudson.model.ListView>
  <name>Test View</name>
  <description>Test Description</description>
</hudson.model.ListView>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/createView":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			viewName := r.URL.Query().Get("name")
			if viewName != "Test View" {
				t.Errorf("expected name=Test View, got %s", viewName)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.CreateView(context.Background(), "Test View", configXML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateViewWithCrumb(t *testing.T) {
	configXML := `<?xml version="1.0" encoding="UTF-8"?>
<hudson.model.ListView>
  <name>New View</name>
</hudson.model.ListView>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"crumb":             "test-crumb-value",
				"crumbRequestField": "Jenkins-Crumb",
			})
		case "/createView":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			crumb := r.Header.Get("Jenkins-Crumb")
			if crumb != "test-crumb-value" {
				t.Errorf("expected crumb test-crumb-value, got %s", crumb)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.CreateView(context.Background(), "New View", configXML)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteView(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/view/Test View/doDelete":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.DeleteView(context.Background(), "Test View")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteViewNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/view/NonExistent/doDelete":
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.DeleteView(context.Background(), "NonExistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAddJobToView(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/view/My View/addJobToView":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			jobName := r.URL.Query().Get("name")
			if jobName != "my-job" {
				t.Errorf("expected name=my-job, got %s", jobName)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.AddJobToView(context.Background(), "My View", "my-job")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddJobToViewWithSpaces(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/view/Test View/addJobToView":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			jobName := r.URL.Query().Get("name")
			if jobName != "Test Job" {
				t.Errorf("expected name=Test Job, got %s", jobName)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.AddJobToView(context.Background(), "Test View", "Test Job")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveJobFromView(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/view/My View/removeJobFromView":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			jobName := r.URL.Query().Get("name")
			if jobName != "my-job" {
				t.Errorf("expected name=my-job, got %s", jobName)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.RemoveJobFromView(context.Background(), "My View", "my-job")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveJobFromViewNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/view/NonExistent/removeJobFromView":
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.RemoveJobFromView(context.Background(), "NonExistent", "my-job")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
