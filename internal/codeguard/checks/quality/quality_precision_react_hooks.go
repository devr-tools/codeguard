package quality

import (
	"regexp"
	"strings"
	"unicode"
)

var reactHookNamePattern = regexp.MustCompile(`^use[A-Z0-9]`)

func onlyReactHookLocalStateMutation(fn precisionFunction) bool {
	sawLocalStateMutation := false
	for _, assignment := range fn.Assignments {
		if assignment.Augmented {
			return false
		}
	}
	for _, call := range fn.Calls {
		if !mutatingCallPattern.MatchString(call.Callee) {
			continue
		}
		if !isReactLocalStateMutationCall(call.Callee) {
			return false
		}
		sawLocalStateMutation = true
	}
	return sawLocalStateMutation
}

func isReactHookStateBoundary(file string, fn precisionFunction) bool {
	if !isScriptLikeSourcePath(file) {
		return false
	}
	if isReactHookName(fn.Name) {
		return functionUsesReactState(fn) || callsReactLocalStateSetter(fn)
	}
	if isReactHookFile(file) {
		return callsReactLocalStateSetter(fn)
	}
	return false
}

func isReactLocalStateBoundary(file string, fn precisionFunction) bool {
	if isReactHookStateBoundary(file, fn) {
		return true
	}
	return isReactComponentOrHookBoundary(file, fn) && callsReactLocalStateSetter(fn)
}

func isScriptLikeSourcePath(file string) bool {
	lowered := strings.ToLower(file)
	return strings.HasSuffix(lowered, ".ts") || strings.HasSuffix(lowered, ".tsx") ||
		strings.HasSuffix(lowered, ".js") || strings.HasSuffix(lowered, ".jsx")
}

func isReactHookFile(file string) bool {
	normalized := strings.ReplaceAll(file, "\\", "/")
	base := normalized
	if slash := strings.LastIndex(base, "/"); slash >= 0 {
		base = base[slash+1:]
	}
	lowered := strings.ToLower(base)
	return strings.HasPrefix(lowered, "use-") || reactHookNamePattern.MatchString(base)
}

func isReactHookName(name string) bool {
	return reactHookNamePattern.MatchString(strings.TrimSpace(name))
}

func functionUsesReactState(fn precisionFunction) bool {
	for _, call := range fn.Calls {
		if call.Callee == "useState" || call.Callee == "React.useState" || call.Callee == "useReducer" || call.Callee == "React.useReducer" {
			return true
		}
	}
	return strings.Contains(fn.Body, "useState(") || strings.Contains(fn.Body, "useReducer(")
}

func callsReactLocalStateSetter(fn precisionFunction) bool {
	for _, call := range fn.Calls {
		if isReactLocalStateMutationCall(call.Callee) {
			return true
		}
	}
	return false
}

func isReactLocalStateMutationCall(callee string) bool {
	if strings.ContainsAny(callee, ".:>") {
		return false
	}
	if callee == "dispatch" {
		return true
	}
	if len(callee) <= len("set") || !strings.HasPrefix(callee, "set") {
		return false
	}
	next := rune(callee[len("set")])
	return unicode.IsUpper(next) || unicode.IsDigit(next)
}
