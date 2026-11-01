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

type mutationBinding struct {
	origin string
	target string
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
	return functionMutationAnalysis(fn, "").Mutations
}

func functionMutationAnalysis(fn precisionFunction, language string) mutationAnalysis {
	if fn.GoDecl != nil {
		return goFunctionMutationEvidence(fn)
	}
	if fn.Language == string(support.CLikeCPP) || language == "c++" || language == "cpp" {
		return cppFunctionMutationEvidence(fn)
	}
	return scriptFunctionMutationAnalysis(fn, language)
}

// scriptMutationAnalyzer resolves mutation evidence for parsed (non-Go,
// non-C++) functions from their binding origins and textual statements.
type scriptMutationAnalyzer struct {
	fn             precisionFunction
	language       string
	origins        map[string]string
	targets        map[string]string
	analysis       mutationAnalysis
	seen           map[string]struct{}
	unresolvedSeen map[string]struct{}
	// escapedAt records the line where a locally allocated binding was stored
	// into non-local state; later mutations through it are shared.
	escapedAt map[string]int
	// escapedNames records locally allocated bindings assigned anywhere in the
	// body to a non-local value, so every field write through them is shared.
	escapedNames map[string]bool
}

func scriptFunctionMutationAnalysis(fn precisionFunction, language string) mutationAnalysis {
	origins, targets := resolvedMutationBindings(fn)
	analyzer := &scriptMutationAnalyzer{
		fn:             fn,
		language:       language,
		origins:        origins,
		targets:        targets,
		seen:           map[string]struct{}{},
		unresolvedSeen: map[string]struct{}{},
		escapedAt:      localEscapeLines(fn, origins),
		escapedNames:   localsStoredInSharedState(fn, origins),
	}
	analyzer.scanFieldMutations()
	analyzer.scanAssignmentMutations()
	analyzer.scanCallMutations()
	return analyzer.analysis
}

func (a *scriptMutationAnalyzer) add(item mutationEvidence) {
	key := item.Target + "|" + item.Effect + "|" + item.Origin + "|" + item.Detail
	if _, ok := a.seen[key]; ok {
		return
	}
	a.seen[key] = struct{}{}
	a.analysis.Mutations = append(a.analysis.Mutations, item)
}

func (a *scriptMutationAnalyzer) addUnresolved(line int, operation string, symbol string) {
	key := a.language + "|" + operation + "|" + symbol + "|" + strconv.Itoa(line)
	if _, ok := a.unresolvedSeen[key]; ok {
		return
	}
	a.unresolvedSeen[key] = struct{}{}
	a.analysis.Unresolved = append(a.analysis.Unresolved, unresolvedMutationEvidence{
		Language:  a.language,
		Line:      line,
		Operation: operation,
		Symbol:    symbol,
		Reason:    "symbol ownership could not be resolved",
	})
}

// escapedLocal reports whether a locally allocated binding had already
// escaped into non-local state before line.
func (a *scriptMutationAnalyzer) escapedLocal(name string, line int) bool {
	at := a.escapedAt[name]
	return at > 0 && line > at
}

// scanFieldMutations records field and index writes found anywhere in the
// body on caller-owned, shared, or escaped bindings.
func (a *scriptMutationAnalyzer) scanFieldMutations() {
	for _, match := range bodyFieldMutationPattern.FindAllStringSubmatch(a.fn.Body, -1) {
		name := match[1]
		target, origin := a.targets[name], a.origins[name]
		if origin == originUnknown {
			continue
		}
		if origin == originLocal && a.escapedNames[name] {
			target, origin = targetEscaped, originShared
		} else if origin == originLocal || target == "" {
			continue
		}
		a.add(mutationEvidence{Target: target, Effect: "shared_state", Origin: origin, Line: a.fn.StartLine, Detail: name})
	}
}

// scanAssignmentMutations records assignments and increments whose left-hand
// side writes through a non-local binding.
func (a *scriptMutationAnalyzer) scanAssignmentMutations() {
	for _, statement := range directStatements(a.fn) {
		line := firstNonEmptyString(statement.Raw, statement.Text)
		if !lineHasAssignmentOperator(line) && !strings.Contains(line, "++") && !strings.Contains(line, "--") {
			continue
		}
		for _, match := range mutationRootPattern.FindAllStringSubmatch(assignmentLeftHandSide(line), -1) {
			a.recordAssignment(statement.Line, match[1])
		}
	}
}

func (a *scriptMutationAnalyzer) recordAssignment(line int, name string) {
	target, origin := a.targets[name], a.origins[name]
	if origin == originLocal {
		if !a.escapedLocal(name, line) {
			return
		}
		target, origin = targetEscaped, originShared
	}
	if origin == originUnknown || target == "" {
		a.addUnresolved(line, "assignment", name)
		return
	}
	a.add(mutationEvidence{Target: target, Effect: "shared_state", Origin: origin, Line: line, Detail: name})
}

// scanCallMutations records observable or mutating calls on non-local
// bindings, and calls whose receiver ownership cannot be resolved.
func (a *scriptMutationAnalyzer) scanCallMutations() {
	for _, call := range directCalls(a.fn) {
		targetName := mutationCallTarget(call.Callee)
		if isObjectAssignCall(call) {
			targetName = firstCallArgName(call)
		}
		target, origin := a.targets[targetName], a.origins[targetName]
		if origin == originUnknown {
			target = ""
		}
		if origin == originLocal {
			if !a.escapedLocal(targetName, call.Line) {
				continue
			}
			target, origin = targetEscaped, originShared
		}
		symbol := targetName
		if symbol == "" {
			symbol = call.Callee
		}
		effect := observableCallEffect(call.Callee)
		if effect == "" {
			if !mutatingCallPattern.MatchString(call.Callee) {
				continue
			}
			effect = "shared_state"
		}
		if target == "" {
			a.addUnresolved(call.Line, "call", symbol)
			continue
		}
		a.add(mutationEvidence{Target: target, Effect: effect, Origin: origin, Line: call.Line, Detail: call.Callee})
	}
}

// localEscapeLines finds, for each locally allocated binding, the last
// statement that stores it into a field, index, or undeclared name.
func localEscapeLines(fn precisionFunction, origins map[string]string) map[string]int {
	escapedAt := map[string]int{}
	for _, statement := range directStatements(fn) {
		line := firstNonEmptyString(statement.Raw, statement.Text)
		if !lineHasAssignmentOperator(line) {
			continue
		}
		lhs := assignmentLeftHandSide(line)
		lhsName := strings.TrimSpace(lhs)
		if !mutationRootPattern.MatchString(lhs) && (!aliasExprPattern.MatchString(lhsName) || origins[lhsName] != "") {
			continue
		}
		for name, origin := range origins {
			if origin == originLocal && strings.Contains(line, name) && !strings.Contains(lhs, name) {
				escapedAt[name] = statement.Line
			}
		}
	}
	return escapedAt
}

// localsStoredInSharedState finds locally allocated bindings assigned to a
// name or field whose root is not itself locally allocated.
func localsStoredInSharedState(fn precisionFunction, origins map[string]string) map[string]bool {
	escapedNames := map[string]bool{}
	for name, origin := range origins {
		if origin != originLocal {
			continue
		}
		storePattern := cachedPattern(`\b([A-Za-z_$][\w$]*)(?:\.[A-Za-z_$][\w$]*)?\s*=\s*` + regexp.QuoteMeta(name) + `\b`)
		for _, match := range storePattern.FindAllStringSubmatch(fn.Body, -1) {
			if origins[match[1]] != originLocal {
				escapedNames[name] = true
				break
			}
		}
	}
	return escapedNames
}

func resolvedMutationBindings(fn precisionFunction) (map[string]string, map[string]string) {
	origins, targets := declaredMutationBindings(fn)
	for _, assignment := range directAssignments(fn) {
		applyAssignmentBinding(fn, assignment, origins, targets)
	}
	for _, pattern := range []*regexp.Regexp{goAliasPattern, cppAliasPattern} {
		for _, match := range pattern.FindAllStringSubmatch(fn.Body, -1) {
			if origin := origins[match[2]]; origin != "" {
				origins[match[1]], targets[match[1]] = origin, targets[match[2]]
			}
		}
	}
	return origins, targets
}

// declaredMutationBindings seeds ownership from what the function declares:
// captured bindings, proven globals, parameters, the receiver, and locals.
func declaredMutationBindings(fn precisionFunction) (map[string]string, map[string]string) {
	origins := make(map[string]string, len(fn.CapturedBindings))
	targets := make(map[string]string, len(fn.CapturedBindings))
	for name, binding := range fn.CapturedBindings {
		origins[name], targets[name] = binding.origin, binding.target
	}
	for name := range fn.ProvenGlobals {
		origins[name], targets[name] = originShared, targetGlobal
	}
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
	for name := range localMutationTargets(fn) {
		origins[name], targets[name] = originLocal, targetLocal
	}
	return origins, targets
}

// applyAssignmentBinding updates ownership for one assignment: a local
// declaration shadows outer bindings, an alias inherits its source's
// ownership, local allocations are local, and other call results are
// unknown.
func applyAssignmentBinding(fn precisionFunction, assignment support.ParsedAssignment, origins map[string]string, targets map[string]string) {
	name := strings.TrimSpace(assignment.Name)
	if name == "" {
		return
	}
	if assignmentDeclaresLocal(fn, assignment) {
		delete(origins, name)
		delete(targets, name)
	}
	if source := assignmentAliasSource(fn, assignment); source != "" && assignmentCanReceiveAlias(fn, assignment) {
		if origin := origins[source]; origin != "" {
			origins[name], targets[name] = origin, targets[source]
		}
	}
	if assignmentLooksLocalAccumulator(fn, assignment) || assignmentLooksLocalBuilder(fn, assignment) || looksLikeLocalObjectAllocation(fn, assignment) {
		origins[name], targets[name] = originLocal, targetLocal
	} else if origins[name] == "" && strings.Contains(assignment.Expr, "(") {
		origins[name], targets[name] = originUnknown, ""
	}
}

func assignmentAliasSource(fn precisionFunction, assignment support.ParsedAssignment) string {
	if match := aliasExprPattern.FindStringSubmatch(strings.TrimSpace(assignment.Expr)); len(match) == 2 {
		return match[1]
	}
	name := regexp.QuoteMeta(strings.TrimSpace(assignment.Name))
	match := cachedPattern(`\b` + name + `\s*(?::?=)\s*&?([A-Za-z_$][\w$]*)\b`).FindStringSubmatch(assignmentStatement(fn, assignment.Line))
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
	return cachedPattern(`\b`+name+`\s*:=`).MatchString(statement) ||
		cachedPattern(`(?i)\b(?:const|let|var|auto)\s+`+name+`\b`).MatchString(statement) ||
		cachedPattern(`\b[A-Za-z_$][\w$:<>, ]*\s*&\s*`+name+`\b`).MatchString(statement)
}

func assignmentCanReceiveAlias(fn precisionFunction, assignment support.ParsedAssignment) bool {
	if assignmentDeclaresLocal(fn, assignment) {
		return true
	}
	name := regexp.QuoteMeta(strings.TrimSpace(assignment.Name))
	return cachedPattern(`(?m)\b(?:var|let|const|auto)\s+` + name + `\b`).MatchString(fn.Body)
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
	return cachedPattern(`\b`+name+`\s*:=\s*(?:&|make\s*\()`).MatchString(lowerStatement) ||
		cachedPattern(`\b`+name+`\s*=\s*new\b`).MatchString(lowerStatement) ||
		cachedPattern(`\b[A-Za-z_$][\w$:<>, ]+\s+`+name+`\s*\{`).MatchString(lowerStatement) ||
		cachedPattern(`\b`+name+`\s*:=\s*(?:new|create|build)[a-z0-9_$]*\s*\(`).MatchString(lowerStatement)
}

func observableCallEffect(callee string) string {
	identifiers := identifierTokenPattern.FindAllString(callee, -1)
	if len(identifiers) == 0 {
		return ""
	}
	words := identifierWords(identifiers[len(identifiers)-1])
	if containsWord(words, "publish", "emit", "dispatch", "enqueue") {
		return "event"
	}
	// Read grammar belongs to the invoked method, not its receiver. A receiver such
	// as fetchQueue must not turn Enqueue into a read, while RecordPrefetch must not
	// become an event or persistence effect merely because of an embedded token.
	if readCallPattern.MatchString(identifiers[len(identifiers)-1]) {
		return ""
	}
	receiverWords := make([]string, 0, len(identifiers)-1)
	for _, identifier := range identifiers[:len(identifiers)-1] {
		receiverWords = append(receiverWords, identifierWords(identifier)...)
	}
	if containsWord(words, "fetch", "axios", "send", "upload") {
		return "network"
	}
	if containsWord(receiverWords, "http") && containsWord(words, "post", "put", "patch") {
		return "network"
	}
	if containsWord(words, "save", "insert", "update", "upsert", "delete", "exec", "commit", "rollback", "write", "persist") {
		return "persistence"
	}
	if containsWord(receiverWords, "cache") && containsWord(words, "set", "put") {
		return "persistence"
	}
	return ""
}

func terminalCallIdentifier(callee string) string {
	identifiers := identifierTokenPattern.FindAllString(callee, -1)
	if len(identifiers) == 0 {
		return ""
	}
	return identifiers[len(identifiers)-1]
}

func containsWord(words []string, candidates ...string) bool {
	for _, word := range words {
		for _, candidate := range candidates {
			if word == candidate {
				return true
			}
		}
	}
	return false
}

func firstReportableMutationEvidence(fn precisionFunction) (mutationEvidence, bool) {
	var first mutationEvidence
	found := false
	for _, evidence := range functionMutationEvidence(fn) {
		if evidence.Target == targetLocal {
			continue
		}
		if !found {
			first = evidence
			found = true
		}
		if evidence.Target == targetEscaped {
			return evidence, true
		}
	}
	return first, found
}
