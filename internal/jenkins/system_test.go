package jenkins

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExecuteGroovy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/scriptText":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Errorf("expected form content type, got %s", r.Header.Get("Content-Type"))
			}
			r.ParseForm()
			script := r.FormValue("script")
			if script != "println 'hello'" {
				t.Errorf("unexpected script: %q", script)
			}
			w.Write([]byte("hello\n"))
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	result, err := c.ExecuteGroovy(context.Background(), "println 'hello'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", result)
	}
}

func TestExecuteGroovyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/scriptText":
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("permission denied"))
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	_, err := c.ExecuteGroovy(context.Background(), "bad script")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetSystemLog(t *testing.T) {
	logContent := "2024-01-01 INFO Started\n2024-01-01 INFO Ready\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/log/all" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(logContent))
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	result, err := c.GetSystemLog(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "Started") {
		t.Errorf("expected log to contain 'Started', got %q", result)
	}
}
