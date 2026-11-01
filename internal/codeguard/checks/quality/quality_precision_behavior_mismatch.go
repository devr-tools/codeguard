package quality

import (
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

var readCallPattern = regexp.MustCompile(`(^|[.>:\-_])(?i:count|fetch|find|get|list|load|lookup|query|read|select|search)([A-Z_:\-.]|$)`)

func behaviorMismatch(file string, fn precisionFunction) bool {
	if isQualityFixturePath(file) {
		return false
	}
	if isFrameworkOrchestrationBoundary(file, fn) {
		return false
	}
	if isReactComponentOrNamedHookBoundary(file, fn) || isReactComponentOrHookBoundary(file, fn) {
		return false
	}
	if explicitMutationName(fn.Name) || isUIHelperOrMappingContext(file, fn) || isUICommandHelperName(file, fn.Name) || isDomainSideEffectBoundaryName(fn.Name) || isFactoryHelperName(fn.Name) || isSeedOrScriptSourcePath(file) || isAdapterOrOrchestrationFunction(file, fn) {
		return false
	}
	if hiddenSideEffect(file, fn) {
		return true
	}
	if !leadingCommandVerb(fn.Name) || mutatingFunctionEvidence(fn) {
		return false
	}
	for _, call := range fn.Calls {
		if readCallPattern.MatchString(call.Callee) {
			return true
		}
	}
	return false
}

func hiddenMutationEvidence(file string, fn precisionFunction) (mutationEvidence, bool) {
	if isQualityFixturePath(file) {
		return mutationEvidence{}, false
	}
	if isCPPConstructorOrDestructor(fn) {
		return mutationEvidence{}, false
	}
	if explicitMutationName(fn.Name) || isUICommandHelperName(file, fn.Name) || isDomainSideEffectBoundaryName(fn.Name) || isFrameworkOrchestrationBoundary(file, fn) || isScriptEntrypoint(file, fn.Name) || isSeedOrScriptSourcePath(file) || isAdapterOrOrchestrationFunction(file, fn) || isSecurityOrConfigUtilityFunction(file, fn) {
		return mutationEvidence{}, false
	}
	if isUIActionAssemblyFunction(file, fn) {
		return mutationEvidence{}, false
	}
	if isFactoryHelperName(fn.Name) {
		return mutationEvidence{}, false
	}
	if isReactComponentOrNamedHookBoundary(file, fn) {
		return mutationEvidence{}, false
	}
	evidence, hasEvidence := firstReportableMutationEvidence(fn)
	mutatesParam := hasEvidence && evidence.Target == targetArgument
	mutatesState := hasEvidence
	if isUIHelperOrMappingContext(file, fn) && !hasPersistentCollaboratorSideEffect(fn) {
		return mutationEvidence{}, false
	}
	if isPureComputationHelperName(fn.Name) && !mutatesParam && !hasPersistentCollaboratorSideEffect(fn) {
		return mutationEvidence{}, false
	}
	if isReactLocalStateBoundary(file, fn) && mutatesState && !mutatesParam && onlyReactHookLocalStateMutation(fn) {
		return mutationEvidence{}, false
	}
	if isPredicateName(fn.Name) && !mutatesParam && !predicateHasObviousSideEffect(fn) {
		return mutationEvidence{}, false
	}
	if isAccumulatorBuilderFunctionName(fn.Name) && !hasLikelyExternalMutationCall(fn) && !hasLikelyParameterAssignment(fn) {
		return mutationEvidence{}, false
	}
	if mutatesState || mutatesParam {
		return evidence, true
	}
	return mutationEvidence{}, false
}

func isCPPConstructorOrDestructor(fn precisionFunction) bool {
	if fn.Language != string(support.CLikeCPP) || fn.QualifiedOwner == "" {
		return false
	}
	owner := fn.QualifiedOwner
	if cut := strings.LastIndex(owner, "::"); cut >= 0 {
		owner = owner[cut+2:]
	}
	name := fn.Name
	if cut := strings.LastIndex(name, "::"); cut >= 0 {
		name = name[cut+2:]
	}
	return name == owner || name == "~"+owner
}

func predicateHasObviousSideEffect(fn precisionFunction) bool {
	body := strings.ToLower(fn.Body)
	return containsAny(body, []string{
		"await ", "fetch(", "prisma.", "db.", "client.", "repo.",
		".create(", ".delete(", ".insert(", ".remove(", ".save(", ".send(", ".set(", ".update(", ".upsert(",
		".push(", ".pop(", ".splice(", ".sort(", ".reverse(",
	})
}

func isUIActionAssemblyFunction(file string, fn precisionFunction) bool {
	if !isUIHelperOrMappingContext(file, fn) {
		return false
	}
	loweredName := strings.ToLower(strings.Trim(fn.Name, "_$"))
	if !strings.HasPrefix(loweredName, "build") || !strings.Contains(loweredName, "actions") {
		return false
	}
	body := strings.ToLower(fn.Body)
	return strings.Contains(body, "return {") &&
		containsAny(body, []string{"onchange", "onclick", "ondiscard", "onsave", "ontoggle", "onadd", "onremove", "onselect"})
}

func explicitMutationName(name string) bool {
	lowered := strings.ToLower(strings.TrimSpace(name))
	if cut := strings.LastIndex(lowered, "::"); cut >= 0 {
		lowered = lowered[cut+2:]
	}
	return leadingCommandVerb(name) ||
		conventionalMutationBoundaryPattern.MatchString(lowered) ||
		isEventHandlerName(name) ||
		strings.Contains(lowered, "mutat") ||
		strings.Contains(lowered, "persist") ||
		strings.Contains(lowered, "write")
}

var commandFunctionVerbs = map[string]struct{}{
	"add": {}, "allocate": {}, "append": {}, "apply": {}, "assign": {}, "bind": {}, "bulk": {},
	"cancel": {}, "capture": {}, "choose": {}, "clear": {}, "cleanup": {}, "close": {}, "configure": {}, "copy": {}, "create": {},
	"deactivate": {}, "delete": {}, "disable": {}, "discard": {}, "do": {},
	"emit": {}, "enable": {}, "exchange": {}, "expose": {}, "export": {},
	"fill": {}, "flip": {}, "flash": {}, "handle": {}, "hydrate": {}, "insert": {}, "issue": {},
	"link": {}, "log": {}, "mark": {}, "move": {}, "mutate": {}, "note": {}, "notify": {}, "open": {},
	"persist": {}, "publish": {}, "record": {}, "recompute": {}, "release": {}, "remember": {}, "remove": {}, "reset": {}, "retire": {}, "revert": {}, "revoke": {}, "rollback": {},
	"save": {}, "send": {}, "set": {}, "shutdown": {}, "store": {}, "submit": {}, "subscribe": {},
	"toggle": {}, "transfer": {}, "trigger": {}, "update": {}, "upsert": {}, "upload": {}, "walk": {}, "write": {},
}

func leadingCommandVerb(name string) bool {
	terminal := terminalCallIdentifier(strings.TrimSpace(name))
	words := identifierWords(terminal)
	if len(words) == 0 {
		return false
	}
	_, ok := commandFunctionVerbs[words[0]]
	return ok
}
