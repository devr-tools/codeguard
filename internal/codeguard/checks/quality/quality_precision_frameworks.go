package quality

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

func isFrameworkOrchestrationBoundary(file string, fn precisionFunction) bool {
	return isFrameworkCommandBoundary(file, fn.Name) || isTrackedRouteBoundary(file, fn) || isNestJSRequestBoundary(file)
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
