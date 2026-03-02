package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const cacheTTL = 24 * time.Hour

type cachedCheck struct {
	CheckedAt      time.Time    `json:"checked_at"`
	CurrentVersion string       `json:"current_version"`
	Result         *CheckResult `json:"result"`
}

func cacheFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".jenkins-cli", "update-check.json")
}

// LoadCache returns a cached check result, or nil if the cache is missing, corrupt, or expired.
func LoadCache(currentVersion string) *cachedCheck {
	path := cacheFilePath()
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var c cachedCheck
	if err := json.Unmarshal(data, &c); err != nil {
		return nil
	}

	// Invalidate if version changed (e.g. after upgrading).
	if c.CurrentVersion != currentVersion {
		return nil
	}

	if time.Since(c.CheckedAt) > cacheTTL {
		return nil
	}

	return &c
}

// SaveCache writes the check result to the cache file.
func SaveCache(currentVersion string, result *CheckResult) {
	path := cacheFilePath()
	if path == "" {
		return
	}

	c := cachedCheck{
		CheckedAt:      time.Now(),
		CurrentVersion: currentVersion,
		Result:         result,
	}

	data, err := json.Marshal(c)
	if err != nil {
		return
	}

	_ = os.MkdirAll(filepath.Dir(path), 0755)
	_ = os.WriteFile(path, data, 0600)
}

// CheckWithCache performs an update check, using a 24h cache to avoid excessive API calls.
func CheckWithCache(currentVersion string) *CheckResult {
	if currentVersion == "dev" {
		return nil
	}

	if cached := LoadCache(currentVersion); cached != nil {
		return cached.Result
	}

	result := Check(currentVersion)
	SaveCache(currentVersion, result)
	return result
}
