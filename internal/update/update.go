package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const releaseURL = "https://api.github.com/repos/PhilipKram/Jenkins-CLI/releases/latest"

// ReleaseInfo holds data from the GitHub releases API.
type ReleaseInfo struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// CheckResult is returned when an update is available.
type CheckResult struct {
	CurrentVersion string
	LatestVersion  string
	HTMLURL        string
}

// Semver holds a parsed semantic version.
type Semver struct {
	Major, Minor, Patch int
}

// ParseSemver parses a version string like "v1.2.3" or "1.2.3".
func ParseSemver(s string) (Semver, error) {
	s = strings.TrimPrefix(s, "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) != 3 {
		return Semver{}, fmt.Errorf("invalid semver: %q", s)
	}
	// Strip any pre-release suffix from the patch component.
	parts[2] = strings.SplitN(parts[2], "-", 2)[0]

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Semver{}, fmt.Errorf("invalid semver major: %q", s)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Semver{}, fmt.Errorf("invalid semver minor: %q", s)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return Semver{}, fmt.Errorf("invalid semver patch: %q", s)
	}
	return Semver{Major: major, Minor: minor, Patch: patch}, nil
}

// IsNewer reports whether s is a newer version than other.
func (s Semver) IsNewer(other Semver) bool {
	if s.Major != other.Major {
		return s.Major > other.Major
	}
	if s.Minor != other.Minor {
		return s.Minor > other.Minor
	}
	return s.Patch > other.Patch
}

func (s Semver) String() string {
	return fmt.Sprintf("%d.%d.%d", s.Major, s.Minor, s.Patch)
}

// FetchLatestRelease queries GitHub for the latest release.
func FetchLatestRelease() (*ReleaseInfo, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(releaseURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	var info ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// Check fetches the latest release and returns a CheckResult if an update is available.
// Returns nil if the current version is "dev" or already up to date.
func Check(currentVersion string) *CheckResult {
	if currentVersion == "dev" {
		return nil
	}

	release, err := FetchLatestRelease()
	if err != nil {
		return nil
	}

	current, err := ParseSemver(currentVersion)
	if err != nil {
		return nil
	}

	latest, err := ParseSemver(release.TagName)
	if err != nil {
		return nil
	}

	if !latest.IsNewer(current) {
		return nil
	}

	return &CheckResult{
		CurrentVersion: current.String(),
		LatestVersion:  latest.String(),
		HTMLURL:        release.HTMLURL,
	}
}
