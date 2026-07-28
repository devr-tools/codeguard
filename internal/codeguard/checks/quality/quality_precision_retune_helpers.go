package quality

import "strings"

func isDomainSideEffectBoundaryName(name string) bool {
	lowered := strings.ToLower(strings.TrimSpace(name))
	if lowered == "" {
		return false
	}
	if strings.HasPrefix(lowered, "maybe") && containsAny(lowered, []string{"alert", "notify", "record", "track", "emit"}) {
		return true
	}
	if strings.HasPrefix(lowered, "evaluate") && containsAny(lowered, []string{"abuse", "policy", "rule", "risk", "fraud", "quota", "limit"}) {
		return true
	}
	if strings.HasPrefix(lowered, "load") && containsAny(lowered, []string{"config", "defaults", "settings", "policy"}) {
		return true
	}
	return false
}

func isAdapterOrOrchestrationFunction(file string, fn precisionFunction) bool {
	loweredName := strings.ToLower(strings.Trim(fn.Name, "_$"))
	if containsAny(loweredName, []string{"adapter", "bugreport", "bug_report", "slack", "webhook", "sync", "abuseconfig", "abuse_config"}) {
		return true
	}
	if strings.HasPrefix(loweredName, "save") || strings.HasPrefix(loweredName, "insert") || strings.HasPrefix(loweredName, "post") ||
		strings.HasPrefix(loweredName, "send") || strings.HasPrefix(loweredName, "publish") || strings.HasPrefix(loweredName, "record") {
		if containsAny(loweredName, []string{"config", "report", "slack", "webhook", "audit", "event", "job"}) {
			return true
		}
	}
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	return containsAny(normalized, []string{"/adapters/", "/adapter/", "/connectors/", "/connector/", "/integrations/", "/webhooks/", "/slack/", "/jobs/"})
}

func isAdapterOrchestrationName(name string) bool {
	loweredName := strings.ToLower(strings.Trim(name, "_$"))
	return containsAny(loweredName, []string{"abuseconfig", "abuse_config", "bugreport", "bug_report", "slack", "webhook", "adapter"})
}

func configuredPluralDomainAbbreviation(name string) bool {
	switch name {
	case "docs", "krs":
		return true
	default:
		return false
	}
}
