package semantic

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/cachefile"
)

const (
	requestVersion = 1
	cacheVersion   = 1
)

type verdictCache struct {
	path    string
	entries map[string]cacheEntry
	dirty   bool
}

type cacheEntry struct {
	Response Response `json:"response"`
}

func loadVerdictCache(path string) *verdictCache {
	cache := &verdictCache{
		path:    path,
		entries: map[string]cacheEntry{},
	}
	if entries := cachefile.LoadEntries[cacheEntry](path, cacheVersion); entries != nil {
		cache.entries = entries
	}
	return cache
}

func (cache *verdictCache) save() error {
	if cache == nil || !cache.dirty || strings.TrimSpace(cache.path) == "" {
		return nil
	}
	if err := cachefile.WriteEntries(cache.path, cacheVersion, cache.entries); err != nil {
		return err
	}
	cache.dirty = false
	return nil
}

func requestHash(req Request) string {
	payload := struct {
		Version      int            `json:"version"`
		Runtime      string         `json:"runtime"`
		TargetName   string         `json:"target_name"`
		Language     string         `json:"language"`
		BaseRef      string         `json:"base_ref,omitempty"`
		Diff         string         `json:"diff,omitempty"`
		ChangedFiles []string       `json:"changed_files,omitempty"`
		Checks       []CheckSpec    `json:"checks"`
		SourceFiles  []FileSnapshot `json:"source_files,omitempty"`
		TestFiles    []FileSnapshot `json:"test_files,omitempty"`
	}{
		Version:      req.Version,
		Runtime:      req.Runtime,
		TargetName:   req.TargetName,
		Language:     req.Language,
		BaseRef:      req.BaseRef,
		Diff:         req.Diff,
		ChangedFiles: req.ChangedFiles,
		Checks:       req.Checks,
		SourceFiles:  req.SourceFiles,
		TestFiles:    req.TestFiles,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha1.Sum(append([]byte("semantic-request-v1|"), data...))
	return hex.EncodeToString(sum[:])
}

func CachePathForBase(base string) string {
	trimmed := strings.TrimSpace(base)
	if trimmed == "" {
		return ""
	}
	ext := filepath.Ext(trimmed)
	if ext == "" {
		return trimmed + ".semantic"
	}
	return strings.TrimSuffix(trimmed, ext) + ".semantic" + ext
}
