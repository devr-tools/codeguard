package whatsnew

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func updateCheckDisabled() bool {
	if isTruthy(os.Getenv(disableEnv)) {
		return true
	}
	return strings.TrimSpace(os.Getenv("CI")) != ""
}

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

type cacheEntry struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

func (c *UpdateChecker) cachePath() string {
	if strings.TrimSpace(c.CacheDir) == "" {
		return ""
	}
	return filepath.Join(c.CacheDir, cacheFileName)
}

func (c *UpdateChecker) readCache() (cacheEntry, bool) {
	path := c.cachePath()
	if path == "" {
		return cacheEntry{}, false
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is derived from os.UserCacheDir, not user input.
	if err != nil {
		return cacheEntry{}, false
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return cacheEntry{}, false
	}
	return entry, true
}

func (c *UpdateChecker) writeCache(entry cacheEntry) {
	path := c.cachePath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(c.CacheDir, 0o700); err != nil {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o600)
}
