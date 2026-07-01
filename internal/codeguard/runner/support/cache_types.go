package support

import (
	"sync"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// ScanCache holds the persisted per-file findings cache plus the AI triage and
// natural-language rule verdict caches. Its maps are guarded by mu so sections
// can read and write concurrently while running in parallel.
type ScanCache struct {
	mu            sync.Mutex
	path          string
	entries       map[string]cacheEntry
	triageVerdict map[string]core.AITriageCacheVerdict
	nlRuleVerdict map[string]core.AINLRuleCacheVerdict
	dirty         bool
}

type cacheFile struct {
	Version       int                                  `json:"version"`
	Entries       map[string]cacheEntry                `json:"entries"`
	TriageVerdict map[string]core.AITriageCacheVerdict `json:"triage_verdicts,omitempty"`
	NLRuleVerdict map[string]core.AINLRuleCacheVerdict `json:"nl_rule_verdicts,omitempty"`
}

type cacheEntry struct {
	FileHash   string         `json:"file_hash"`
	ConfigHash string         `json:"config_hash"`
	Findings   []core.Finding `json:"findings"`
}

// scanCacheVersion is bumped whenever the on-disk cache layout or the meaning of
// a stored fingerprint changes, so stale caches are discarded wholesale rather
// than reused with mismatched semantics. v7 introduced per-section config
// fingerprints (see SectionConfigHashes).
const scanCacheVersion = 7
