package support

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type ScanCache struct {
	path          string
	entries       map[string]cacheEntry
	triageVerdict map[string]core.AITriageCacheVerdict
	dirty         bool
}

type cacheFile struct {
	Version       int                                  `json:"version"`
	Entries       map[string]cacheEntry                `json:"entries"`
	TriageVerdict map[string]core.AITriageCacheVerdict `json:"triage_verdicts,omitempty"`
}

type cacheEntry struct {
	FileHash   string         `json:"file_hash"`
	ConfigHash string         `json:"config_hash"`
	Findings   []core.Finding `json:"findings"`
}

const scanCacheVersion = 6

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

func cacheKey(sectionID string, targetPath string, rel string) string {
	return strings.Join([]string{sectionID, filepath.Clean(targetPath), filepath.ToSlash(rel)}, "|")
}

func hashBytes(data []byte) string {
	sum := sha1.Sum(data)
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
