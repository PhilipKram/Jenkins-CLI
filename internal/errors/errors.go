package errors

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
)

// ConnectionError represents a failure to connect to Jenkins.
type ConnectionError struct {
	URL         string
	Err         error
	Suggestions []string
}

func (e *ConnectionError) Error() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Failed to connect to Jenkins at %s", e.URL))

	if e.Err != nil {
		b.WriteString(fmt.Sprintf(": %v", e.Err))
	}

	if len(e.Suggestions) > 0 {
		b.WriteString("\n\nSuggestions:")
		for _, s := range e.Suggestions {
			b.WriteString(fmt.Sprintf("\n  - %s", s))
		}
	}

	return b.String()
}

func (e *ConnectionError) Unwrap() error {
	return e.Err
}

// AuthenticationError represents an authentication failure.
type AuthenticationError struct {
	URL         string
	AuthMethod  string
	StatusCode  int
	Err         error
	Suggestions []string
}

func (e *AuthenticationError) Error() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Authentication failed using %s method", e.AuthMethod))

	if e.StatusCode > 0 {
		b.WriteString(fmt.Sprintf(" (HTTP %d)", e.StatusCode))
	}

	if e.URL != "" {
		b.WriteString(fmt.Sprintf(" at %s", e.URL))
	}

	if e.Err != nil {
		b.WriteString(fmt.Sprintf(": %v", e.Err))
	}

	if len(e.Suggestions) > 0 {
		b.WriteString("\n\nSuggestions:")
		for _, s := range e.Suggestions {
			b.WriteString(fmt.Sprintf("\n  - %s", s))
		}
	}

	return b.String()
}

func (e *AuthenticationError) Unwrap() error {
	return e.Err
}

// PermissionError represents a permission denied error.
type PermissionError struct {
	URL          string
	Permission   string
	User         string
	AuthMethod   string
	Err          error
	Suggestions  []string
}

func (e *PermissionError) Error() string {
	var b strings.Builder

	if e.User != "" {
		b.WriteString(fmt.Sprintf("User '%s' is missing", e.User))
	} else {
		b.WriteString("Missing")
	}

	if e.Permission != "" {
		b.WriteString(fmt.Sprintf(" %s permission", e.Permission))
	} else {
		b.WriteString(" required permission")
	}

	if e.URL != "" {
		b.WriteString(fmt.Sprintf(" for %s", e.URL))
	}

	if e.Err != nil {
		b.WriteString(fmt.Sprintf(": %v", e.Err))
	}

	if len(e.Suggestions) > 0 {
		b.WriteString("\n\nSuggestions:")
		for _, s := range e.Suggestions {
			b.WriteString(fmt.Sprintf("\n  - %s", s))
		}
	}

	return b.String()
}

func (e *PermissionError) Unwrap() error {
	return e.Err
}

// NotFoundError represents a resource not found error.
type NotFoundError struct {
	ResourceType string
	ResourceName string
	URL          string
	Err          error
	Suggestions  []string
}

func (e *NotFoundError) Error() string {
	var b strings.Builder

	if e.ResourceType != "" && e.ResourceName != "" {
		b.WriteString(fmt.Sprintf("%s '%s' not found", e.ResourceType, e.ResourceName))
	} else if e.ResourceName != "" {
		b.WriteString(fmt.Sprintf("'%s' not found", e.ResourceName))
	} else {
		b.WriteString("Resource not found")
	}

	if e.URL != "" {
		b.WriteString(fmt.Sprintf(" at %s", e.URL))
	}

	if e.Err != nil {
		b.WriteString(fmt.Sprintf(": %v", e.Err))
	}

	if len(e.Suggestions) > 0 {
		b.WriteString("\n\nDid you mean:")
		for _, s := range e.Suggestions {
			b.WriteString(fmt.Sprintf("\n  - %s", s))
		}
	}

	return b.String()
}

func (e *NotFoundError) Unwrap() error {
	return e.Err
}

// Error detection helpers

// AsConnectionError checks if the error is a ConnectionError and returns it.
func AsConnectionError(err error) (*ConnectionError, bool) {
	var connErr *ConnectionError
	if errors.As(err, &connErr) {
		return connErr, true
	}
	return nil, false
}

// AsAuthenticationError checks if the error is an AuthenticationError and returns it.
func AsAuthenticationError(err error) (*AuthenticationError, bool) {
	var authErr *AuthenticationError
	if errors.As(err, &authErr) {
		return authErr, true
	}
	return nil, false
}

// AsPermissionError checks if the error is a PermissionError and returns it.
func AsPermissionError(err error) (*PermissionError, bool) {
	var permErr *PermissionError
	if errors.As(err, &permErr) {
		return permErr, true
	}
	return nil, false
}

// AsNotFoundError checks if the error is a NotFoundError and returns it.
func AsNotFoundError(err error) (*NotFoundError, bool) {
	var notFoundErr *NotFoundError
	if errors.As(err, &notFoundErr) {
		return notFoundErr, true
	}
	return nil, false
}

// IsAnonymousPermissionError checks if the error message indicates anonymous user
// is missing Overall/Read permission.
func IsAnonymousPermissionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "anonymous") &&
		strings.Contains(errStr, "overall/read") &&
		strings.Contains(errStr, "permission")
}

// IsConnectionRefused checks if the error is a connection refused error.
func IsConnectionRefused(err error) bool {
	if err == nil {
		return false
	}

	// Check for syscall.ECONNREFUSED
	var syscallErr syscall.Errno
	if errors.As(err, &syscallErr) && syscallErr == syscall.ECONNREFUSED {
		return true
	}

	// Check for net.OpError with connection refused
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Err != nil {
			return IsConnectionRefused(opErr.Err)
		}
	}

	// Check error message as fallback
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "connection refused")
}

// IsAuthenticationFailure checks if the error indicates an authentication failure.
// This includes 401 Unauthorized and 403 Forbidden responses.
func IsAuthenticationFailure(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's already wrapped as AuthenticationError
	if _, ok := AsAuthenticationError(err); ok {
		return true
	}

	// Check error message for HTTP status codes and auth-related keywords
	errStr := strings.ToLower(err.Error())

	// Check for HTTP 401 or 403
	if strings.Contains(errStr, "http 401") || strings.Contains(errStr, "http 403") {
		return true
	}

	// Check for 401 or 403 status codes
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "403") {
		if strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "forbidden") {
			return true
		}
	}

	// Check for authentication-related keywords
	authKeywords := []string{
		"unauthorized",
		"authentication failed",
		"invalid credentials",
		"access denied",
	}

	for _, keyword := range authKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	return false
}

// IsNotFound checks if the error indicates a resource was not found.
// This typically corresponds to HTTP 404 responses.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's already wrapped as NotFoundError
	if _, ok := AsNotFoundError(err); ok {
		return true
	}

	// Check error message for HTTP 404
	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "http 404") || strings.Contains(errStr, "404") {
		return true
	}

	if strings.Contains(errStr, "not found") {
		return true
	}

	return false
}

// IsTimeoutError checks if the error is a timeout error.
func IsTimeoutError(err error) bool {
	if err == nil {
		return false
	}

	// Check for net.Error with Timeout() method
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Check error message as fallback
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "timed out") ||
		strings.Contains(errStr, "deadline exceeded")
}

// IsDNSError checks if the error is a DNS resolution error.
func IsDNSError(err error) bool {
	if err == nil {
		return false
	}

	// Check for net.DNSError
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	// Check for net.OpError with DNS-related operations
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if opErr.Op == "dial" || opErr.Op == "lookup" {
			if opErr.Err != nil {
				return IsDNSError(opErr.Err)
			}
		}
	}

	// Check error message as fallback
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "no such host") ||
		strings.Contains(errStr, "dns") ||
		strings.Contains(errStr, "name resolution")
}

// IsTLSError checks if the error is related to TLS/SSL.
func IsTLSError(err error) bool {
	if err == nil {
		return false
	}

	// Check error message for TLS-related keywords
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "tls") ||
		strings.Contains(errStr, "ssl") ||
		strings.Contains(errStr, "certificate") ||
		strings.Contains(errStr, "x509")
}
