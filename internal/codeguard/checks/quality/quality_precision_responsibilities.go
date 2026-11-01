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

// responsibilityKeywords maps a call or statement to the first responsibility
// whose keywords it contains. Order matters: a text matching several rows is
// classified by the first.
var responsibilityKeywords = []struct {
	label    string
	keywords []string
}{
	{"validate", []string{"validat", "sanitize"}},
	{"authorize", []string{"auth", "permission", "allow"}},
	{"load", []string{"fetch", "find", "load", "query", "read", "select"}},
	{"write", []string{"save", "insert", "update", "delete", "persist", "write"}},
	{"send", []string{"send", "publish", "emit", "notify"}},
	{"cache", []string{"cache", "redis"}},
	{"transform", []string{"format", "map", "transform", "serialize", "json"}},
	{"observe", []string{"log", "metric", "trace"}},
}

func classifyResponsibility(text string, record func(string)) {
	for _, responsibility := range responsibilityKeywords {
		if containsAny(text, responsibility.keywords) {
			record(responsibility.label)
			return
		}
	}
}

var (
	orchestratorNameKeywords  = []string{"handler", "controller", "job", "worker"}
	orchestratorBodyKeywords  = []string{"request", "response"}
	infrastructureCallVerbs   = []string{"fetch", "save", "send", "cache", "publish", "query"}
	domainDecisionKeywords    = []string{"order", "user", "account", "price", "eligib", "valid"}
	renderingFunctionKeywords = []string{"return <", "jsx", "classname", "onclick", "onchange"}
)

func orchestrationDomainMix(file string, fn precisionFunction) bool {
	if isFrameworkOrchestrationBoundary(file, fn) || isUIHelperOrMappingContext(file, fn) ||
		isAdapterOrOrchestrationFunction(file, fn) || isSeedOrScriptSourcePath(file) {
		return false
	}
	body := strings.ToLower(fn.Body)
	if isReactHookName(fn.Name) || containsAny(body, renderingFunctionKeywords) {
		return false
	}
	if !isRequestOrJobOrchestrator(fn, body) {
		return false
	}
	return touchesInfrastructure(fn) && strings.Contains(body, "if ") && containsAny(body, domainDecisionKeywords)
}

// isRequestOrJobOrchestrator reports whether a function's name, signature,
// or lower-cased body marks it as a request handler, controller, or job.
func isRequestOrJobOrchestrator(fn precisionFunction, body string) bool {
	return containsAny(strings.ToLower(fn.Name), orchestratorNameKeywords) ||
		strings.Contains(fn.Signature, "Request") || strings.Contains(fn.Signature, "Response") ||
		containsAny(body, orchestratorBodyKeywords)
}

// touchesInfrastructure reports whether a function performs low-level
// infrastructure operations or calls fetch/save/send-style collaborators.
func touchesInfrastructure(fn precisionFunction) bool {
	if lowLevelOperationPattern.MatchString(fn.Body) {
		return true
	}
	for _, call := range fn.Calls {
		if containsAny(strings.ToLower(call.Callee), infrastructureCallVerbs) {
			return true
		}
	}
	return false
}
