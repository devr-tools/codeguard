package support

import "github.com/devr-tools/codeguard/internal/codeguard/core"

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
