package quality

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

func isFrameworkOrchestrationBoundary(file string, fn precisionFunction) bool {
	return isFrameworkCommandBoundary(file, fn.Name) || isNestJSRequestBoundary(file)
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
