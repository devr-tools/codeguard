package support

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func CacheEnabled(cfg core.CacheConfig) bool {
	return cfg.Enabled != nil && *cfg.Enabled
}

func LoadScanCache(path string) *ScanCache {
	cache := &ScanCache{
		path:          path,
		entries:       map[string]cacheEntry{},
		triageVerdict: map[string]core.AITriageCacheVerdict{},
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
	if file.TriageVerdict != nil {
		cache.triageVerdict = file.TriageVerdict
	}
	return cache
}

func (cache *ScanCache) Save() error {
	if cache == nil || !cache.dirty || strings.TrimSpace(cache.path) == "" {
		return nil
	}
	payload := cacheFile{
		Version:       scanCacheVersion,
		Entries:       cache.entries,
		TriageVerdict: cache.triageVerdict,
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
