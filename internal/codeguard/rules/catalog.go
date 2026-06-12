package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var catalog = mergeRuleCatalogs(
	qualityCatalog,
	designCatalog,
	securityCatalog,
	securityTaintCatalog,
	miscCatalog,
)

func Catalog() map[string]core.RuleMetadata {
	out := make(map[string]core.RuleMetadata, len(catalog))
	for id, meta := range catalog {
		out[id] = core.NormalizeRuleMetadata(meta)
	}
	return out
}

func mergeRuleCatalogs(parts ...map[string]core.RuleMetadata) map[string]core.RuleMetadata {
	total := 0
	for _, part := range parts {
		total += len(part)
	}
	merged := make(map[string]core.RuleMetadata, total)
	for _, part := range parts {
		for id, meta := range part {
			merged[id] = meta
		}
	}
	return merged
}
