package quality

import (
	"regexp"
	"strings"
)

type mutationEvidence struct {
	Target string
	Effect string
	Origin string
	Line   int
	Detail string
}

func mutationEvidenceMetadata(evidence mutationEvidence) map[string]string {
	return map[string]string{
		"mutation_target": evidence.Target,
		"effect_kind":     evidence.Effect,
		"origin":          evidence.Origin,
	}
}

const (
	originLocal    = "locally_allocated"
	originCaller   = "caller_owned"
	originShared   = "shared"
	originUnknown  = "unknown"
	targetLocal    = "local"
	targetArgument = "argument"
	targetReceiver = "receiver"
	targetGlobal   = "global"
	targetEscaped  = "escaped"
)

var (
	mutationRootPattern      = regexp.MustCompile(`\b([A-Za-z_$][\w$]*)\s*(?:\.|->|\[)`)
	aliasExprPattern         = regexp.MustCompile(`^\s*&?([A-Za-z_$][\w$]*)\s*$`)
	goAliasPattern           = regexp.MustCompile(`\b([A-Za-z_$][\w$]*)\s*:=\s*&?([A-Za-z_$][\w$]*)\b`)
	cppAliasPattern          = regexp.MustCompile(`\b[A-Za-z_$][\w$:<>, ]*\s*&\s*([A-Za-z_$][\w$]*)\s*=\s*([A-Za-z_$][\w$]*)\b`)
	bodyFieldMutationPattern = regexp.MustCompile(`\b([A-Za-z_$][\w$]*)\s*(?:(?:\.|->)\s*[A-Za-z_$][\w$]*|\[[^]]*\])\s*(?:=\s|\+\+|--|\+=|-=|\*=|/=)`)
)

func functionMutationEvidence(fn precisionFunction) []mutationEvidence {
	origins := map[string]string{}
	targets := map[string]string{}
	for _, param := range fn.Params {
		if param.Name != "" {
			origins[param.Name], targets[param.Name] = originCaller, targetArgument
		}
	}
	if fn.ReceiverName != "" {
		origins[fn.ReceiverName], targets[fn.ReceiverName] = originCaller, targetReceiver
	}
	if fn.Receiver != "" {
		origins["this"], targets["this"] = originCaller, targetReceiver
	}

	locals := localMutationTargets(fn)
	for name := range locals {
		origins[name], targets[name] = originLocal, targetLocal
	}
	for _, assignment := range directAssignments(fn) {
		name := strings.TrimSpace(assignment.Name)
		if name == "" {
			continue
		}
		if origins[name] == "" {
			origins[name], targets[name] = originLocal, targetLocal
		}
		if match := aliasExprPattern.FindStringSubmatch(strings.TrimSpace(assignment.Expr)); len(match) == 2 {
			if origin := origins[match[1]]; origin != "" {
				origins[name], targets[name] = origin, targets[match[1]]
			}
		}
		if assignmentLooksLocalAccumulator(fn, assignment) || assignmentLooksLocalBuilder(fn, assignment) || looksLikeLocalObjectAllocation(assignmentStatement(fn, assignment.Line), assignment.Expr) {
			origins[name], targets[name] = originLocal, targetLocal
		}
	}
	for _, pattern := range []*regexp.Regexp{goAliasPattern, cppAliasPattern} {
		for _, match := range pattern.FindAllStringSubmatch(fn.Body, -1) {
			if origin := origins[match[2]]; origin != "" {
				origins[match[1]], targets[match[1]] = origin, targets[match[2]]
			}
		}
	}

	escapedAt := map[string]int{}
	escapedNames := map[string]bool{}
	for _, statement := range directStatements(fn) {
		line := firstNonEmptyString(statement.Raw, statement.Text)
		if !lineHasAssignmentOperator(line) {
			continue
		}
		lhs := assignmentLeftHandSide(line)
		for name, origin := range origins {
			if origin != originLocal || !strings.Contains(line, name) || strings.Contains(lhs, name) {
				continue
			}
			lhsName := strings.TrimSpace(lhs)
			if mutationRootPattern.MatchString(lhs) || (aliasExprPattern.MatchString(lhsName) && origins[lhsName] == "") {
				escapedAt[name] = statement.Line
			}
		}
	}
	for name, origin := range origins {
		if origin != originLocal {
			continue
		}
		storePattern := regexp.MustCompile(`\b([A-Za-z_$][\w$]*)(?:\.[A-Za-z_$][\w$]*)?\s*=\s*` + regexp.QuoteMeta(name) + `\b`)
		for _, match := range storePattern.FindAllStringSubmatch(fn.Body, -1) {
			if origins[match[1]] != originLocal {
				escapedNames[name] = true
				break
			}
		}
	}
	var evidence []mutationEvidence
	seen := map[string]struct{}{}
	add := func(item mutationEvidence) {
		key := item.Target + "|" + item.Effect + "|" + item.Origin + "|" + item.Detail
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		evidence = append(evidence, item)
	}
	for _, match := range bodyFieldMutationPattern.FindAllStringSubmatch(fn.Body, -1) {
		name := match[1]
		target, origin := targets[name], origins[name]
		if origin == originLocal && escapedNames[name] {
			target, origin = targetEscaped, originShared
		} else if origin == originLocal || target == "" {
			continue
		}
		add(mutationEvidence{Target: target, Effect: "shared_state", Origin: origin, Line: fn.StartLine, Detail: name})
	}
	for _, statement := range directStatements(fn) {
		line := firstNonEmptyString(statement.Raw, statement.Text)
		if !lineHasAssignmentOperator(line) && !strings.Contains(line, "++") && !strings.Contains(line, "--") {
			continue
		}
		lhs := assignmentLeftHandSide(line)
		for _, match := range mutationRootPattern.FindAllStringSubmatch(lhs, -1) {
			name := match[1]
			target, origin := targets[name], origins[name]
			if origin == originLocal {
				if escapedAt[name] > 0 && statement.Line > escapedAt[name] {
					target, origin = targetEscaped, originShared
				} else {
					continue
				}
			}
			if target == "" {
				target, origin = targetGlobal, originShared
			}
			add(mutationEvidence{Target: target, Effect: "shared_state", Origin: origin, Line: statement.Line, Detail: name})
		}
	}
	for _, call := range directCalls(fn) {
		effect := observableCallEffect(call.Callee)
		targetName := mutationCallTarget(call.Callee)
		if isObjectAssignCall(call) {
			targetName = firstCallArgName(call)
		}
		target, origin := targets[targetName], origins[targetName]
		if origin == originLocal {
			if escapedAt[targetName] > 0 && call.Line > escapedAt[targetName] {
				target, origin = targetEscaped, originShared
			} else {
				continue
			}
		}
		if effect == "" {
			if target == "" || !mutatingCallPattern.MatchString(call.Callee) || isConstructionOrHydrationCall(call.Callee) {
				continue
			}
			effect = "shared_state"
		}
		if target == "" {
			target, origin = targetGlobal, originShared
		}
		add(mutationEvidence{Target: target, Effect: effect, Origin: origin, Line: call.Line, Detail: call.Callee})
	}
	return evidence
}

func looksLikeLocalObjectAllocation(statement, expr string) bool {
	lower := strings.ToLower(statement + " " + expr)
	return containsAny(lower, []string{":= &", ":= make(", "= new ", "{}", "std::", "builder", "dto", "response", "payload"})
}

func observableCallEffect(callee string) string {
	lower := strings.ToLower(callee)
	if isConstructionOrHydrationCall(callee) || readCallPattern.MatchString(callee) {
		return ""
	}
	if containsAny(lower, []string{"publish", "emit", "dispatch", "enqueue"}) {
		return "event"
	}
	if containsAny(lower, []string{"http.post", "http.put", "http.patch", "fetch", "axios", ".send", ".upload"}) {
		return "network"
	}
	if containsAny(lower, []string{".save", ".insert", ".update", ".upsert", ".delete", ".exec", ".commit", ".rollback", ".write", ".persist", "cache.set", "cache.put"}) {
		return "persistence"
	}
	return ""
}

func isConstructionOrHydrationCall(callee string) bool {
	lower := strings.ToLower(callee)
	return containsAny(lower, []string{
		".scan", ".setname", ".setid", ".setvalue", ".setfield", "proto.", "protobuf",
		"json.marshal", "json.stringify", "serialize", "metrics.", "metric.", "observe", "recordlatency",
		"builder.", ".with", ".addfield", ".appendfield",
	})
}

func firstReportableMutationEvidence(fn precisionFunction) (mutationEvidence, bool) {
	for _, evidence := range functionMutationEvidence(fn) {
		if evidence.Target != targetLocal {
			return evidence, true
		}
	}
	return mutationEvidence{}, false
}
