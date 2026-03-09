package mcp

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
)

// RegisterDefaultResources registers all Jenkins resource templates with the server.
func RegisterDefaultResources(s *Server) {
	s.AddResourceTemplate(
		ResourceTemplate{
			URITemplate: "jenkins:///{job}/{number}/log",
			Name:        "build_log",
			Description: "Console output for a Jenkins build (text/plain, capped at 1 MiB)",
			MimeType:    "text/plain",
		},
		handleBuildLogResource,
	)

	s.AddResourceTemplate(
		ResourceTemplate{
			URITemplate: "jenkins:///{job}/config.xml",
			Name:        "job_config",
			Description: "Job XML configuration",
			MimeType:    "application/xml",
		},
		handleJobConfigResource,
	)

	s.AddResourceTemplate(
		ResourceTemplate{
			URITemplate: "jenkins:///system/log",
			Name:        "system_log",
			Description: "Jenkins system log",
			MimeType:    "text/plain",
		},
		handleSystemLogResource,
	)
}

func handleBuildLogResource(uri string) (*ResourceReadResult, error) {
	// Parse URI: jenkins:///{job}/{number}/log
	trimmed := strings.TrimPrefix(uri, "jenkins:///")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 || parts[len(parts)-1] != "log" {
		return nil, fmt.Errorf("invalid build log URI: %s", uri)
	}

	// Everything except the last two parts is the job path
	job := strings.Join(parts[:len(parts)-2], "/")
	numberStr := parts[len(parts)-2]

	var number int
	if _, err := fmt.Sscanf(numberStr, "%d", &number); err != nil {
		return nil, fmt.Errorf("invalid build number in URI: %s", numberStr)
	}

	client, err := clientutil.NewClient(60*time.Second, 3)
	if err != nil {
		return nil, err
	}

	rc, err := client.GetBuildLog(nil, job, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get build log: %w", err)
	}
	defer rc.Close()

	data, err := io.ReadAll(io.LimitReader(rc, maxLogSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read build log: %w", err)
	}

	text := string(data)
	if len(data) == maxLogSize {
		text += "\n\n[Output truncated at 1 MiB]"
	}

	return &ResourceReadResult{
		Contents: []ResourceContents{
			{
				URI:      uri,
				MimeType: "text/plain",
				Text:     text,
			},
		},
	}, nil
}

func handleJobConfigResource(uri string) (*ResourceReadResult, error) {
	// Parse URI: jenkins:///{job}/config.xml
	trimmed := strings.TrimPrefix(uri, "jenkins:///")
	if !strings.HasSuffix(trimmed, "/config.xml") {
		return nil, fmt.Errorf("invalid job config URI: %s", uri)
	}
	job := strings.TrimSuffix(trimmed, "/config.xml")

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	configXML, err := client.GetJobConfig(nil, job)
	if err != nil {
		return nil, fmt.Errorf("failed to get job config: %w", err)
	}

	return &ResourceReadResult{
		Contents: []ResourceContents{
			{
				URI:      uri,
				MimeType: "application/xml",
				Text:     configXML,
			},
		},
	}, nil
}

func handleSystemLogResource(uri string) (*ResourceReadResult, error) {
	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return nil, err
	}

	log, err := client.GetSystemLog(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get system log: %w", err)
	}

	return &ResourceReadResult{
		Contents: []ResourceContents{
			{
				URI:      uri,
				MimeType: "text/plain",
				Text:     log,
			},
		},
	}, nil
}
