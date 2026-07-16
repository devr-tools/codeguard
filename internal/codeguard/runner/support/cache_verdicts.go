package support

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func (cache *ScanCache) GetTriageVerdict(contentHash string) (core.AITriageCacheVerdict, bool) {
	if cache == nil {
		return core.AITriageCacheVerdict{}, false
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	verdict, ok := cache.triageVerdict[contentHash]
	return verdict, ok
}

func (cache *ScanCache) PutTriageVerdict(contentHash string, verdict core.AITriageCacheVerdict) {
	if cache == nil || strings.TrimSpace(contentHash) == "" {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.triageVerdict == nil {
		cache.triageVerdict = map[string]core.AITriageCacheVerdict{}
	}
	cache.triageVerdict[contentHash] = verdict
	cache.dirty = true
}

func (cache *ScanCache) GetNLRuleVerdict(key string) (core.AINLRuleCacheVerdict, bool) {
	if cache == nil {
		return core.AINLRuleCacheVerdict{}, false
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	verdict, ok := cache.nlRuleVerdict[key]
	return verdict, ok
}

func (cache *ScanCache) PutNLRuleVerdict(key string, verdict core.AINLRuleCacheVerdict) {
	if cache == nil || strings.TrimSpace(key) == "" {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.nlRuleVerdict == nil {
		cache.nlRuleVerdict = map[string]core.AINLRuleCacheVerdict{}
	}
	cache.nlRuleVerdict[key] = verdict
	cache.dirty = true
}
