package jenkins

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStreamBuildLog(t *testing.T) {
	// Test successful streaming with multiple polls
	t.Run("successful streaming", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			expectedPath := "/job/my-job/42/logText/progressiveText"
			if !strings.HasPrefix(r.URL.Path, expectedPath) {
				t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
			}

			callCount++
			switch callCount {
			case 1:
				// First poll: send some data, more to come
				w.Header().Set("X-More-Data", "true")
				w.Header().Set("X-Text-Size", "12")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Build log 1\n"))
			case 2:
				// Second poll: send more data, more to come
				start := r.URL.Query().Get("start")
				if start != "12" {
					t.Errorf("unexpected start offset: got %s, want 12", start)
				}
				w.Header().Set("X-More-Data", "true")
				w.Header().Set("X-Text-Size", "24")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Build log 2\n"))
			case 3:
				// Third poll: send final data, no more
				start := r.URL.Query().Get("start")
				if start != "24" {
					t.Errorf("unexpected start offset: got %s, want 24", start)
				}
				w.Header().Set("X-More-Data", "false")
				w.Header().Set("X-Text-Size", "36")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Build log 3\n"))
			default:
				t.Errorf("unexpected call count: %d", callCount)
			}
		}))
		defer server.Close()

		c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
		var buf bytes.Buffer
		err := c.StreamBuildLog(context.Background(), "my-job", 42, &buf, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := "Build log 1\nBuild log 2\nBuild log 3\n"
		if buf.String() != expected {
			t.Errorf("unexpected output: got %q, want %q", buf.String(), expected)
		}

		if callCount != 3 {
			t.Errorf("unexpected call count: got %d, want 3", callCount)
		}
	})

	// Test build completes on first poll
	t.Run("build completes immediately", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-More-Data", "false")
			w.Header().Set("X-Text-Size", "20")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Complete build log\n"))
		}))
		defer server.Close()

		c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
		var buf bytes.Buffer
		err := c.StreamBuildLog(context.Background(), "my-job", 42, &buf, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := "Complete build log\n"
		if buf.String() != expected {
			t.Errorf("unexpected output: got %q, want %q", buf.String(), expected)
		}
	})

	// Test empty log content
	t.Run("empty log content", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-More-Data", "false")
			w.Header().Set("X-Text-Size", "0")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
		var buf bytes.Buffer
		err := c.StreamBuildLog(context.Background(), "my-job", 42, &buf, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if buf.String() != "" {
			t.Errorf("expected empty output, got %q", buf.String())
		}
	})

	// Test HTTP error handling
	t.Run("HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("build not found"))
		}))
		defer server.Close()

		c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
		var buf bytes.Buffer
		err := c.StreamBuildLog(context.Background(), "my-job", 42, &buf, 10*time.Millisecond)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !strings.Contains(err.Error(), "HTTP 404") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// Test write error handling
	t.Run("write error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-More-Data", "false")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("log content"))
		}))
		defer server.Close()

		c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
		errWriter := &errorWriter{err: errors.New("write failed")}
		err := c.StreamBuildLog(context.Background(), "my-job", 42, errWriter, 10*time.Millisecond)
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if !strings.Contains(err.Error(), "writing log content") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// Test default poll interval
	t.Run("default poll interval", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			if callCount == 1 {
				w.Header().Set("X-More-Data", "true")
				w.Header().Set("X-Text-Size", "5")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("log\n"))
			} else {
				w.Header().Set("X-More-Data", "false")
				w.WriteHeader(http.StatusOK)
			}
		}))
		defer server.Close()

		c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
		var buf bytes.Buffer
		// Pass 0 to test default interval
		err := c.StreamBuildLog(context.Background(), "my-job", 42, &buf, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if callCount != 2 {
			t.Errorf("unexpected call count: got %d, want 2", callCount)
		}
	})

	// Test offset tracking with missing X-Text-Size header
	t.Run("missing X-Text-Size header", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			start := r.URL.Query().Get("start")

			switch callCount {
			case 1:
				if start != "0" {
					t.Errorf("unexpected start: got %s, want 0", start)
				}
				w.Header().Set("X-More-Data", "true")
				// Missing X-Text-Size header
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("log 1\n"))
			case 2:
				// Should still use offset 0 since X-Text-Size was missing
				if start != "0" {
					t.Errorf("unexpected start: got %s, want 0", start)
				}
				w.Header().Set("X-More-Data", "false")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("log 2\n"))
			}
		}))
		defer server.Close()

		c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
		var buf bytes.Buffer
		err := c.StreamBuildLog(context.Background(), "my-job", 42, &buf, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Test job name encoding
	t.Run("job name with special characters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// encodeJobPath converts "my/project/my-job" to "my/job/project/job/my-job"
			expectedPath := "/job/my/job/project/job/my-job/42/logText/progressiveText"
			if !strings.HasPrefix(r.URL.Path, expectedPath) {
				t.Errorf("unexpected path: got %s, want prefix %s", r.URL.Path, expectedPath)
			}
			w.Header().Set("X-More-Data", "false")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
		var buf bytes.Buffer
		err := c.StreamBuildLog(context.Background(), "my/project/my-job", 42, &buf, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Test context cancellation
	t.Run("context cancellation", func(t *testing.T) {
		callCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount++
			// Always return "more data" to simulate long-running build
			w.Header().Set("X-More-Data", "true")
			w.Header().Set("X-Text-Size", fmt.Sprintf("%d", callCount*10))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("Log line %d\n", callCount)))
		}))
		defer server.Close()

		c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel context after first response
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		var buf bytes.Buffer
		err := c.StreamBuildLog(ctx, "my-job", 42, &buf, 20*time.Millisecond)

		// Should return nil (graceful cancellation, no error)
		if err != nil {
			t.Errorf("expected nil on context cancellation, got: %v", err)
		}

		// Should have received at least one log line before cancellation
		if buf.Len() == 0 {
			t.Error("expected some output before cancellation")
		}

		// Should have stopped after cancellation (not made many calls)
		if callCount > 3 {
			t.Errorf("expected streaming to stop quickly, but made %d calls", callCount)
		}
	})
}

// errorWriter is a test helper that always returns an error on Write
type errorWriter struct {
	err error
}

func (w *errorWriter) Write(p []byte) (n int, err error) {
	return 0, w.err
}

func TestStreamBuildLogIntegration(t *testing.T) {
	// Simulate a more realistic streaming scenario
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := r.URL.Query().Get("start")

		switch start {
		case "0":
			// Initial poll
			w.Header().Set("X-More-Data", "true")
			w.Header().Set("X-Text-Size", "50")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "Starting build...\n")
			fmt.Fprint(w, "Fetching dependencies...\n")
		case "50":
			// Second poll
			w.Header().Set("X-More-Data", "true")
			w.Header().Set("X-Text-Size", "89")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "Running tests...\n")
			fmt.Fprint(w, "All tests passed!\n")
		case "89":
			// Final poll
			w.Header().Set("X-More-Data", "false")
			w.Header().Set("X-Text-Size", "107")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "Build complete!\n")
		default:
			t.Errorf("unexpected start offset: %s", start)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	var buf bytes.Buffer
	err := c.StreamBuildLog(context.Background(), "test-job", 1, &buf, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	expectedLines := []string{
		"Starting build...",
		"Fetching dependencies...",
		"Running tests...",
		"All tests passed!",
		"Build complete!",
	}

	for _, line := range expectedLines {
		if !strings.Contains(output, line) {
			t.Errorf("output missing expected line: %q", line)
		}
	}
}
