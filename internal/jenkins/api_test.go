package jenkins

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRawRequestGET(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Write([]byte(`{"mode":"NORMAL"}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	resp, err := c.RawRequest(context.Background(), "GET", "/api/json", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"mode":"NORMAL"}` {
		t.Errorf("unexpected body: %s", string(body))
	}
}

func TestRawRequestPOSTWithCrumb(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"crumb":"test-crumb","crumbRequestField":"Jenkins-Crumb"}`))
		case "/job/my-job/build":
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Jenkins-Crumb") != "test-crumb" {
				t.Errorf("expected crumb header, got %q", r.Header.Get("Jenkins-Crumb"))
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	resp, err := c.RawRequest(context.Background(), "POST", "/job/my-job/build", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
}
