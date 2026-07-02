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
	// version 8: findings gained ContextFingerprint, so entries cached by
	// earlier scanners must be recomputed rather than replayed without it.
	prefix := "scanner-version-8|" + strings.Join(extras, "|") + "|"
	return hashBytes(append([]byte(prefix), data...))
}

// SectionConfigHashes builds a fingerprint per config family, hashing only the
// settings that can change that family's per-file findings. This lets an edit to
// one section's rules (or to a finding-irrelevant field such as the output
// format, target names, or the exclude list) reuse cached findings for every
// unaffected section. The rule catalog is included in every family because a
// rule-metadata override can change any finding's level or title through
// NewFinding. The "" key is the conservative all-checks fallback used for any
// section id that sectionConfigFamily does not recognize, so a newly added
// section can never silently serve stale cache entries.
func SectionConfigHashes(cfg core.Config, catalog map[string]core.RuleMetadata, extras ...string) map[string]string {
	// v2: findings gained ContextFingerprint (see ConfigFingerprint).
	prefix := "section-config-v2|" + strings.Join(extras, "|") + "|"
	checks := cfg.Checks
	return map[string]string{
		// quality reads both QualityRules and DesignRules, and its AI-quality
		// findings depend on the AI config.
		"quality":   sectionFingerprint(prefix, "quality", catalog, cfg.AI, checks.QualityRules, checks.DesignRules),
		"design":    sectionFingerprint(prefix, "design", catalog, checks.DesignRules),
		"security":  sectionFingerprint(prefix, "security", catalog, checks.SecurityRules),
		"prompts":   sectionFingerprint(prefix, "prompts", catalog, checks.PromptRules),
		"ci":        sectionFingerprint(prefix, "ci", catalog, checks.CIRules),
		"contracts": sectionFingerprint(prefix, "contracts", catalog, checks.ContractRules),
		"":          sectionFingerprint(prefix, "all", catalog, cfg.AI, checks),
	}
}

// sectionConfigFamily maps a per-file cache section id to the config family
// whose settings can change that section's findings. Compound ids share the
// family of their prefix (e.g. "quality-clone" and "security-secrets"); any
// unrecognized id maps to "" so it falls back to the all-checks fingerprint.
func sectionConfigFamily(sectionID string) string {
	if i := strings.IndexByte(sectionID, '-'); i >= 0 {
		sectionID = sectionID[:i]
	}
	switch sectionID {
	case "quality", "design", "security", "prompts", "ci", "contracts":
		return sectionID
	default:
		return ""
	}
}

// sectionConfigHash resolves the fingerprint for a section id, preferring the
// scoped per-family hash and falling back to the all-checks hash, then to the
// legacy ConfigHash for Contexts assembled without SectionConfigHash.
func (sc Context) sectionConfigHash(sectionID string) string {
	if sc.SectionConfigHash != nil {
		if hash, ok := sc.SectionConfigHash[sectionConfigFamily(sectionID)]; ok {
			return hash
		}
		if hash, ok := sc.SectionConfigHash[""]; ok {
			return hash
		}
	}
	return sc.ConfigHash
}

func sectionFingerprint(prefix string, family string, components ...any) string {
	data, err := json.Marshal(components)
	if err != nil {
		return ""
	}
	return hashBytes(append([]byte(prefix+family+"|"), data...))
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
