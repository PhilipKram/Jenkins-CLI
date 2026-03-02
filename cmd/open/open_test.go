package open

import (
	"strings"
	"testing"
)

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr string // empty means no error expected
	}{
		{"valid http", "http://jenkins.example.com:8080", ""},
		{"valid https", "https://jenkins.example.com:8080", ""},
		{"valid with path", "https://jenkins.example.com/job/test-job", ""},
		{"valid with query", "https://jenkins.example.com/job/test?param=value", ""},
		{"valid with port", "https://jenkins.example.com:9090/", ""},
		{"valid with encoded chars", "https://jenkins.example.com/job/test%20job", ""},
		{"valid with fragment", "https://jenkins.example.com/job/test#section", ""},
		{"valid uppercase scheme", "HTTP://jenkins.example.com", ""},

		// Rejected schemes
		{"file scheme", "file:///etc/passwd", "unsupported URL scheme"},
		{"javascript scheme", "javascript:alert('xss')", "unsupported URL scheme"},
		{"ftp scheme", "ftp://example.com", "unsupported URL scheme"},
		{"data scheme", "data:text/html,<script>alert('xss')</script>", "unsupported URL scheme"},

		// Missing/invalid
		{"empty URL", "", "unsupported URL scheme"},
		{"no scheme", "jenkins.example.com", "unsupported URL scheme"},
		{"relative URL", "/job/test-job", "unsupported URL scheme"},
		{"malformed URL", "ht!tp://invalid", "invalid URL"},

		// Missing host
		{"http no host", "http://", "missing host"},
		{"https no host", "https://", "missing host"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateURL(tc.url)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tc.wantErr)
				} else if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("expected error containing %q, got: %v", tc.wantErr, err)
				}
			}
		})
	}
}

func TestCmdStructure(t *testing.T) {
	if Cmd == nil {
		t.Fatal("Cmd should not be nil")
	}
	if Cmd.Use == "" {
		t.Error("Cmd.Use should not be empty")
	}
	if Cmd.Short == "" {
		t.Error("Cmd.Short should not be empty")
	}
	if Cmd.RunE == nil {
		t.Error("Cmd.RunE should not be nil")
	}
}
