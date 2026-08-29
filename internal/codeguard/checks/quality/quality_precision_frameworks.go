package quality

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

func isFrameworkOrchestrationBoundary(file string, fn precisionFunction) bool {
	return isFrameworkCommandBoundary(file, fn.Name) || isTrackedRouteBoundary(file, fn) || isNestJSRequestBoundary(file) || isGoRouterRegistrationBoundary(file, fn)
}

func isFrameworkConventionalAmbiguousName(file string, fn precisionFunction, name string) bool {
	if !isFrameworkOrchestrationBoundary(file, fn) {
		return false
	}
	switch strings.ToLower(strings.Trim(name, "_$")) {
	case "data", "value", "values", "item", "items":
		return true
	default:
		return false
	}
}

func isNestJSRequestBoundary(file string) bool {
	return isScriptLikeSourcePath(file) && support.IsNestJSBoundaryPath(file)
}

func isTrackedRouteBoundary(file string, fn precisionFunction) bool {
	if !isScriptLikeSourcePath(file) || !isNextRouteFile(file) {
		return false
	}
	loweredName := strings.ToLower(strings.Trim(fn.Name, "_$"))
	if isHTTPMethodName(fn.Name) {
		return true
	}
	if strings.HasSuffix(loweredName, "handler") {
		for _, method := range []string{"get", "post", "put", "patch", "delete", "head", "options"} {
			if strings.HasPrefix(loweredName, method) {
				return true
			}
		}
	}
	body := strings.ToLower(fn.Body)
	return strings.Contains(body, "nextresponse.") || strings.Contains(body, "response.json(")
}

func isGoRouterRegistrationBoundary(file string, fn precisionFunction) bool {
	if !strings.HasSuffix(strings.ToLower(file), ".go") {
		return false
	}
	loweredName := strings.ToLower(strings.Trim(fn.Name, "_$"))
	if loweredName == "registerroutes" || strings.HasPrefix(loweredName, "add") && strings.Contains(loweredName, "routes") {
		return hasRouterRegistrationEvidence(fn)
	}
	return false
}

func hasRouterRegistrationEvidence(fn precisionFunction) bool {
	signature := strings.ToLower(fn.Signature)
	if strings.Contains(signature, "chi.router") || strings.Contains(signature, "mux.router") || strings.Contains(signature, "gin.engine") {
		return true
	}
	for _, call := range fn.Calls {
		lowered := strings.ToLower(call.Callee)
		if strings.HasSuffix(lowered, ".route") || strings.HasSuffix(lowered, ".get") ||
			strings.HasSuffix(lowered, ".post") || strings.HasSuffix(lowered, ".put") ||
			strings.HasSuffix(lowered, ".patch") || strings.HasSuffix(lowered, ".delete") ||
			strings.HasSuffix(lowered, ".handle") || strings.HasSuffix(lowered, ".handlefunc") {
			return true
		}
	}
	return false
}
