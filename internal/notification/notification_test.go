package notification

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// mockNotifier is a test notifier that records calls instead of executing them.
type mockNotifier struct {
	title   string
	message string
	sendErr error
}

func (m *mockNotifier) Send(title, message string) error {
	m.title = title
	m.message = message
	return m.sendErr
}

func TestNewNotifier(t *testing.T) {
	n := NewNotifier()
	if n == nil {
		t.Fatal("NewNotifier() returned nil")
	}

	// Verify it returns a notifier instance
	notif, ok := n.(*notifier)
	if !ok {
		t.Fatalf("NewNotifier() returned unexpected type: %T", n)
	}

	// Verify OS type is set
	if notif.osType == "" {
		t.Error("notifier osType is empty")
	}
}

func TestNotifier_Send_Darwin(t *testing.T) {
	// Create notifier with mocked darwin OS
	n := &notifier{osType: "darwin"}

	// Capture stderr output
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Send notification (will fail since osascript may not be available or command may fail)
	// But it should not return an error - just log to stderr
	err := n.Send("Test Title", "Test Message")

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured stderr
	var buf bytes.Buffer
	buf.ReadFrom(r)
	stderr := buf.String()

	// Should not return an error even if command fails
	if err != nil {
		t.Errorf("Send() returned error: %v", err)
	}

	// If osascript is not available or fails, stderr should contain error message
	// If it succeeds, stderr will be empty
	// Both are acceptable outcomes
	t.Logf("stderr output: %s", stderr)
}

func TestNotifier_Send_Linux(t *testing.T) {
	// Create notifier with mocked linux OS
	n := &notifier{osType: "linux"}

	// Capture stderr output
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Send notification (will fail since notify-send may not be available)
	// But it should not return an error - just log to stderr
	err := n.Send("Test Title", "Test Message")

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured stderr
	var buf bytes.Buffer
	buf.ReadFrom(r)
	stderr := buf.String()

	// Should not return an error even if command fails
	if err != nil {
		t.Errorf("Send() returned error: %v", err)
	}

	// If notify-send is not available or fails, stderr should contain error message
	// If it succeeds, stderr will be empty
	// Both are acceptable outcomes
	t.Logf("stderr output: %s", stderr)
}

func TestNotifier_Send_UnsupportedOS(t *testing.T) {
	// Create notifier with unsupported OS
	n := &notifier{osType: "windows"}

	// Capture stderr output
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Send notification
	err := n.Send("Test Title", "Test Message")

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Read captured stderr
	var buf bytes.Buffer
	buf.ReadFrom(r)
	stderr := buf.String()

	// Should not return an error
	if err != nil {
		t.Errorf("Send() returned error: %v", err)
	}

	// Should log unsupported OS message to stderr
	if !strings.Contains(stderr, "Desktop notifications not supported") {
		t.Errorf("Expected unsupported OS message in stderr, got: %s", stderr)
	}
	if !strings.Contains(stderr, "windows") {
		t.Errorf("Expected OS name in stderr message, got: %s", stderr)
	}
}

func TestSendBuildComplete(t *testing.T) {
	// Capture stderr output to suppress any error messages
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Test with different build results
	tests := []struct {
		job         string
		buildNumber int
		result      string
	}{
		{"my-job", 123, "SUCCESS"},
		{"test-build", 456, "FAILURE"},
		{"ci-pipeline", 789, "UNSTABLE"},
		{"deploy-job", 1, "ABORTED"},
	}

	for _, tt := range tests {
		t.Run(tt.result, func(t *testing.T) {
			err := SendBuildComplete(tt.job, tt.buildNumber, tt.result)
			if err != nil {
				t.Errorf("SendBuildComplete() returned error: %v", err)
			}
		})
	}

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Drain pipe
	var buf bytes.Buffer
	buf.ReadFrom(r)
	t.Logf("stderr output: %s", buf.String())
}

func TestSendBuildComplete_MessageFormat(t *testing.T) {
	// We can't easily test the actual notification content without mocking,
	// but we can test that the function doesn't panic or return errors
	tests := []struct {
		name        string
		job         string
		buildNumber int
		result      string
	}{
		{
			name:        "success build",
			job:         "my-job",
			buildNumber: 42,
			result:      "SUCCESS",
		},
		{
			name:        "failed build",
			job:         "test-job",
			buildNumber: 1,
			result:      "FAILURE",
		},
		{
			name:        "job with spaces",
			job:         "my test job",
			buildNumber: 999,
			result:      "UNSTABLE",
		},
		{
			name:        "special characters",
			job:         "job-with-dashes_and_underscores",
			buildNumber: 1234,
			result:      "ABORTED",
		},
	}

	// Capture stderr to suppress error messages during tests
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SendBuildComplete(tt.job, tt.buildNumber, tt.result)
			if err != nil {
				t.Errorf("SendBuildComplete(%q, %d, %q) returned error: %v",
					tt.job, tt.buildNumber, tt.result, err)
			}
		})
	}

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	// Drain pipe
	var buf bytes.Buffer
	buf.ReadFrom(r)
}
