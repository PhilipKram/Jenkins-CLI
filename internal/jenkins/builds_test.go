package jenkins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBuildStatus(t *testing.T) {
	tests := []struct {
		building bool
		result   string
		expected string
	}{
		{true, "", "BUILDING"},
		{false, "SUCCESS", "SUCCESS"},
		{false, "FAILURE", "FAILURE"},
		{false, "", "UNKNOWN"},
	}

	for _, tt := range tests {
		b := Build{Building: tt.building, Result: tt.result}
		if got := b.Status(); got != tt.expected {
			t.Errorf("Build{building=%v, result=%q}.Status() = %q, want %q",
				tt.building, tt.result, got, tt.expected)
		}
	}
}

func TestBuildStartTime(t *testing.T) {
	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC).UnixMilli()
	b := Build{Timestamp: ts}
	got := b.StartTime()
	if got.Year() != 2025 || got.Month() != 1 || got.Day() != 15 {
		t.Errorf("unexpected start time: %v", got)
	}
}

func TestBuildDurationStr(t *testing.T) {
	tests := []struct {
		duration int64
		building bool
		expected string
	}{
		{30000, false, "30s"},
		{90000, false, "1m 30s"},
		{3660000, false, "1h 1m"},
	}

	for _, tt := range tests {
		b := Build{Duration: tt.duration, Building: tt.building}
		if got := b.DurationStr(); got != tt.expected {
			t.Errorf("Build{duration=%d}.DurationStr() = %q, want %q",
				tt.duration, got, tt.expected)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{45 * time.Second, "45s"},
		{2*time.Minute + 30*time.Second, "2m 30s"},
		{1*time.Hour + 15*time.Minute, "1h 15m"},
	}

	for _, tt := range tests {
		if got := formatDuration(tt.d); got != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.expected)
		}
	}
}

func TestListBuilds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(buildListResponse{
			Builds: []Build{
				{Number: 10, Result: "SUCCESS", Duration: 30000},
				{Number: 9, Result: "FAILURE", Duration: 45000},
			},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	builds, err := c.ListBuilds(context.Background(), "my-job", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(builds) != 2 {
		t.Fatalf("expected 2 builds, got %d", len(builds))
	}
	if builds[0].Number != 10 {
		t.Errorf("expected build 10, got %d", builds[0].Number)
	}
}

func TestGetBuild(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/job/my-job/5/api/json"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildDetail{
			Build: Build{Number: 5, Result: "SUCCESS"},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 30*time.Second, 3)
	build, err := c.GetBuild(context.Background(), "my-job", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if build.Number != 5 {
		t.Errorf("expected build 5, got %d", build.Number)
	}
}

func TestKillBuild(t *testing.T) {
	tests := []struct {
		name           string
		jobName        string
		buildNumber    int
		crumbAvailable bool
		statusCode     int
		wantErr        bool
	}{
		{
			name:           "success with crumb",
			jobName:        "my-job",
			buildNumber:    42,
			crumbAvailable: true,
			statusCode:     http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "success without crumb",
			jobName:        "my-job",
			buildNumber:    42,
			crumbAvailable: false,
			statusCode:     http.StatusOK,
			wantErr:        false,
		},
		{
			name:           "http error",
			jobName:        "my-job",
			buildNumber:    42,
			crumbAvailable: true,
			statusCode:     http.StatusNotFound,
			wantErr:        true,
		},
		{
			name:           "job name with spaces",
			jobName:        "my job",
			buildNumber:    10,
			crumbAvailable: true,
			statusCode:     http.StatusOK,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var killPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Handle crumb request
				if r.URL.Path == "/crumbIssuer/api/json" {
					if tt.crumbAvailable {
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(map[string]string{
							"crumb":             "test-crumb",
							"crumbRequestField": "Jenkins-Crumb",
						})
					} else {
						w.WriteHeader(http.StatusNotFound)
					}
					return
				}

				// Handle kill request
				killPath = r.URL.Path
				if r.Method != "POST" {
					t.Errorf("expected POST method, got %s", r.Method)
				}

				// Verify crumb header when available
				if tt.crumbAvailable {
					if got := r.Header.Get("Jenkins-Crumb"); got != "test-crumb" {
						t.Errorf("expected Jenkins-Crumb header 'test-crumb', got %q", got)
					}
				}

				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 0)
			err := c.KillBuild(context.Background(), tt.jobName, tt.buildNumber)

			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Verify the kill endpoint was called
			// Note: HTTP server receives URLs decoded, so spaces won't be %20
			if killPath == "" {
				t.Errorf("kill endpoint was not called")
			}
			if !strings.Contains(killPath, "/kill") {
				t.Errorf("expected path to contain '/kill', got %s", killPath)
			}
			if !strings.Contains(killPath, fmt.Sprintf("/%d/", tt.buildNumber)) {
				t.Errorf("expected path to contain build number %d, got %s", tt.buildNumber, killPath)
			}
		})
	}
}

func TestReplayBuild(t *testing.T) {
	replayCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(crumbResponse{
				Crumb:             "test-crumb",
				CrumbRequestField: "Jenkins-Crumb",
			})
		case "/job/my-job/api/json":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(JobDetail{
				Job: Job{
					Name:     "my-job",
					FullName: "my-job",
				},
				NextBuildNumber: 10,
			})
		case "/job/my-job/5/replay/rebuild":
			replayCalled = true
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			crumb := r.Header.Get("Jenkins-Crumb")
			if crumb != "test-crumb" {
				t.Errorf("expected crumb header, got %q", crumb)
			}
			contentType := r.Header.Get("Content-Type")
			if contentType != "application/x-www-form-urlencoded" {
				t.Errorf("expected form content type, got %q", contentType)
			}
			body, _ := io.ReadAll(r.Body)
			bodyStr := string(body)
			if !strings.Contains(bodyStr, "mainScript=") {
				t.Error("expected mainScript in form data")
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 0)
	newBuildNumber, err := c.ReplayBuild(context.Background(), "my-job", 5, "println 'test'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newBuildNumber != 10 {
		t.Errorf("expected build number 10, got %d", newBuildNumber)
	}
	if !replayCalled {
		t.Error("expected POST to /job/my-job/5/replay/rebuild")
	}
}

func TestReplayBuildWithoutScript(t *testing.T) {
	replayCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/job/my-job/api/json":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(JobDetail{
				Job: Job{
					Name:     "my-job",
					FullName: "my-job",
				},
				NextBuildNumber: 15,
			})
		case "/job/my-job/10/replay/rebuild":
			replayCalled = true
			body, _ := io.ReadAll(r.Body)
			bodyStr := string(body)
			// Empty script should result in empty form data
			if bodyStr != "" {
				t.Errorf("expected empty form data, got %q", bodyStr)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 0)
	newBuildNumber, err := c.ReplayBuild(context.Background(), "my-job", 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newBuildNumber != 15 {
		t.Errorf("expected build number 15, got %d", newBuildNumber)
	}
	if !replayCalled {
		t.Error("expected POST to /job/my-job/10/replay/rebuild")
	}
}

func TestReplayBuildInFolder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/job/my-folder/job/my-job/api/json":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(JobDetail{
				Job: Job{
					Name:     "my-job",
					FullName: "my-folder/my-job",
				},
				NextBuildNumber: 20,
			})
		case "/job/my-folder/job/my-job/7/replay/rebuild":
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 0)
	newBuildNumber, err := c.ReplayBuild(context.Background(), "my-folder/my-job", 7, "println 'hello'")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newBuildNumber != 20 {
		t.Errorf("expected build number 20, got %d", newBuildNumber)
	}
}

func TestReplayBuildGetJobError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/job/my-job/api/json":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("job not found"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 0)
	_, err := c.ReplayBuild(context.Background(), "my-job", 5, "println 'test'")
	if err == nil {
		t.Fatal("expected error when GetJob fails")
	}
	if !strings.Contains(err.Error(), "getting job info") {
		t.Errorf("expected 'getting job info' in error, got %v", err)
	}
}

func TestReplayBuildPostError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/job/my-job/api/json":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(JobDetail{
				Job: Job{
					Name:     "my-job",
					FullName: "my-job",
				},
				NextBuildNumber: 10,
			})
		case "/job/my-job/5/replay/rebuild":
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("permission denied"))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 0)
	_, err := c.ReplayBuild(context.Background(), "my-job", 5, "println 'test'")
	if err == nil {
		t.Fatal("expected error when POST fails")
	}
}

func TestStopBuild(t *testing.T) {
	stopCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/job/my-job/5/stop":
			stopCalled = true
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 0)
	err := c.StopBuild(context.Background(), "my-job", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !stopCalled {
		t.Error("expected POST to /job/my-job/5/stop")
	}
}

func TestDeleteBuild(t *testing.T) {
	deleteCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/crumbIssuer/api/json":
			w.WriteHeader(http.StatusNotFound)
		case "/job/my-job/3/doDelete":
			deleteCalled = true
			if r.Method != "POST" {
				t.Errorf("expected POST, got %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 0)
	err := c.DeleteBuild(context.Background(), "my-job", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !deleteCalled {
		t.Error("expected POST to /job/my-job/3/doDelete")
	}
}

func TestGetBuildLog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/job/my-job/1/consoleText"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
		}
		w.Write([]byte("Build started\nBuild successful\n"))
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 0)
	reader, err := c.GetBuildLog(context.Background(), "my-job", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer reader.Close()

	buf := make([]byte, 1024)
	n, _ := reader.Read(buf)
	output := string(buf[:n])
	if !strings.Contains(output, "Build started") {
		t.Errorf("expected 'Build started' in output, got %s", output)
	}
}

func TestGetLastBuild(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/job/my-job/lastBuild/api/json"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildDetail{
			Build: Build{
				Number: 100,
				Result: "SUCCESS",
			},
			DisplayName: "#100",
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 0)
	build, err := c.GetLastBuild(context.Background(), "my-job")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if build.Number != 100 {
		t.Errorf("expected build 100, got %d", build.Number)
	}
}

func TestBuildURL(t *testing.T) {
	url := BuildURL("http://jenkins", "my-job", 42)
	expected := "http://jenkins/job/my-job/42"
	if url != expected {
		t.Errorf("BuildURL() = %q, want %q", url, expected)
	}
}

func TestGetBuildArtifacts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/job/my-job/5/api/json" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(buildArtifactsResponse{
			Artifacts: []Artifact{
				{FileName: "app.jar", RelativePath: "app.jar", Size: 1024},
				{FileName: "test.war", RelativePath: "target/test.war", Size: 2048},
			},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 0)
	artifacts, err := c.GetBuildArtifacts(context.Background(), "my-job", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(artifacts))
	}
	if artifacts[0].FileName != "app.jar" {
		t.Errorf("expected app.jar, got %s", artifacts[0].FileName)
	}
	if artifacts[0].Size != 1024 {
		t.Errorf("expected size 1024, got %d", artifacts[0].Size)
	}
}

func TestGetBuildArtifactsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(buildArtifactsResponse{
			Artifacts: []Artifact{},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 0)
	artifacts, err := c.GetBuildArtifacts(context.Background(), "my-job", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(artifacts) != 0 {
		t.Errorf("expected 0 artifacts, got %d", len(artifacts))
	}
}

func TestGetLastSuccessfulBuild(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/job/my-job/lastSuccessfulBuild/api/json"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(BuildDetail{
			Build: Build{Number: 42, Result: "SUCCESS"},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 0)
	build, err := c.GetLastSuccessfulBuild(context.Background(), "my-job")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if build.Number != 42 {
		t.Errorf("expected build 42, got %d", build.Number)
	}
	if build.Result != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %s", build.Result)
	}
}

func TestDownloadArtifact(t *testing.T) {
	testContent := []byte("test artifact content")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/job/my-job/5/artifact/app.jar"
		if r.URL.Path != expectedPath {
			t.Errorf("unexpected path: got %s, want %s", r.URL.Path, expectedPath)
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testContent)))
		w.Write(testContent)
	}))
	defer server.Close()

	c := NewClient(server.URL, "admin", "token", false, 5*time.Second, 0)

	var buf bytes.Buffer
	var progressCalled bool
	callback := func(downloaded, total int64) {
		progressCalled = true
		if total != int64(len(testContent)) {
			t.Errorf("expected total %d, got %d", len(testContent), total)
		}
	}

	err := c.DownloadArtifact(context.Background(), "my-job", 5, "app.jar", &buf, callback)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), testContent) {
		t.Errorf("content mismatch: got %q, want %q", buf.String(), string(testContent))
	}
	if !progressCalled {
		t.Error("progress callback was not called")
	}
}
