package runner

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type scanCache struct {
	path    string
	entries map[string]cacheEntry
	dirty   bool
}

type cacheFile struct {
	Version int                   `json:"version"`
	Entries map[string]cacheEntry `json:"entries"`
}

type cacheEntry struct {
	FileHash   string         `json:"file_hash"`
	ConfigHash string         `json:"config_hash"`
	Findings   []core.Finding `json:"findings"`
}

const scanCacheVersion = 2

func cacheEnabled(cfg core.CacheConfig) bool {
	return cfg.Enabled != nil && *cfg.Enabled
}

func loadScanCache(path string) *scanCache {
	cache := &scanCache{
		path:    path,
		entries: map[string]cacheEntry{},
	}
	if strings.TrimSpace(path) == "" {
		return cache
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cache
	}
	var file cacheFile
	if err := json.Unmarshal(data, &file); err != nil {
		return cache
	}
	if file.Version != scanCacheVersion {
		return cache
	}
	if file.Entries != nil {
		cache.entries = file.Entries
	}
	return cache
}

func (cache *scanCache) save() error {
	if cache == nil || !cache.dirty || strings.TrimSpace(cache.path) == "" {
		return nil
	}
	payload := cacheFile{
		Version: scanCacheVersion,
		Entries: cache.entries,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cache.path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(cache.path, append(data, '\n'), 0o644); err != nil {
		return err
	}
	cache.dirty = false
	return nil
}

func cacheKey(sectionID string, targetPath string, rel string) string {
	return strings.Join([]string{sectionID, filepath.Clean(targetPath), filepath.ToSlash(rel)}, "|")
}

func hashBytes(data []byte) string {
	sum := sha1.Sum(data)
	return hex.EncodeToString(sum[:])
}

func configFingerprint(cfg core.Config) string {
	data, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return hashBytes(append([]byte("scanner-version-2|"), data...))
}

func cloneFindings(findings []core.Finding) []core.Finding {
	out := make([]core.Finding, len(findings))
	copy(out, findings)
	return out
}
