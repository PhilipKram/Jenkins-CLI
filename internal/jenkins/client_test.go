package jenkins

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:8080", "admin", "token123", false, 30*time.Second, 3)

	if c.BaseURL != "http://localhost:8080" {
		t.Errorf("expected BaseURL http://localhost:8080, got %s", c.BaseURL)
	}
	basic, ok := c.Auth.(*BasicAuth)
	if !ok {
		t.Fatal("expected BasicAuth type")
	}
	if basic.User != "admin" {
		t.Errorf("expected User admin, got %s", basic.User)
	}
	if basic.Token != "token123" {
		t.Errorf("expected Token token123, got %s", basic.Token)
	}
}

func TestNewClientWithBearerAuth(t *testing.T) {
	c := NewClientWithAuth("http://localhost:8080", &BearerTokenAuth{Token: "my-token"}, false, 30*time.Second, 3)

	if c.BaseURL != "http://localhost:8080" {
		t.Errorf("expected BaseURL http://localhost:8080, got %s", c.BaseURL)
	}
	bearer, ok := c.Auth.(*BearerTokenAuth)
	if !ok {
		t.Fatal("expected BearerTokenAuth type")
	}
	if bearer.Token != "my-token" {
		t.Errorf("expected Token my-token, got %s", bearer.Token)
	}
}

func TestBearerTokenAuthApply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-bearer-token" {
			t.Errorf("expected Bearer header, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"mode": "NORMAL"})
	}))
	defer server.Close()

	c := NewClientWithAuth(server.URL, &BearerTokenAuth{Token: "test-bearer-token"}, false, 30*time.Second, 3)
	var result map[string]string
	err := c.get(context.Background(), "/api/json", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["mode"] != "NORMAL" {
		t.Errorf("expected mode NORMAL, got %s", result["mode"])
	}
}

func TestAuthMethodString(t *testing.T) {
	if (&BasicAuth{}).String() != "basic" {
		t.Error("BasicAuth.String() should return 'basic'")
	}
	if (&BearerTokenAuth{}).String() != "bearer" {
		t.Error("BearerTokenAuth.String() should return 'bearer'")
	}
}

func TestNewClientTrimsTrailingSlash(t *testing.T) {
	c := NewClient("http://localhost:8080/", "admin", "token123", false, 30*time.Second, 3)
	if c.BaseURL != "http://localhost:8080" {
		t.Errorf("expected trailing slash trimmed, got %s", c.BaseURL)
	}
}

func TestStripJobPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://localhost:8080", "http://localhost:8080"},
		{"http://localhost:8080/jenkins", "http://localhost:8080/jenkins"},
		{"http://localhost:8080/job/myproject", "http://localhost:8080"},
		{"http://localhost:8080/ssbu-01/job/allot_secure_team_multiproject", "http://localhost:8080/ssbu-01"},
		{"https://jenkins.example.com/prefix/job/folder/job/pipeline", "https://jenkins.example.com/prefix"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := stripJobPath(tt.input)
			if got != tt.want {
				t.Errorf("stripJobPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewClientStripsJobPath(t *testing.T) {
	c := NewClient("https://jenkins.example.com/ssbu-01/job/myproject/", "admin", "token", false, 30*time.Second, 3)
	want := "https://jenkins.example.com/ssbu-01"
	if c.BaseURL != want {
		t.Errorf("expected BaseURL %s, got %s", want, c.BaseURL)
	}
}

func TestGetJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		// Verify basic auth
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "token" {
			t.Error("expected basic auth")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"mode": "NORMAL"})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)

	var result map[string]string
	err := c.get(context.Background(), "/api/json", &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["mode"] != "NORMAL" {
		t.Errorf("expected mode NORMAL, got %s", result["mode"])
	}
}

func TestGetHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)

	var result map[string]string
	err := c.get(context.Background(), "/api/json", &result)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestPostWithCrumb(t *testing.T) {
	postCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(crumbResponse{
				Crumb:             "test-crumb",
				CrumbRequestField: "Jenkins-Crumb",
			})
		case "/job/test/build":
			postCalled = true
			crumb := r.Header.Get("Jenkins-Crumb")
			if crumb != "test-crumb" {
				t.Errorf("expected crumb header, got %q", crumb)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.postWithCrumb(context.Background(), "/job/test/build", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !postCalled {
		t.Error("expected POST to /job/test/build")
	}
}

func TestPostWithCrumbDisabled(t *testing.T) {
	postCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/job/test/build":
			postCalled = true
			w.WriteHeader(http.StatusCreated)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.postWithCrumb(context.Background(), "/job/test/build", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !postCalled {
		t.Error("expected POST to /job/test/build")
	}
}

func TestPostWithCrumbRetry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(crumbResponse{
				Crumb:             "test-crumb",
				CrumbRequestField: "Jenkins-Crumb",
			})
		case "/job/test/build":
			callCount++
			if callCount < 3 {
				// Return transient error for first 2 attempts
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			// Success on 3rd attempt
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	err := c.postWithCrumb(context.Background(), "/job/test/build", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 attempts (2 retries + 1 success), got %d", callCount)
	}
}

func TestGetStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("console output line 1\nconsole output line 2\n"))
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	reader, err := c.getStream(context.Background(), "/job/test/1/consoleText")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.Close()

	buf := make([]byte, 1024)
	n, _ := reader.Read(buf)
	if string(buf[:n]) != "console output line 1\nconsole output line 2\n" {
		t.Errorf("unexpected output: %s", string(buf[:n]))
	}
}

// TestTransientErrorDetection tests the isTransientError function for various error types
func TestTransientErrorDetection(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		statusCode int
		want       bool
	}{
		{
			name:       "502 Bad Gateway is transient",
			err:        nil,
			statusCode: http.StatusBadGateway,
			want:       true,
		},
		{
			name:       "503 Service Unavailable is transient",
			err:        nil,
			statusCode: http.StatusServiceUnavailable,
			want:       true,
		},
		{
			name:       "504 Gateway Timeout is transient",
			err:        nil,
			statusCode: http.StatusGatewayTimeout,
			want:       true,
		},
		{
			name:       "404 Not Found is not transient",
			err:        nil,
			statusCode: http.StatusNotFound,
			want:       false,
		},
		{
			name:       "401 Unauthorized is not transient",
			err:        nil,
			statusCode: http.StatusUnauthorized,
			want:       false,
		},
		{
			name:       "200 OK is not transient",
			err:        nil,
			statusCode: http.StatusOK,
			want:       false,
		},
		{
			name:       "network error is transient",
			err:        &net.OpError{Op: "dial", Err: &net.DNSError{IsTimeout: true}},
			statusCode: 0,
			want:       true,
		},
		{
			name:       "connection refused is transient",
			err:        &net.OpError{Op: "dial", Net: "tcp", Err: context.DeadlineExceeded},
			statusCode: 0,
			want:       true,
		},
		{
			name:       "no error and 200 status is not transient",
			err:        nil,
			statusCode: 200,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientError(tt.err, tt.statusCode)
			if got != tt.want {
				t.Errorf("isTransientError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRetryWithBackoffSuccess tests successful request without retries
func TestRetryWithBackoffSuccess(t *testing.T) {
	callCount := 0
	operation := func() (*http.Response, error) {
		callCount++
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	ctx := context.Background()
	resp, err := retryWithBackoff(ctx, 3, operation)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

// TestRetryWithBackoffTransientError tests retry behavior with transient errors
func TestRetryWithBackoffTransientError(t *testing.T) {
	callCount := 0
	operation := func() (*http.Response, error) {
		callCount++
		if callCount < 3 {
			// Return transient error for first 2 attempts
			return &http.Response{StatusCode: http.StatusServiceUnavailable}, nil
		}
		// Succeed on 3rd attempt
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	ctx := context.Background()
	resp, err := retryWithBackoff(ctx, 3, operation)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

// TestRetryWithBackoffNonTransientError tests that non-transient errors are not retried
func TestRetryWithBackoffNonTransientError(t *testing.T) {
	callCount := 0
	operation := func() (*http.Response, error) {
		callCount++
		return &http.Response{StatusCode: http.StatusNotFound}, nil
	}

	ctx := context.Background()
	resp, err := retryWithBackoff(ctx, 3, operation)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call (no retries), got %d", callCount)
	}
}

// TestRetryWithBackoffMaxRetriesExceeded tests that retries stop after maxRetries
func TestRetryWithBackoffMaxRetriesExceeded(t *testing.T) {
	callCount := 0
	operation := func() (*http.Response, error) {
		callCount++
		// Always return transient error
		return &http.Response{StatusCode: http.StatusBadGateway}, nil
	}

	ctx := context.Background()
	maxRetries := 2
	resp, err := retryWithBackoff(ctx, maxRetries, operation)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected status 502, got %d", resp.StatusCode)
	}
	// Should attempt initial + maxRetries times = 3 total
	expectedCalls := maxRetries + 1
	if callCount != expectedCalls {
		t.Errorf("expected %d calls, got %d", expectedCalls, callCount)
	}
}

// TestRetryWithBackoffContextCancellation tests that retries stop when context is cancelled
func TestRetryWithBackoffContextCancellation(t *testing.T) {
	callCount := 0
	ctx, cancel := context.WithCancel(context.Background())

	operation := func() (*http.Response, error) {
		callCount++
		if callCount == 2 {
			// Cancel context on second call
			cancel()
		}
		return &http.Response{StatusCode: http.StatusServiceUnavailable}, nil
	}

	_, err := retryWithBackoff(ctx, 5, operation)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
	// Should stop after context cancellation
	if callCount > 3 {
		t.Errorf("expected at most 3 calls, got %d", callCount)
	}
}

// TestRetryWithBackoffNetworkError tests retry behavior with network errors
func TestRetryWithBackoffNetworkError(t *testing.T) {
	callCount := 0
	operation := func() (*http.Response, error) {
		callCount++
		if callCount < 2 {
			// Return network error on first attempt
			return nil, &net.OpError{Op: "dial", Err: &net.DNSError{IsTimeout: true}}
		}
		// Succeed on second attempt
		return &http.Response{StatusCode: http.StatusOK}, nil
	}

	ctx := context.Background()
	resp, err := retryWithBackoff(ctx, 3, operation)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls, got %d", callCount)
	}
}

// TestRetryIntegration tests the full retry flow through the client
func TestRetryIntegration(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 2 {
			// Return 503 on first attempt
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		// Succeed on second attempt
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 2)

	var result map[string]string
	err := c.get(context.Background(), "/api/json", &result)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status ok, got %s", result["status"])
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", callCount)
	}
}
