package quality

import (
	"sort"
	"strings"
)

func responsibilityCount(fn precisionFunction) (int, []string) {
	if isAdapterOrchestrationName(fn.Name) {
		return 0, nil
	}
	seen := map[string]struct{}{}
	record := func(label string) {
		seen[label] = struct{}{}
	}
	body := strings.ToLower(fn.Body)
	for _, call := range fn.Calls {
		classifyResponsibility(strings.ToLower(call.Callee), record)
	}
	for _, statement := range fn.Statements {
		classifyResponsibility(strings.ToLower(statement.Text), record)
	}
	if strings.Contains(body, " if ") || strings.Contains(body, "\tif ") || strings.Contains(body, "\nif ") {
		record("decision")
	}
	labels := make([]string, 0, len(seen))
	for label := range seen {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	return len(labels), labels
}

func responsibilityThreshold(file string, fn precisionFunction) int {
	if isReactComponentOrHookBoundary(file, fn) {
		return 7
	}
	return 6
}

func classifyResponsibility(text string, record func(string)) {
	switch {
	case strings.Contains(text, "validat") || strings.Contains(text, "sanitize"):
		record("validate")
	case strings.Contains(text, "auth") || strings.Contains(text, "permission") || strings.Contains(text, "allow"):
		record("authorize")
	case strings.Contains(text, "fetch") || strings.Contains(text, "find") || strings.Contains(text, "load") || strings.Contains(text, "query") || strings.Contains(text, "read") || strings.Contains(text, "select"):
		record("load")
	case strings.Contains(text, "save") || strings.Contains(text, "insert") || strings.Contains(text, "update") || strings.Contains(text, "delete") || strings.Contains(text, "persist") || strings.Contains(text, "write"):
		record("write")
	case strings.Contains(text, "send") || strings.Contains(text, "publish") || strings.Contains(text, "emit") || strings.Contains(text, "notify"):
		record("send")
	case strings.Contains(text, "cache") || strings.Contains(text, "redis"):
		record("cache")
	case strings.Contains(text, "format") || strings.Contains(text, "map") || strings.Contains(text, "transform") || strings.Contains(text, "serialize") || strings.Contains(text, "json"):
		record("transform")
	case strings.Contains(text, "log") || strings.Contains(text, "metric") || strings.Contains(text, "trace"):
		record("observe")
	}
}

func orchestrationDomainMix(file string, fn precisionFunction) bool {
	if isFrameworkOrchestrationBoundary(file, fn) || isUIHelperOrMappingContext(file, fn) ||
		isAdapterOrOrchestrationFunction(file, fn) || isSeedOrScriptSourcePath(file) {
		return false
	}
	name := strings.ToLower(fn.Name)
	body := strings.ToLower(fn.Body)
	if isReactHookName(fn.Name) || containsAny(body, []string{"return <", "jsx", "classname", "onclick", "onchange"}) {
		return false
	}
	orchestrator := strings.Contains(name, "handler") || strings.Contains(name, "controller") || strings.Contains(name, "job") ||
		strings.Contains(name, "worker") || strings.Contains(fn.Signature, "Request") || strings.Contains(fn.Signature, "Response") ||
		strings.Contains(body, "request") || strings.Contains(body, "response")
	if !orchestrator {
		return false
	}
	hasInfra := lowLevelOperationPattern.MatchString(fn.Body)
	for _, call := range fn.Calls {
		lowered := strings.ToLower(call.Callee)
		if strings.Contains(lowered, "fetch") || strings.Contains(lowered, "save") || strings.Contains(lowered, "send") ||
			strings.Contains(lowered, "cache") || strings.Contains(lowered, "publish") || strings.Contains(lowered, "query") {
			hasInfra = true
			break
		}
	}
	hasDomainDecision := strings.Contains(body, "if ") && (strings.Contains(body, "order") || strings.Contains(body, "user") ||
		strings.Contains(body, "account") || strings.Contains(body, "price") || strings.Contains(body, "eligib") ||
		strings.Contains(body, "valid"))
	return hasInfra && hasDomainDecision
}
