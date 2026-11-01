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

// hiddenMutationExemptions cover functions whose name, location, or role
// announces that they mutate state.
var hiddenMutationExemptions = []precisionExemption{
	exemptPath(isQualityFixturePath),
	func(_ string, fn precisionFunction) bool { return isCPPConstructorOrDestructor(fn) },
	exemptName(explicitMutationName),
	exemptFileAndName(isUICommandHelperName),
	exemptName(isDomainSideEffectBoundaryName),
	isFrameworkOrchestrationBoundary,
	exemptFileAndName(isScriptEntrypoint),
	exemptPath(isSeedOrScriptSourcePath),
	isAdapterOrOrchestrationFunction,
	isSecurityOrConfigUtilityFunction,
	isUIActionAssemblyFunction,
	exemptName(isFactoryHelperName),
	isReactComponentOrNamedHookBoundary,
}

func hiddenMutationEvidence(file string, fn precisionFunction) (mutationEvidence, bool) {
	if anyPrecisionExemption(file, fn, hiddenMutationExemptions) {
		return mutationEvidence{}, false
	}
	evidence, hasEvidence := firstReportableMutationEvidence(fn)
	if !hasEvidence || isBenignMutationContext(file, fn, evidence) {
		return mutationEvidence{}, false
	}
	return evidence, true
}

// isBenignMutationContext reports whether a mutation is expected given the
// function's role: UI helpers and builders that only touch local state, and
// pure helpers, React local-state boundaries, or predicates that do not
// mutate their arguments.
func isBenignMutationContext(file string, fn precisionFunction, evidence mutationEvidence) bool {
	if isUIHelperOrMappingContext(file, fn) && !hasPersistentCollaboratorSideEffect(fn) {
		return true
	}
	if isAccumulatorBuilderFunctionName(fn.Name) && !hasLikelyExternalMutationCall(fn) && !hasLikelyParameterAssignment(fn) {
		return true
	}
	if evidence.Target == targetArgument {
		return false
	}
	return (isPureComputationHelperName(fn.Name) && !hasPersistentCollaboratorSideEffect(fn)) ||
		(isReactLocalStateBoundary(file, fn) && onlyReactHookLocalStateMutation(fn)) ||
		(isPredicateName(fn.Name) && !predicateHasObviousSideEffect(fn))
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
