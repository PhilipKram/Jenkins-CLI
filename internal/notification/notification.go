package notification

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// Notifier sends desktop notifications.
type Notifier interface {
	Send(title, message string) error
}

// notifier is the default notifier implementation.
type notifier struct {
	osType string
}

// NewNotifier creates a new notifier based on the current OS.
func NewNotifier() Notifier {
	return &notifier{
		osType: runtime.GOOS,
	}
}

// Send sends a desktop notification with the given title and message.
// On macOS, it uses osascript with display notification.
// On Linux, it uses notify-send.
// Errors are logged to stderr but do not cause the function to fail,
// allowing the CLI to continue even if notifications are unavailable.
func (n *notifier) Send(title, message string) error {
	var cmd *exec.Cmd

	switch n.osType {
	case "darwin": // macOS
		script := fmt.Sprintf("display notification %q with title %q", message, title)
		cmd = exec.Command("osascript", "-e", script)
	case "linux":
		cmd = exec.Command("notify-send", title, message)
	default:
		// Unsupported OS - log to stderr but don't fail
		fmt.Fprintf(os.Stderr, "Desktop notifications not supported on %s\n", n.osType)
		return nil
	}

	// Run the command and capture any errors
	if err := cmd.Run(); err != nil {
		// Log error to stderr but don't propagate it
		fmt.Fprintf(os.Stderr, "Failed to send desktop notification: %v\n", err)
		return nil
	}

	return nil
}

// SendBuildComplete sends a notification for a completed build.
// It formats the notification with job name, build number, and result.
func SendBuildComplete(job string, buildNumber int, result string) error {
	n := NewNotifier()
	title := "Jenkins Build Complete"
	message := fmt.Sprintf("Job: %s #%d - %s", job, buildNumber, result)
	return n.Send(title, message)
}
