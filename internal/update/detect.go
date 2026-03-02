package update

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// InstallMethod describes how the CLI was installed.
type InstallMethod int

const (
	InstallUnknown  InstallMethod = iota
	InstallHomebrew               // installed via Homebrew
	InstallBinary                 // standalone binary (GitHub release / manual)
)

// DetectInstallMethod resolves symlinks on the current executable path
// to determine whether it was installed via Homebrew.
func DetectInstallMethod() InstallMethod {
	exe, err := os.Executable()
	if err != nil {
		return InstallUnknown
	}

	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return InstallUnknown
	}

	// Homebrew installs live under /Cellar/ (macOS) or /linuxbrew/ (Linux).
	if strings.Contains(resolved, "/Cellar/") || strings.Contains(resolved, "/linuxbrew/") {
		return InstallHomebrew
	}

	return InstallBinary
}

// ArchiveName returns the archive filename for a given version, matching the GoReleaser template.
func ArchiveName(ver string) string {
	ver = strings.TrimPrefix(ver, "v")
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("jenkins-cli_%s_%s_%s.%s", ver, runtime.GOOS, runtime.GOARCH, ext)
}

// DownloadURL returns the full GitHub release download URL for a given version.
func DownloadURL(ver string) string {
	ver = strings.TrimPrefix(ver, "v")
	return fmt.Sprintf("https://github.com/PhilipKram/Jenkins-CLI/releases/download/v%s/%s", ver, ArchiveName(ver))
}
