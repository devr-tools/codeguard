package runtime

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/cachefile"
)

type Cache struct {
	path    string
	entries map[string]CachedVerdict
	dirty   bool
}

const cacheVersion = 1

func LoadCache(path string) *Cache {
	cache := &Cache{
		path:    path,
		entries: map[string]CachedVerdict{},
	}
	if entries := cachefile.LoadEntries[CachedVerdict](path, cacheVersion); entries != nil {
		cache.entries = entries
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
	if err := cachefile.WriteEntries(c.path, cacheVersion, c.entries); err != nil {
		return err
	}
	c.dirty = false
	return nil
}
