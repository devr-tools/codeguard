package semantic

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/cachefile"
)

const (
	requestVersion = 6
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
	data, err := json.Marshal(req.hashPayload())
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(append([]byte("semantic-request-v1|"), data...))
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
