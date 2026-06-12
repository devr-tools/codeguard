package support

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/cachefile"
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
		nlRuleVerdict: map[string]core.AINLRuleCacheVerdict{},
	}
	var file cacheFile
	if !cachefile.Load(path, &file) || file.Version != scanCacheVersion {
		return cache
	}
	if file.Entries != nil {
		cache.entries = file.Entries
	}
	if file.TriageVerdict != nil {
		cache.triageVerdict = file.TriageVerdict
	}
	if file.NLRuleVerdict != nil {
		cache.nlRuleVerdict = file.NLRuleVerdict
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
		NLRuleVerdict: cache.nlRuleVerdict,
	}
	if err := cachefile.Write(cache.path, payload); err != nil {
		return err
	}
	cache.dirty = false
	return nil
}
