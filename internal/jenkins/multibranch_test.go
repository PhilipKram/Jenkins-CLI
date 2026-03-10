package jenkins

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestScanMultibranchPipeline(t *testing.T) {
	var gotMethod, gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 0)
	err := c.ScanMultibranchPipeline(context.Background(), "my-multibranch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotMethod != "POST" {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/job/my-multibranch/build" {
		t.Errorf("expected /job/my-multibranch/build, got %s", gotPath)
	}
}

func TestScanMultibranchPipelineWithFolder(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 0)
	err := c.ScanMultibranchPipeline(context.Background(), "folder/my-multibranch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != "/job/folder/job/my-multibranch/build" {
		t.Errorf("expected /job/folder/job/my-multibranch/build, got %s", gotPath)
	}
}

func TestGetScanLog(t *testing.T) {
	expectedLog := "Started\nChecking branch main\nChecking branch develop\nFinished"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/job/my-multibranch/indexing/consoleText" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(expectedLog))
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 0)
	log, err := c.GetScanLog(context.Background(), "my-multibranch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if log != expectedLog {
		t.Errorf("expected %q, got %q", expectedLog, log)
	}
}

func TestListBranches(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/job/my-multibranch/api/json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jobListResponse{
			Jobs: []Job{
				{Name: "main", Color: "blue", Class: "org.jenkinsci.plugins.workflow.job.WorkflowJob"},
				{Name: "develop", Color: "blue", Class: "org.jenkinsci.plugins.workflow.job.WorkflowJob"},
				{Name: "feature/login", Color: "red", Class: "org.jenkinsci.plugins.workflow.job.WorkflowJob"},
			},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 0)
	branches, err := c.ListJobs(context.Background(), "my-multibranch")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(branches) != 3 {
		t.Fatalf("expected 3 branches, got %d", len(branches))
	}
	if branches[0].Name != "main" {
		t.Errorf("expected main, got %s", branches[0].Name)
	}
	if branches[1].Name != "develop" {
		t.Errorf("expected develop, got %s", branches[1].Name)
	}
	if branches[2].Name != "feature/login" {
		t.Errorf("expected feature/login, got %s", branches[2].Name)
	}
}
