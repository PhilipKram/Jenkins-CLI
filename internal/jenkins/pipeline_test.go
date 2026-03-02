package jenkins

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidatePipelineSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/pipeline-model-converter/validate":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Errorf("expected form content type, got %s", r.Header.Get("Content-Type"))
			}
			w.Write([]byte("Jenkinsfile successfully validated.\n"))
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	result, err := c.ValidatePipeline(context.Background(), `pipeline { agent any; stages { stage("Test") { steps { echo "hello" } } } }`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "Jenkinsfile successfully validated." {
		t.Errorf("expected success message, got %q", result)
	}
}

func TestValidatePipelineFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/pipeline-model-converter/validate":
			w.Write([]byte("Errors encountered validating Jenkinsfile:\n  Expected a stage\n"))
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	result, err := c.ValidatePipeline(context.Background(), "invalid pipeline")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("expected error message, got empty string")
	}
}
