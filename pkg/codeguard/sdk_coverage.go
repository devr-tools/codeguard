package codeguard

import (
	"github.com/devr-tools/codeguard/internal/codeguard/config"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// OWASPCoverageEntry records which rules cover an OWASP Top 10 (2021) category.
type OWASPCoverageEntry = core.OWASPCoverageEntry

// OWASPCoverage reports OWASP Top 10 (2021) coverage for the built-in rules.
func OWASPCoverage() []OWASPCoverageEntry {
	return core.OWASPCoverageForRules(config.RuleList())
}

// OWASPCoverageForConfig reports OWASP Top 10 (2021) coverage for the rules
// active under cfg (including custom rule packs).
func OWASPCoverageForConfig(cfg Config) []OWASPCoverageEntry {
	return core.OWASPCoverageForRules(config.RuleListForConfig(cfg))
}
