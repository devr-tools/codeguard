package quality

import (
	"regexp"
	"strings"
)

var paramMutationPattern = regexp.MustCompile(`\b([A-Za-z_$][\w$]*)\s*(?:\.|->|\[)`)

func isFactoryHelperName(name string) bool {
	lowered := strings.ToLower(strings.Trim(name, "_$"))
	return strings.HasPrefix(lowered, "make") ||
		strings.HasPrefix(lowered, "factory") ||
		strings.HasPrefix(lowered, "create") && strings.Contains(lowered, "factory")
}

func isPureComputationHelperName(name string) bool {
	lowered := strings.ToLower(strings.Trim(name, "_$"))
	for _, prefix := range []string{
		"as", "arrivals", "build", "calculate", "classify", "collect", "compress", "compute", "derive",
		"deep", "esc", "extract", "find", "focus", "format", "formvalues", "movement", "normalize",
		"overdue", "parse", "pick", "quick", "rank", "rme", "score", "stats", "to",
	} {
		if strings.HasPrefix(lowered, prefix) {
			return true
		}
	}
	return false
}

func hasIOOrPersistenceSideEffect(fn precisionFunction) bool {
	body := strings.ToLower(fn.Body)
	return containsAny(body, []string{
		"await ", "fetch(", "axios.", "prisma.", "db.", "client.", "repo.", "repository.",
		"localstorage.", "sessionstorage.", "document.cookie",
		".create(", ".delete(", ".insert(", ".remove(", ".save(", ".send(", ".update(", ".upsert(",
		"writefile", "writefilesync", "fs.write", "console.error(",
	})
}

func hasPersistentCollaboratorSideEffect(fn precisionFunction) bool {
	body := strings.ToLower(fn.Body)
	return containsAny(body, []string{
		"await ", "fetch(", "axios.", "prisma.", "db.", "repo.", "repository.",
		"localstorage.", "sessionstorage.", "document.cookie",
		".save(", ".send(", ".upsert(", ".delete(",
	})
}

func hasLikelyExternalMutationCall(fn precisionFunction) bool {
	localTargets := localMutationTargets(fn)
	params := paramNames(fn)
	for _, call := range directCalls(fn) {
		if !mutatingCallPattern.MatchString(call.Callee) {
			continue
		}
		if isLocalMutationCall(call, localTargets) || isLocalBuilderMutationCall(fn, call) || isBuilderAccumulatorMutationCall(fn, call) {
			continue
		}
		target := mutationCallTarget(call.Callee)
		if target == "" {
			continue
		}
		if _, isParam := params[target]; isParam {
			return true
		}
		if isAccumulatorLikeLocalName(target) {
			continue
		}
		return true
	}
	return false
}

func hasLikelyParameterAssignment(fn precisionFunction) bool {
	params := paramNames(fn)
	if len(params) == 0 {
		return false
	}
	for _, statement := range directStatements(fn) {
		line := firstNonEmptyString(statement.Raw, statement.Text)
		if !lineHasAssignmentOperator(line) {
			continue
		}
		for _, match := range paramMutationPattern.FindAllStringSubmatch(assignmentLeftHandSide(line), -1) {
			if _, ok := params[match[1]]; ok {
				return true
			}
		}
	}
	return false
}

func mutatingFunctionEvidence(fn precisionFunction) bool {
	localTargets := localMutationTargets(fn)
	for _, call := range directCalls(fn) {
		if mutatingCallPattern.MatchString(call.Callee) && !isLocalMutationCall(call, localTargets) && !isLocalBuilderMutationCall(fn, call) && !isBuilderAccumulatorMutationCall(fn, call) {
			return true
		}
	}
	for _, assignment := range directAssignments(fn) {
		if assignment.Augmented &&
			!isLocalMutationTarget(assignment.Name, localTargets) &&
			!isBuilderAccumulatorAssignment(fn, assignment) &&
			!isLocalScalarAccumulatorAssignment(fn, assignment.Name) {
			return true
		}
	}
	return false
}

func assignmentLeftHandSide(line string) string {
	for idx := 0; idx < len(line); idx++ {
		if line[idx] != '=' {
			continue
		}
		prev := byte(0)
		next := byte(0)
		if idx > 0 {
			prev = line[idx-1]
		}
		if idx+1 < len(line) {
			next = line[idx+1]
		}
		if prev == '=' || prev == '!' || prev == '<' || prev == '>' || next == '=' || next == '>' {
			continue
		}
		return trimControlFlowConditionFromAssignmentLHS(line[:idx])
	}
	return line
}

func trimControlFlowConditionFromAssignmentLHS(lhs string) string {
	trimmed := strings.TrimSpace(lhs)
	lowered := strings.ToLower(trimmed)
	if !strings.HasPrefix(lowered, "if ") && !strings.HasPrefix(lowered, "else if ") && !strings.HasPrefix(lowered, "while ") {
		return lhs
	}
	if idx := strings.LastIndex(trimmed, ")"); idx >= 0 && idx+1 < len(trimmed) {
		return trimmed[idx+1:]
	}
	return lhs
}

func lineHasAssignmentOperator(line string) bool {
	for idx := 0; idx < len(line); idx++ {
		if line[idx] != '=' {
			continue
		}
		prev := byte(0)
		next := byte(0)
		if idx > 0 {
			prev = line[idx-1]
		}
		if idx+1 < len(line) {
			next = line[idx+1]
		}
		if prev == '=' || prev == '!' || prev == '<' || prev == '>' || next == '=' || next == '>' {
			continue
		}
		return true
	}
	return false
}
