package support

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func cacheKey(sectionID string, targetPath string, rel string) string {
	return strings.Join([]string{sectionID, filepath.Clean(targetPath), filepath.ToSlash(rel)}, "|")
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func ConfigFingerprint(cfg core.Config, extras ...string) string {
	data, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	prefix := "scanner-version-6|" + strings.Join(extras, "|") + "|"
	return hashBytes(append([]byte(prefix), data...))
}

func cloneFindings(findings []core.Finding) []core.Finding {
	out := make([]core.Finding, len(findings))
	copy(out, findings)
	return out
}

func (cache *ScanCache) GetTriageVerdict(contentHash string) (core.AITriageCacheVerdict, bool) {
	if cache == nil {
		return core.AITriageCacheVerdict{}, false
	}
	verdict, ok := cache.triageVerdict[contentHash]
	return verdict, ok
}

func (cache *ScanCache) PutTriageVerdict(contentHash string, verdict core.AITriageCacheVerdict) {
	if cache == nil || strings.TrimSpace(contentHash) == "" {
		return
	}
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
	verdict, ok := cache.nlRuleVerdict[key]
	return verdict, ok
}

func (cache *ScanCache) PutNLRuleVerdict(key string, verdict core.AINLRuleCacheVerdict) {
	if cache == nil || strings.TrimSpace(key) == "" {
		return
	}
	if cache.nlRuleVerdict == nil {
		cache.nlRuleVerdict = map[string]core.AINLRuleCacheVerdict{}
	}
	cache.nlRuleVerdict[key] = verdict
	cache.dirty = true
}
