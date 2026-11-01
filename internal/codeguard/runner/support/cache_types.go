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
	FileHash   string `json:"file_hash"`
	ConfigHash string `json:"config_hash"`
	// DepsHash digests the cross-file inputs (sibling sources, manifests,
	// target-level state) a dependency-declaring evaluator read. Empty for
	// evaluators that depend on the file alone, so their entries are unchanged.
	DepsHash string         `json:"deps_hash,omitempty"`
	Findings []core.Finding `json:"findings"`
	// Diagnostics are the per-file diagnostics an evaluator reported beside
	// its findings (for example suppressed synthetic credential fixtures).
	Diagnostics []core.Diagnostic `json:"diagnostics,omitempty"`
}

// scanCacheVersion is bumped whenever the on-disk cache layout or the meaning of
// a stored fingerprint changes, so stale caches are discarded wholesale rather
// than reused with mismatched semantics. v8 introduced content fingerprints for
// path-insensitive baseline suppression after code moves. v9 stores per-file
// diagnostics; a v8 entry has none recorded, and replaying it would silently
// drop them.
const scanCacheVersion = 9
