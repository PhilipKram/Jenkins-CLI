package open

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"

	"github.com/PhilipKram/jenkins-cli/internal/config"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "open [job-name]",
	Short: "Open Jenkins in your browser",
	Long:  "Open the Jenkins dashboard or a specific job page in your default browser.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		url := cfg.URL
		if len(args) > 0 {
			url = cfg.URL + "/job/" + args[0]
		}

		fmt.Printf("Opening %s\n", url)
		return openBrowser(url)
	},
}

func validateURL(urlStr string) error {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s (only http and https are allowed)", parsedURL.Scheme)
	}
	if parsedURL.Host == "" {
		return fmt.Errorf("invalid URL: missing host")
	}
	return nil
}

func openBrowser(urlStr string) error {
	if err := validateURL(urlStr); err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", urlStr)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr)
	default:
		cmd = exec.Command("xdg-open", urlStr)
	}
	return cmd.Start()
}
