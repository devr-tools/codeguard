package quality

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

type mutationEvidence struct {
	Target string
	Effect string
	Origin string
	Line   int
	Detail string
}

type unresolvedMutationEvidence struct {
	Language  string
	Line      int
	Operation string
	Symbol    string
	Reason    string
}

type mutationAnalysis struct {
	Mutations  []mutationEvidence
	Unresolved []unresolvedMutationEvidence
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
	return functionMutationAnalysis(fn, "").Mutations
}

func functionMutationAnalysis(fn precisionFunction, language string) mutationAnalysis {
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
		if source := assignmentAliasSource(fn, assignment); source != "" && assignmentCanReceiveAlias(fn, assignment) {
			if origin := origins[source]; origin != "" {
				origins[name], targets[name] = origin, targets[source]
			}
		}
		if assignmentLooksLocalAccumulator(fn, assignment) || assignmentLooksLocalBuilder(fn, assignment) || looksLikeLocalObjectAllocation(fn, assignment) {
			origins[name], targets[name] = originLocal, targetLocal
		} else if origins[name] == "" && strings.Contains(assignment.Expr, "(") {
			origins[name], targets[name] = originUnknown, targetEscaped
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
	var analysis mutationAnalysis
	seen := map[string]struct{}{}
	add := func(item mutationEvidence) {
		key := item.Target + "|" + item.Effect + "|" + item.Origin + "|" + item.Detail
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		analysis.Mutations = append(analysis.Mutations, item)
	}
	unresolvedSeen := map[string]struct{}{}
	addUnresolved := func(line int, operation string, symbol string) {
		key := language + "|" + operation + "|" + symbol + "|" + strconv.Itoa(line)
		if _, ok := unresolvedSeen[key]; ok {
			return
		}
		unresolvedSeen[key] = struct{}{}
		analysis.Unresolved = append(analysis.Unresolved, unresolvedMutationEvidence{
			Language:  language,
			Line:      line,
			Operation: operation,
			Symbol:    symbol,
			Reason:    "symbol ownership could not be resolved",
		})
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
				addUnresolved(statement.Line, "assignment", name)
				continue
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
		unresolvedSymbol := targetName
		if unresolvedSymbol == "" {
			unresolvedSymbol = call.Callee
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
			if target == "" {
				if mutatingCallPattern.MatchString(call.Callee) && !isConstructionOrHydrationCall(call.Callee) {
					addUnresolved(call.Line, "call", unresolvedSymbol)
				}
				continue
			}
			if !mutatingCallPattern.MatchString(call.Callee) || isConstructionOrHydrationCall(call.Callee) {
				continue
			}
			effect = "shared_state"
		}
		if target == "" {
			addUnresolved(call.Line, "call", unresolvedSymbol)
			continue
		}
		add(mutationEvidence{Target: target, Effect: effect, Origin: origin, Line: call.Line, Detail: call.Callee})
	}
	return analysis
}

func assignmentAliasSource(fn precisionFunction, assignment support.ParsedAssignment) string {
	if match := aliasExprPattern.FindStringSubmatch(strings.TrimSpace(assignment.Expr)); len(match) == 2 {
		return match[1]
	}
	name := regexp.QuoteMeta(strings.TrimSpace(assignment.Name))
	match := regexp.MustCompile(`\b` + name + `\s*(?::?=)\s*&?([A-Za-z_$][\w$]*)\b`).FindStringSubmatch(assignmentStatement(fn, assignment.Line))
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

func assignmentDeclaresLocal(fn precisionFunction, assignment support.ParsedAssignment) bool {
	statement := assignmentStatement(fn, assignment.Line)
	name := regexp.QuoteMeta(strings.TrimSpace(assignment.Name))
	if name == "" {
		return false
	}
	return regexp.MustCompile(`\b`+name+`\s*:=`).MatchString(statement) ||
		regexp.MustCompile(`(?i)\b(?:const|let|var|auto)\s+`+name+`\b`).MatchString(statement) ||
		regexp.MustCompile(`\b[A-Za-z_$][\w$:<>, ]*\s*&\s*`+name+`\b`).MatchString(statement)
}

func assignmentCanReceiveAlias(fn precisionFunction, assignment support.ParsedAssignment) bool {
	if assignmentDeclaresLocal(fn, assignment) {
		return true
	}
	name := regexp.QuoteMeta(strings.TrimSpace(assignment.Name))
	return regexp.MustCompile(`(?m)\b(?:var|let|const|auto)\s+` + name + `\b`).MatchString(fn.Body)
}

func looksLikeLocalObjectAllocation(fn precisionFunction, assignment support.ParsedAssignment) bool {
	lowerExpr := strings.ToLower(strings.TrimSpace(assignment.Expr))
	if lowerExpr != "" {
		return strings.HasPrefix(lowerExpr, "&") || strings.HasPrefix(lowerExpr, "new ") ||
			strings.HasPrefix(lowerExpr, "make(") || strings.Contains(lowerExpr, "{}") ||
			strings.HasPrefix(lowerExpr, "std::") || containsAny(lowerExpr, []string{"builder", "dto", "response", "payload"})
	}
	lowerStatement := strings.ToLower(assignmentStatement(fn, assignment.Line))
	name := regexp.QuoteMeta(strings.ToLower(strings.TrimSpace(assignment.Name)))
	return regexp.MustCompile(`\b`+name+`\s*:=\s*(?:&|make\s*\()`).MatchString(lowerStatement) ||
		regexp.MustCompile(`\b`+name+`\s*=\s*new\b`).MatchString(lowerStatement) ||
		regexp.MustCompile(`\b[A-Za-z_$][\w$:<>, ]+\s+`+name+`\s*\{`).MatchString(lowerStatement) ||
		regexp.MustCompile(`\b`+name+`\s*:=\s*(?:new|create|build)[a-z0-9_$]*\s*\(`).MatchString(lowerStatement)
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
