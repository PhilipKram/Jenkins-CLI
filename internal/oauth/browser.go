package oauth

import (
	"os/exec"
	"runtime"
)

// openBrowserPlatform opens the given URL in the default browser.
func openBrowserPlatform(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
