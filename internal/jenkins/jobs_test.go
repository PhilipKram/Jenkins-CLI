package jenkins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEncodeJobPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-job", "my-job"},
		{"folder/my-job", "folder/job/my-job"},
		{"a/b/c", "a/job/b/job/c"},
	}

	for _, tt := range tests {
		got := encodeJobPath(tt.input)
		if got != tt.expected {
			t.Errorf("encodeJobPath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestColorToStatus(t *testing.T) {
	tests := []struct {
		color  string
		status string
	}{
		{"blue", "SUCCESS"},
		{"blue_anime", "BUILDING"},
		{"red", "FAILURE"},
		{"red_anime", "BUILDING"},
		{"yellow", "UNSTABLE"},
		{"grey", "DISABLED"},
		{"disabled", "DISABLED"},
		{"aborted", "ABORTED"},
		{"notbuilt", "NOT_BUILT"},
	}

	for _, tt := range tests {
		got := ColorToStatus(tt.color)
		if got != tt.status {
			t.Errorf("ColorToStatus(%q) = %q, want %q", tt.color, got, tt.status)
		}
	}
}

func TestListJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jobListResponse{
			Jobs: []Job{
				{Name: "job-1", Color: "blue", Class: "hudson.model.FreeStyleProject"},
				{Name: "job-2", Color: "red", Class: "org.jenkinsci.plugins.workflow.job.WorkflowJob"},
			},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	jobs, err := c.ListJobs(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].Name != "job-1" {
		t.Errorf("expected job-1, got %s", jobs[0].Name)
	}
	if jobs[1].Name != "job-2" {
		t.Errorf("expected job-2, got %s", jobs[1].Name)
	}
}

func TestListJobsInFolder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get() appends query params; check path only
		expectedPath := "/job/my-folder/api/json"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jobListResponse{
			Jobs: []Job{{Name: "nested-job", Color: "blue"}},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	jobs, err := c.ListJobs(context.Background(), "my-folder")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != 1 || jobs[0].Name != "nested-job" {
		t.Errorf("unexpected jobs: %v", jobs)
	}
}

func TestGetJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(JobDetail{
			Job: Job{
				Name:     "my-job",
				Color:    "blue",
				FullName: "my-job",
			},
			LastBuild:       &BuildRef{Number: 42},
			NextBuildNumber: 43,
			InQueue:         false,
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	job, err := c.GetJob(context.Background(), "my-job")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if job.Name != "my-job" {
		t.Errorf("expected my-job, got %s", job.Name)
	}
	if job.LastBuild.Number != 42 {
		t.Errorf("expected last build 42, got %d", job.LastBuild.Number)
	}
}

func TestBuildJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/job/my-job/build":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.BuildJob(context.Background(), "my-job", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildJobWithParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/job/my-job/buildWithParameters":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			branch := r.URL.Query().Get("BRANCH")
			if branch != "main" {
				t.Errorf("expected BRANCH=main, got %s", branch)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.BuildJob(context.Background(), "my-job", map[string]string{"BRANCH": "main"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetJobConfig(t *testing.T) {
	xmlConfig := `<?xml version='1.0' encoding='UTF-8'?><project><description>test</description></project>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/job/my-job/config.xml" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(xmlConfig))
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	config, err := c.GetJobConfig(context.Background(), "my-job")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config != xmlConfig {
		t.Errorf("expected XML config, got %q", config)
	}
}

func TestUpdateJobConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/job/my-job/config.xml":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/xml" {
				t.Errorf("expected application/xml, got %s", r.Header.Get("Content-Type"))
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.UpdateJobConfig(context.Background(), "my-job", "<project/>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateJob(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/createItem":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Query().Get("name") != "new-job" {
				t.Errorf("expected name=new-job, got %s", r.URL.Query().Get("name"))
			}
			if r.Header.Get("Content-Type") != "application/xml" {
				t.Errorf("expected application/xml, got %s", r.Header.Get("Content-Type"))
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.CreateJob(context.Background(), "new-job", "<project/>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
