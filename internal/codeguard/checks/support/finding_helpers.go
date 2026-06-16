package support

import "github.com/devr-tools/codeguard/internal/codeguard/core"

func DedupeFindings(findings []core.Finding, keyFn func(core.Finding) string) []core.Finding {
	if len(findings) <= 1 {
		return findings
	}
	seen := make(map[string]struct{}, len(findings))
	deduped := make([]core.Finding, 0, len(findings))
	for _, finding := range findings {
		key := keyFn(finding)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, finding)
	}
	return deduped
}
