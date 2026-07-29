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
	if strings.HasPrefix(lowered, "build") && containsAny(lowered, []string{"editdata", "finalization", "offboard", "request", "transaction"}) {
		return true
	}
	for _, prefix := range []string{"assist", "capture", "chat", "cleanup", "copy", "deliver", "exchange", "export", "hydrate", "link", "mitigation", "next", "notify", "summarize", "trigger", "user", "walk"} {
		if strings.HasPrefix(lowered, prefix) {
			return true
		}
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
	if strings.Contains(normalized, "error-monitor") {
		return true
	}
	return containsAny(normalized, []string{"/adapters/", "/adapter/", "/connectors/", "/connector/", "/integrations/", "integrations/", "/webhooks/", "/slack/", "/jobs/", "/routers/digest/"})
}

func isSecurityOrConfigUtilityFunction(file string, fn precisionFunction) bool {
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	loweredName := strings.ToLower(strings.Trim(fn.Name, "_$"))
	if strings.Contains(normalized, "/auth/") || strings.Contains(normalized, "/crypto/") ||
		strings.Contains(normalized, "oauth") || strings.Contains(normalized, "ava") {
		return true
	}
	return containsAny(loweredName, []string{"origin", "env", "policy", "publickey", "sign", "verify", "decode"})
}

func isAdapterOrchestrationName(name string) bool {
	loweredName := strings.ToLower(strings.Trim(name, "_$"))
	return containsAny(loweredName, []string{"abuseconfig", "abuse_config", "bugreport", "bug_report", "slack", "webhook", "adapter"})
}

func isPackageAPIRouterPath(file string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	return strings.HasPrefix(normalized, "packages/api/src/routers/") ||
		strings.Contains(normalized, "/packages/api/src/routers/")
}

func isIntegrationAdapterPath(file string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	return strings.HasPrefix(normalized, "packages/integrations/") ||
		strings.Contains(normalized, "/packages/integrations/") ||
		strings.Contains(normalized, "/integrations/")
}

func configuredPluralDomainAbbreviation(name string) bool {
	switch name {
	case "docs", "krs":
		return true
	default:
		return false
	}
}
