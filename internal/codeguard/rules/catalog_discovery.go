package rules

import (
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// sectionKeysByDisplayName exists because catalog entries carry a display
// name while configuration speaks in section ids; without it, explain could
// not name the checks.<id> toggle and `rules -enabled` could not evaluate it.
// Sub-families such as "Code Quality / Local Precision" run inside their
// parent section, which tests/rules/examples_test.go confirms per example.
var sectionKeysByDisplayName = map[string]string{
	"Code Quality":                   "quality",
	"Code Quality / Local Precision": "quality",
	"Code Smells / History":          "quality",
	"Maintainability History":        "quality",
	"Maintainability Delta":          "quality",
	"Performance":                    "performance",
	"Design Patterns":                "design",
	"Security":                       "security",
	"AI Prompts":                     "prompts",
	"CI/CD":                          "ci",
	"Supply Chain":                   "supply_chain",
	"Delivery":                       "delivery",
	"Reliability":                    "reliability",
	"Data Correctness":               "data",
	"Observability":                  "observability",
	"Operations":                     "operations",
	"Change Safety":                  "change",
	"Change Safety / Testability":    "change",
	"Change Safety / Refactors":      "change",
	"API Contracts":                  "contracts",
	"Agent Context":                  "context",
	"Custom Rules":                   "custom",
}

// SectionKey returns the section id a rule reports under (quality, security,
// supply_chain, ...), or "" when its display section is not a known one.
// Custom rule-pack rules with a non-standard section report "custom".
func SectionKey(meta core.RuleMetadata) string {
	if key, ok := sectionKeysByDisplayName[strings.TrimSpace(meta.Section)]; ok {
		return key
	}
	return ""
}

// sectionToggleKeys are the checks.<section> switches. Every catalogued rule
// whose section has one lists it as an enable key by convention.
var sectionToggleKeys = map[string]string{
	"quality":       "checks.quality",
	"performance":   "checks.performance",
	"design":        "checks.design",
	"security":      "checks.security",
	"prompts":       "checks.prompts",
	"ci":            "checks.ci",
	"supply_chain":  "checks.supply_chain",
	"delivery":      "checks.delivery",
	"reliability":   "checks.reliability",
	"data":          "checks.data",
	"observability": "checks.observability",
	"operations":    "checks.operations",
	"change":        "checks.change",
	"contracts":     "checks.contracts",
	"context":       "checks.context",
}

// SectionToggleKey returns the checks.<section> key that turns a section on
// or off, or "" when the section has none (custom rules).
func SectionToggleKey(sectionKey string) string {
	return sectionToggleKeys[sectionKey]
}

// RuleDocsAnchor returns the docs/checks.md anchor for a rule id: "rule-"
// followed by the id with dots replaced by dashes, the scheme shared with
// SARIF helpUri links.
func RuleDocsAnchor(ruleID string) string {
	return "rule-" + strings.ReplaceAll(strings.TrimSpace(ruleID), ".", "-")
}

// RuleDocsPath returns the repository-relative docs link for a rule.
func RuleDocsPath(ruleID string) string {
	return "docs/checks.md#" + RuleDocsAnchor(ruleID)
}

// AliasesFor lists the retired ids that resolve to ruleID, sorted.
func AliasesFor(ruleID string) []string {
	out := make([]string, 0)
	for alias, target := range ruleAliases {
		if target == ruleID {
			out = append(out, alias)
		}
	}
	sort.Strings(out)
	return out
}

// applyRuleDiscovery attaches the explain-oriented metadata kept outside the
// catalog entries: verified examples, typical confidence, and controlling
// config keys (the section toggle by convention, then explicit entries).
func applyRuleDiscovery(meta core.RuleMetadata) core.RuleMetadata {
	if len(meta.Examples) == 0 {
		meta.Examples = cloneExamples(ruleExamples[meta.ID])
	}
	if meta.Confidence == "" {
		meta.Confidence = ruleConfidence[meta.ID]
	}
	if len(meta.ConfigKeys) == 0 {
		meta.ConfigKeys = ruleConfigKeysFor(meta)
	}
	return meta
}

func ruleConfigKeysFor(meta core.RuleMetadata) []core.RuleConfigKey {
	keys := make([]core.RuleConfigKey, 0, 4)
	seen := map[string]bool{}
	add := func(key core.RuleConfigKey) {
		if key.Key == "" || seen[key.Key] {
			return
		}
		seen[key.Key] = true
		keys = append(keys, key)
	}
	if toggle := SectionToggleKey(SectionKey(meta)); toggle != "" {
		add(core.RuleConfigKey{Key: toggle, Role: core.RuleConfigKeyRoleEnable})
	}
	for _, key := range explicitRuleConfigKeys[meta.ID] {
		add(key)
	}
	if len(keys) == 0 {
		return nil
	}
	return keys
}

func cloneExamples(examples []core.RuleExample) []core.RuleExample {
	if len(examples) == 0 {
		return nil
	}
	out := make([]core.RuleExample, len(examples))
	for i, example := range examples {
		out[i] = example
		out[i].Overrides = append([]string(nil), example.Overrides...)
	}
	return out
}
