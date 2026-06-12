package runtime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Cache struct {
	path    string
	entries map[string]CachedVerdict
	dirty   bool
}

type cacheFile struct {
	Version int                      `json:"version"`
	Entries map[string]CachedVerdict `json:"entries"`
}

const cacheVersion = 1

func LoadCache(path string) *Cache {
	cache := &Cache{
		path:    path,
		entries: map[string]CachedVerdict{},
	}
	if strings.TrimSpace(path) == "" {
		return cache
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cache
	}
	var file cacheFile
	if err := json.Unmarshal(data, &file); err != nil || file.Version != cacheVersion {
		return cache
	}
	if file.Entries != nil {
		cache.entries = file.Entries
	}
	return cache
}

func (c *Cache) Get(key string) (CachedVerdict, bool) {
	if c == nil {
		return CachedVerdict{}, false
	}
	value, ok := c.entries[key]
	return value, ok
}

func (c *Cache) Put(key string, value CachedVerdict) {
	if c == nil {
		return
	}
	c.entries[key] = value
	c.dirty = true
}

func (c *Cache) Save() error {
	if c == nil || !c.dirty || strings.TrimSpace(c.path) == "" {
		return nil
	}
	payload := cacheFile{
		Version: cacheVersion,
		Entries: c.entries,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(c.path, append(data, '\n'), 0o644); err != nil {
		return err
	}
	c.dirty = false
	return nil
}
