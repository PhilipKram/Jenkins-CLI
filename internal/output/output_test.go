package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/PhilipKram/jenkins-cli/internal/errors"
)

func TestPrintTable(t *testing.T) {
	var buf bytes.Buffer
	headers := []string{"NAME", "STATUS"}
	rows := [][]string{
		{"job-1", "SUCCESS"},
		{"job-2", "FAILURE"},
	}

	PrintTable(&buf, headers, rows)
	out := buf.String()

	if !strings.Contains(out, "NAME") {
		t.Error("expected NAME header in output")
	}
	if !strings.Contains(out, "job-1") {
		t.Error("expected job-1 in output")
	}
	if !strings.Contains(out, "job-2") {
		t.Error("expected job-2 in output")
	}
}

func TestPrintJSON(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"key": "value"}

	if err := PrintJSON(&buf, data); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, `"key": "value"`) {
		t.Errorf("unexpected JSON output: %s", out)
	}
}

func TestStatusColor(t *testing.T) {
	tests := []struct {
		status   string
		contains string
	}{
		{"SUCCESS", "SUCCESS"},
		{"FAILURE", "FAILURE"},
		{"UNSTABLE", "UNSTABLE"},
		{"RUNNING", "RUNNING"},
		{"unknown-status", "unknown-status"},
	}

	for _, tt := range tests {
		got := StatusColor(tt.status)
		if !strings.Contains(got, tt.contains) {
			t.Errorf("StatusColor(%q) = %q, should contain %q", tt.status, got, tt.contains)
		}
	}
}

func TestPrintError_ConnectionError_JSON(t *testing.T) {
	var buf bytes.Buffer
	err := &errors.ConnectionError{
		URL: "http://jenkins.example.com",
		Err: fmt.Errorf("connection refused"),
		Suggestions: []string{
			"Check if Jenkins is running",
			"Verify the URL is correct",
		},
	}

	if err := PrintError(&buf, err, FormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output ErrorOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if output.ErrorCode != "connection_error" {
		t.Errorf("expected error_code 'connection_error', got %q", output.ErrorCode)
	}
	if !strings.Contains(output.Message, "jenkins.example.com") {
		t.Errorf("expected message to contain URL, got %q", output.Message)
	}
	if output.Details["url"] != "http://jenkins.example.com" {
		t.Errorf("expected URL in details, got %q", output.Details["url"])
	}
	if len(output.Suggestions) != 2 {
		t.Errorf("expected 2 suggestions, got %d", len(output.Suggestions))
	}
}

func TestPrintError_ConnectionError_Table(t *testing.T) {
	var buf bytes.Buffer
	err := &errors.ConnectionError{
		URL: "http://jenkins.example.com",
		Err: fmt.Errorf("connection refused"),
		Suggestions: []string{
			"Check if Jenkins is running",
		},
	}

	if err := PrintError(&buf, err, FormatTable); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Error:") {
		t.Error("expected 'Error:' prefix in output")
	}
	if !strings.Contains(output, "jenkins.example.com") {
		t.Error("expected URL in output")
	}
	if !strings.Contains(output, "Suggestions:") {
		t.Error("expected 'Suggestions:' in output")
	}
}

func TestPrintError_AuthenticationError_JSON(t *testing.T) {
	var buf bytes.Buffer
	err := &errors.AuthenticationError{
		URL:        "http://jenkins.example.com",
		AuthMethod: "basic",
		StatusCode: 401,
		Suggestions: []string{
			"Verify your username and password",
			"Try running 'jenkins config test'",
		},
	}

	if err := PrintError(&buf, err, FormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output ErrorOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if output.ErrorCode != "authentication_error" {
		t.Errorf("expected error_code 'authentication_error', got %q", output.ErrorCode)
	}
	if output.Details["auth_method"] != "basic" {
		t.Errorf("expected auth_method 'basic', got %q", output.Details["auth_method"])
	}
	if output.Details["status_code"] != "401" {
		t.Errorf("expected status_code '401', got %q", output.Details["status_code"])
	}
}

func TestPrintError_PermissionError_JSON(t *testing.T) {
	var buf bytes.Buffer
	err := &errors.PermissionError{
		URL:        "http://jenkins.example.com",
		Permission: "Overall/Read",
		User:       "anonymous",
		AuthMethod: "none",
		Suggestions: []string{
			"Configure authentication in Jenkins",
			"Run 'jenkins-cli configure' to set up credentials",
		},
	}

	if err := PrintError(&buf, err, FormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output ErrorOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if output.ErrorCode != "permission_error" {
		t.Errorf("expected error_code 'permission_error', got %q", output.ErrorCode)
	}
	if output.Details["user"] != "anonymous" {
		t.Errorf("expected user 'anonymous', got %q", output.Details["user"])
	}
	if output.Details["permission"] != "Overall/Read" {
		t.Errorf("expected permission 'Overall/Read', got %q", output.Details["permission"])
	}
}

func TestPrintError_NotFoundError_JSON(t *testing.T) {
	var buf bytes.Buffer
	err := &errors.NotFoundError{
		ResourceType: "Job",
		ResourceName: "my-missing-job",
		URL:          "http://jenkins.example.com/job/my-missing-job",
		Suggestions: []string{
			"my-other-job",
			"my-backup-job",
		},
	}

	if err := PrintError(&buf, err, FormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output ErrorOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if output.ErrorCode != "not_found" {
		t.Errorf("expected error_code 'not_found', got %q", output.ErrorCode)
	}
	if output.Details["resource_type"] != "Job" {
		t.Errorf("expected resource_type 'Job', got %q", output.Details["resource_type"])
	}
	if output.Details["resource_name"] != "my-missing-job" {
		t.Errorf("expected resource_name 'my-missing-job', got %q", output.Details["resource_name"])
	}
	if len(output.Suggestions) != 2 {
		t.Errorf("expected 2 suggestions, got %d", len(output.Suggestions))
	}
}

func TestPrintError_GenericError_JSON(t *testing.T) {
	var buf bytes.Buffer
	err := fmt.Errorf("something went wrong")

	if err := PrintError(&buf, err, FormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output ErrorOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if output.ErrorCode != "error" {
		t.Errorf("expected error_code 'error', got %q", output.ErrorCode)
	}
	if output.Message != "something went wrong" {
		t.Errorf("expected message 'something went wrong', got %q", output.Message)
	}
}

func TestPrintError_GenericError_Table(t *testing.T) {
	var buf bytes.Buffer
	err := fmt.Errorf("something went wrong")

	if err := PrintError(&buf, err, FormatTable); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Error:") {
		t.Error("expected 'Error:' prefix in output")
	}
	if !strings.Contains(output, "something went wrong") {
		t.Error("expected error message in output")
	}
}

func TestPrintError_Nil(t *testing.T) {
	var buf bytes.Buffer

	if err := PrintError(&buf, nil, FormatJSON); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.Len() != 0 {
		t.Error("expected no output for nil error")
	}
}
