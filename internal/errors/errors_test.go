package errors

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"testing"
)

func TestConnectionError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ConnectionError
		contains []string
	}{
		{
			name: "basic error",
			err: &ConnectionError{
				URL: "http://localhost:8080",
				Err: errors.New("connection refused"),
			},
			contains: []string{"Failed to connect to Jenkins at http://localhost:8080", "connection refused"},
		},
		{
			name: "with suggestions",
			err: &ConnectionError{
				URL: "http://localhost:8080",
				Err: errors.New("timeout"),
				Suggestions: []string{
					"Check if Jenkins is running",
					"Verify the URL is correct",
				},
			},
			contains: []string{"Failed to connect", "timeout", "Suggestions:", "Check if Jenkins is running", "Verify the URL is correct"},
		},
		{
			name: "without underlying error",
			err: &ConnectionError{
				URL:         "http://localhost:8080",
				Suggestions: []string{"Check network connectivity"},
			},
			contains: []string{"Failed to connect to Jenkins at http://localhost:8080", "Check network connectivity"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("expected error to contain %q, got %q", substr, result)
				}
			}
		})
	}
}

func TestConnectionError_Unwrap(t *testing.T) {
	innerErr := errors.New("connection refused")
	err := &ConnectionError{
		URL: "http://localhost:8080",
		Err: innerErr,
	}

	if err.Unwrap() != innerErr {
		t.Error("expected Unwrap to return inner error")
	}
}

func TestAuthenticationError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *AuthenticationError
		contains []string
	}{
		{
			name: "basic error",
			err: &AuthenticationError{
				URL:        "http://localhost:8080",
				AuthMethod: "basic",
				StatusCode: 401,
			},
			contains: []string{"Authentication failed using basic method", "HTTP 401", "http://localhost:8080"},
		},
		{
			name: "with suggestions",
			err: &AuthenticationError{
				URL:         "http://localhost:8080",
				AuthMethod:  "bearer",
				StatusCode:  403,
				Suggestions: []string{"Check your token", "Verify permissions"},
			},
			contains: []string{"Authentication failed using bearer method", "HTTP 403", "Suggestions:", "Check your token"},
		},
		{
			name: "without status code",
			err: &AuthenticationError{
				AuthMethod: "basic",
				Err:        errors.New("invalid credentials"),
			},
			contains: []string{"Authentication failed using basic method", "invalid credentials"},
		},
		{
			name: "without URL",
			err: &AuthenticationError{
				AuthMethod: "basic",
				StatusCode: 401,
			},
			contains: []string{"Authentication failed using basic method", "HTTP 401"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("expected error to contain %q, got %q", substr, result)
				}
			}
		})
	}
}

func TestAuthenticationError_Unwrap(t *testing.T) {
	innerErr := errors.New("invalid token")
	err := &AuthenticationError{
		AuthMethod: "bearer",
		Err:        innerErr,
	}

	if err.Unwrap() != innerErr {
		t.Error("expected Unwrap to return inner error")
	}
}

func TestPermissionError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *PermissionError
		contains []string
	}{
		{
			name: "with user and permission",
			err: &PermissionError{
				URL:        "http://localhost:8080/job/test",
				Permission: "Job/Build",
				User:       "admin",
			},
			contains: []string{"User 'admin' is missing", "Job/Build permission", "http://localhost:8080/job/test"},
		},
		{
			name: "without user",
			err: &PermissionError{
				URL:        "http://localhost:8080",
				Permission: "Overall/Read",
			},
			contains: []string{"Missing", "Overall/Read permission"},
		},
		{
			name: "without permission",
			err: &PermissionError{
				User: "john",
			},
			contains: []string{"User 'john' is missing", "required permission"},
		},
		{
			name: "with suggestions",
			err: &PermissionError{
				User:        "bob",
				Permission:  "Job/Build",
				Suggestions: []string{"Contact your Jenkins administrator", "Check permission matrix"},
			},
			contains: []string{"User 'bob'", "Job/Build permission", "Suggestions:", "Contact your Jenkins administrator"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("expected error to contain %q, got %q", substr, result)
				}
			}
		})
	}
}

func TestPermissionError_Unwrap(t *testing.T) {
	innerErr := errors.New("access denied")
	err := &PermissionError{
		Permission: "Job/Build",
		Err:        innerErr,
	}

	if err.Unwrap() != innerErr {
		t.Error("expected Unwrap to return inner error")
	}
}

func TestNotFoundError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *NotFoundError
		contains []string
	}{
		{
			name: "with resource type and name",
			err: &NotFoundError{
				ResourceType: "Job",
				ResourceName: "my-project",
				URL:          "http://localhost:8080/job/my-project",
			},
			contains: []string{"Job 'my-project' not found", "http://localhost:8080/job/my-project"},
		},
		{
			name: "with only resource name",
			err: &NotFoundError{
				ResourceName: "my-project",
			},
			contains: []string{"'my-project' not found"},
		},
		{
			name: "without resource info",
			err: &NotFoundError{
				URL: "http://localhost:8080",
			},
			contains: []string{"Resource not found", "http://localhost:8080"},
		},
		{
			name: "with suggestions",
			err: &NotFoundError{
				ResourceType: "Job",
				ResourceName: "my-projct",
				Suggestions:  []string{"my-project", "my-proj"},
			},
			contains: []string{"Job 'my-projct' not found", "Did you mean:", "my-project", "my-proj"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			for _, substr := range tt.contains {
				if !strings.Contains(result, substr) {
					t.Errorf("expected error to contain %q, got %q", substr, result)
				}
			}
		})
	}
}

func TestNotFoundError_Unwrap(t *testing.T) {
	innerErr := errors.New("HTTP 404")
	err := &NotFoundError{
		ResourceType: "Job",
		ResourceName: "test",
		Err:          innerErr,
	}

	if err.Unwrap() != innerErr {
		t.Error("expected Unwrap to return inner error")
	}
}

func TestAsConnectionError(t *testing.T) {
	connErr := &ConnectionError{URL: "http://localhost:8080"}
	wrappedErr := fmt.Errorf("wrapped: %w", connErr)

	tests := []struct {
		name      string
		err       error
		wantFound bool
		wantURL   string
	}{
		{
			name:      "direct connection error",
			err:       connErr,
			wantFound: true,
			wantURL:   "http://localhost:8080",
		},
		{
			name:      "wrapped connection error",
			err:       wrappedErr,
			wantFound: true,
			wantURL:   "http://localhost:8080",
		},
		{
			name:      "other error",
			err:       errors.New("not a connection error"),
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := AsConnectionError(tt.err)
			if found != tt.wantFound {
				t.Errorf("expected found=%v, got %v", tt.wantFound, found)
			}
			if found && result.URL != tt.wantURL {
				t.Errorf("expected URL=%s, got %s", tt.wantURL, result.URL)
			}
		})
	}
}

func TestAsAuthenticationError(t *testing.T) {
	authErr := &AuthenticationError{AuthMethod: "basic"}
	wrappedErr := fmt.Errorf("wrapped: %w", authErr)

	tests := []struct {
		name           string
		err            error
		wantFound      bool
		wantAuthMethod string
	}{
		{
			name:           "direct auth error",
			err:            authErr,
			wantFound:      true,
			wantAuthMethod: "basic",
		},
		{
			name:           "wrapped auth error",
			err:            wrappedErr,
			wantFound:      true,
			wantAuthMethod: "basic",
		},
		{
			name:      "other error",
			err:       errors.New("not an auth error"),
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := AsAuthenticationError(tt.err)
			if found != tt.wantFound {
				t.Errorf("expected found=%v, got %v", tt.wantFound, found)
			}
			if found && result.AuthMethod != tt.wantAuthMethod {
				t.Errorf("expected AuthMethod=%s, got %s", tt.wantAuthMethod, result.AuthMethod)
			}
		})
	}
}

func TestAsPermissionError(t *testing.T) {
	permErr := &PermissionError{Permission: "Job/Build"}
	wrappedErr := fmt.Errorf("wrapped: %w", permErr)

	tests := []struct {
		name           string
		err            error
		wantFound      bool
		wantPermission string
	}{
		{
			name:           "direct permission error",
			err:            permErr,
			wantFound:      true,
			wantPermission: "Job/Build",
		},
		{
			name:           "wrapped permission error",
			err:            wrappedErr,
			wantFound:      true,
			wantPermission: "Job/Build",
		},
		{
			name:      "other error",
			err:       errors.New("not a permission error"),
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := AsPermissionError(tt.err)
			if found != tt.wantFound {
				t.Errorf("expected found=%v, got %v", tt.wantFound, found)
			}
			if found && result.Permission != tt.wantPermission {
				t.Errorf("expected Permission=%s, got %s", tt.wantPermission, result.Permission)
			}
		})
	}
}

func TestAsNotFoundError(t *testing.T) {
	notFoundErr := &NotFoundError{ResourceName: "test"}
	wrappedErr := fmt.Errorf("wrapped: %w", notFoundErr)

	tests := []struct {
		name             string
		err              error
		wantFound        bool
		wantResourceName string
	}{
		{
			name:             "direct not found error",
			err:              notFoundErr,
			wantFound:        true,
			wantResourceName: "test",
		},
		{
			name:             "wrapped not found error",
			err:              wrappedErr,
			wantFound:        true,
			wantResourceName: "test",
		},
		{
			name:      "other error",
			err:       errors.New("not a not found error"),
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := AsNotFoundError(tt.err)
			if found != tt.wantFound {
				t.Errorf("expected found=%v, got %v", tt.wantFound, found)
			}
			if found && result.ResourceName != tt.wantResourceName {
				t.Errorf("expected ResourceName=%s, got %s", tt.wantResourceName, result.ResourceName)
			}
		})
	}
}

func TestIsAnonymousPermissionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "anonymous permission error",
			err:  errors.New("Anonymous is missing the Overall/Read permission"),
			want: true,
		},
		{
			name: "anonymous permission error mixed case",
			err:  errors.New("ANONYMOUS is missing the OVERALL/READ PERMISSION"),
			want: true,
		},
		{
			name: "regular permission error",
			err:  errors.New("User is missing the Job/Build permission"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "other error",
			err:  errors.New("something else"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAnonymousPermissionError(tt.err); got != tt.want {
				t.Errorf("IsAnonymousPermissionError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsConnectionRefused(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "syscall ECONNREFUSED",
			err:  syscall.ECONNREFUSED,
			want: true,
		},
		{
			name: "net.OpError with ECONNREFUSED",
			err:  &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED},
			want: true,
		},
		{
			name: "error message contains connection refused",
			err:  errors.New("dial tcp: connection refused"),
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "other error",
			err:  errors.New("something else"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsConnectionRefused(tt.err); got != tt.want {
				t.Errorf("IsConnectionRefused() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAuthenticationFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "AuthenticationError",
			err:  &AuthenticationError{AuthMethod: "basic"},
			want: true,
		},
		{
			name: "HTTP 401",
			err:  errors.New("HTTP 401 Unauthorized"),
			want: true,
		},
		{
			name: "HTTP 403",
			err:  errors.New("HTTP 403 Forbidden"),
			want: true,
		},
		{
			name: "401 unauthorized",
			err:  errors.New("401 unauthorized"),
			want: true,
		},
		{
			name: "403 forbidden",
			err:  errors.New("403 forbidden"),
			want: true,
		},
		{
			name: "authentication failed",
			err:  errors.New("authentication failed"),
			want: true,
		},
		{
			name: "invalid credentials",
			err:  errors.New("invalid credentials"),
			want: true,
		},
		{
			name: "access denied",
			err:  errors.New("access denied"),
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "other error",
			err:  errors.New("something else"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAuthenticationFailure(tt.err); got != tt.want {
				t.Errorf("IsAuthenticationFailure() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "NotFoundError",
			err:  &NotFoundError{ResourceName: "test"},
			want: true,
		},
		{
			name: "HTTP 404",
			err:  errors.New("HTTP 404 Not Found"),
			want: true,
		},
		{
			name: "404 in message",
			err:  errors.New("received 404"),
			want: true,
		},
		{
			name: "not found message",
			err:  errors.New("resource not found"),
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "other error",
			err:  errors.New("something else"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

type timeoutError struct{}

func (e timeoutError) Error() string   { return "timeout" }
func (e timeoutError) Timeout() bool   { return true }
func (e timeoutError) Temporary() bool { return true }

func TestIsTimeoutError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "net.Error with Timeout",
			err:  timeoutError{},
			want: true,
		},
		{
			name: "timeout in message",
			err:  errors.New("connection timeout"),
			want: true,
		},
		{
			name: "timed out in message",
			err:  errors.New("request timed out"),
			want: true,
		},
		{
			name: "deadline exceeded",
			err:  errors.New("context deadline exceeded"),
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "other error",
			err:  errors.New("something else"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTimeoutError(tt.err); got != tt.want {
				t.Errorf("IsTimeoutError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDNSError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "net.DNSError",
			err:  &net.DNSError{Name: "example.com"},
			want: true,
		},
		{
			name: "net.OpError with dial operation",
			err:  &net.OpError{Op: "dial", Err: &net.DNSError{Name: "example.com"}},
			want: true,
		},
		{
			name: "net.OpError with lookup operation",
			err:  &net.OpError{Op: "lookup", Err: &net.DNSError{Name: "example.com"}},
			want: true,
		},
		{
			name: "no such host",
			err:  errors.New("no such host"),
			want: true,
		},
		{
			name: "dns error in message",
			err:  errors.New("dns lookup failed"),
			want: true,
		},
		{
			name: "name resolution in message",
			err:  errors.New("name resolution failed"),
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "other error",
			err:  errors.New("something else"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDNSError(tt.err); got != tt.want {
				t.Errorf("IsDNSError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTLSError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "tls in message",
			err:  errors.New("tls: bad certificate"),
			want: true,
		},
		{
			name: "ssl in message",
			err:  errors.New("ssl handshake failed"),
			want: true,
		},
		{
			name: "certificate in message",
			err:  errors.New("certificate verify failed"),
			want: true,
		},
		{
			name: "x509 in message",
			err:  errors.New("x509: certificate has expired"),
			want: true,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "other error",
			err:  errors.New("something else"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTLSError(tt.err); got != tt.want {
				t.Errorf("IsTLSError() = %v, want %v", got, tt.want)
			}
		})
	}
}
