package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/PhilipKram/jenkins-cli/internal/errors"
)

type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
)

func PrintTable(w io.Writer, headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	fmt.Fprintln(tw, strings.Repeat("-\t", len(headers)))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	tw.Flush()
}

func PrintJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func StatusColor(status string) string {
	switch strings.ToLower(status) {
	case "success", "stable", "blue":
		return "\033[32m" + status + "\033[0m" // green
	case "failure", "failed", "red":
		return "\033[31m" + status + "\033[0m" // red
	case "unstable", "yellow", "in_progress":
		return "\033[33m" + status + "\033[0m" // yellow
	case "aborted", "grey", "disabled", "notbuilt", "not_built", "not_executed":
		return "\033[90m" + status + "\033[0m" // grey
	case "running", "building":
		return "\033[36m" + status + "\033[0m" // cyan
	default:
		return status
	}
}

// ErrorOutput represents a structured error for JSON output
type ErrorOutput struct {
	ErrorCode   string            `json:"error_code"`
	Message     string            `json:"message"`
	Details     map[string]string `json:"details,omitempty"`
	Suggestions []string          `json:"suggestions,omitempty"`
}

// PrintError outputs an error in the specified format
func PrintError(w io.Writer, err error, format Format) error {
	if err == nil {
		return nil
	}

	if format == FormatJSON {
		return printErrorJSON(w, err)
	}
	return printErrorTable(w, err)
}

func printErrorJSON(w io.Writer, err error) error {
	output := ErrorOutput{
		ErrorCode: "error",
		Message:   err.Error(),
		Details:   make(map[string]string),
	}

	// Check for structured error types
	if connErr, ok := errors.AsConnectionError(err); ok {
		output.ErrorCode = "connection_error"
		output.Message = fmt.Sprintf("Failed to connect to Jenkins at %s", connErr.URL)
		output.Details["url"] = connErr.URL
		if connErr.Err != nil {
			output.Details["cause"] = connErr.Err.Error()
		}
		output.Suggestions = connErr.Suggestions
	} else if authErr, ok := errors.AsAuthenticationError(err); ok {
		output.ErrorCode = "authentication_error"
		output.Message = fmt.Sprintf("Authentication failed using %s method", authErr.AuthMethod)
		output.Details["auth_method"] = authErr.AuthMethod
		if authErr.URL != "" {
			output.Details["url"] = authErr.URL
		}
		if authErr.StatusCode > 0 {
			output.Details["status_code"] = fmt.Sprintf("%d", authErr.StatusCode)
		}
		if authErr.Err != nil {
			output.Details["cause"] = authErr.Err.Error()
		}
		output.Suggestions = authErr.Suggestions
	} else if permErr, ok := errors.AsPermissionError(err); ok {
		output.ErrorCode = "permission_error"
		if permErr.User != "" {
			output.Message = fmt.Sprintf("User '%s' is missing %s permission", permErr.User, permErr.Permission)
			output.Details["user"] = permErr.User
		} else {
			output.Message = fmt.Sprintf("Missing %s permission", permErr.Permission)
		}
		if permErr.Permission != "" {
			output.Details["permission"] = permErr.Permission
		}
		if permErr.URL != "" {
			output.Details["url"] = permErr.URL
		}
		if permErr.AuthMethod != "" {
			output.Details["auth_method"] = permErr.AuthMethod
		}
		if permErr.Err != nil {
			output.Details["cause"] = permErr.Err.Error()
		}
		output.Suggestions = permErr.Suggestions
	} else if notFoundErr, ok := errors.AsNotFoundError(err); ok {
		output.ErrorCode = "not_found"
		if notFoundErr.ResourceType != "" && notFoundErr.ResourceName != "" {
			output.Message = fmt.Sprintf("%s '%s' not found", notFoundErr.ResourceType, notFoundErr.ResourceName)
			output.Details["resource_type"] = notFoundErr.ResourceType
			output.Details["resource_name"] = notFoundErr.ResourceName
		} else if notFoundErr.ResourceName != "" {
			output.Message = fmt.Sprintf("'%s' not found", notFoundErr.ResourceName)
			output.Details["resource_name"] = notFoundErr.ResourceName
		}
		if notFoundErr.URL != "" {
			output.Details["url"] = notFoundErr.URL
		}
		if notFoundErr.Err != nil {
			output.Details["cause"] = notFoundErr.Err.Error()
		}
		output.Suggestions = notFoundErr.Suggestions
	}

	return PrintJSON(w, output)
}

func printErrorTable(w io.Writer, err error) error {
	// Use red color for error messages
	errorPrefix := "\033[31mError:\033[0m "

	// For structured errors, format with suggestions
	if connErr, ok := errors.AsConnectionError(err); ok {
		fmt.Fprintf(w, "%s%s\n", errorPrefix, connErr.Error())
		return nil
	}

	if authErr, ok := errors.AsAuthenticationError(err); ok {
		fmt.Fprintf(w, "%s%s\n", errorPrefix, authErr.Error())
		return nil
	}

	if permErr, ok := errors.AsPermissionError(err); ok {
		fmt.Fprintf(w, "%s%s\n", errorPrefix, permErr.Error())
		return nil
	}

	if notFoundErr, ok := errors.AsNotFoundError(err); ok {
		fmt.Fprintf(w, "%s%s\n", errorPrefix, notFoundErr.Error())
		return nil
	}

	// For generic errors, just print with error prefix
	fmt.Fprintf(w, "%s%s\n", errorPrefix, err.Error())
	return nil
}
