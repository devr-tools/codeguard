package quality

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

type cppResolvedOwnership struct {
	origin string
	target string
	name   string
	shape  string
}

var (
	cppFieldMutationPattern  = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*(?:(?:\.|->)\s*[A-Za-z_]\w*|\[[^]]*\])\s*(?:=(?:[^=>]|\z)|\+\+|--|\+=|-=|\*=|/=)`)
	cppBareMutationPattern   = regexp.MustCompile(`(?:^|[;{}])[ \t]*(?:\*[ \t]*)?([A-Za-z_]\w*)[ \t]*(?:=(?:[^=>]|\z)|\+\+|--|\+=|-=|\*=|/=)`)
	cppPrefixMutationPattern = regexp.MustCompile(`(?:^|[;{}])[ \t]*(?:\+\+|--)[ \t]*(?:\*[ \t]*)?([A-Za-z_]\w*)\b`)
	cppEscapeStorePattern    = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*(?:(?:\.|->)\s*[A-Za-z_]\w*)?[ \t]*=[ \t]*&?([A-Za-z_]\w*)\b`)
	cppAutoCallResultPattern = regexp.MustCompile(`^\s*[A-Za-z_]\w*(?:::[A-Za-z_]\w*)*(?:\s*<[^(){};]+>)?\s*\(`)
)

// cppFunctionMutationEvidence resolves mutation ownership from bounded
// declaration metadata. Unknown roots remain diagnostic-only evidence.
func cppFunctionMutationEvidence(fn precisionFunction) mutationAnalysis {
	resolver := cppOwnershipResolver{fn: fn}
	collector := &cppMutationCollector{
		resolver:       resolver,
		escapedAt:      cppEscapedLocals(resolver),
		seen:           map[string]struct{}{},
		unresolvedSeen: map[string]struct{}{},
	}
	for _, statement := range directStatements(fn) {
		collector.recordAssignments(firstNonEmptyString(statement.Raw, statement.Text), statement.Line)
	}
	for _, call := range directCalls(fn) {
		collector.recordCall(call)
	}
	return collector.analysis
}

// cppMutationCollector accumulates de-duplicated C++ mutation evidence.
// escapedAt holds, per locally allocated name, the first line where it was
// stored into non-local state; mutations through it from then on are shared.
type cppMutationCollector struct {
	resolver       cppOwnershipResolver
	escapedAt      map[string]int
	analysis       mutationAnalysis
	seen           map[string]struct{}
	unresolvedSeen map[string]struct{}
}

func (c *cppMutationCollector) addMutation(item mutationEvidence) {
	key := item.Target + "|" + item.Effect + "|" + item.Origin + "|" + item.Detail + "|" + strconv.Itoa(item.Line)
	if _, ok := c.seen[key]; ok {
		return
	}
	c.seen[key] = struct{}{}
	c.analysis.Mutations = append(c.analysis.Mutations, item)
}

func (c *cppMutationCollector) addUnresolved(line int, operation string, symbol string) {
	key := operation + "|" + symbol + "|" + strconv.Itoa(line)
	if _, ok := c.unresolvedSeen[key]; ok {
		return
	}
	c.unresolvedSeen[key] = struct{}{}
	c.analysis.Unresolved = append(c.analysis.Unresolved, unresolvedMutationEvidence{
		Language: "c++", Line: line, Operation: operation, Symbol: symbol,
		Reason: "symbol ownership could not be resolved",
	})
}

// resolveMutation records a mutation through name, or an unresolved entry
// when its ownership is unknown. Local names only count once they escaped.
func (c *cppMutationCollector) resolveMutation(name string, line int, operation string, detail string, effect string) {
	ownership, ok := c.resolver.resolve(name, line, nil, 0)
	if !ok || ownership.origin == originUnknown || ownership.target == "" {
		c.addUnresolved(line, operation, name)
		return
	}
	if ownership.origin == originLocal {
		if c.escapedAt[ownership.name] == 0 || line < c.escapedAt[ownership.name] {
			return
		}
		ownership.origin, ownership.target = originShared, targetEscaped
	}
	c.addMutation(mutationEvidence{Target: ownership.target, Effect: effect, Origin: ownership.origin, Line: line, Detail: detail})
}

// recordAssignments records field/index writes, bare assignments, and
// prefix increments in one statement.
func (c *cppMutationCollector) recordAssignments(text string, line int) {
	for _, match := range cppFieldMutationPattern.FindAllStringSubmatch(text, -1) {
		c.resolveMutation(match[1], line, "assignment", match[1], "shared_state")
	}
	for _, match := range cppBareMutationPattern.FindAllStringSubmatch(text, -1) {
		c.recordRootAssignment(match[1], line)
	}
	for _, match := range cppPrefixMutationPattern.FindAllStringSubmatch(text, -1) {
		c.recordRootAssignment(match[1], line)
	}
}

// recordRootAssignment records a write to a whole variable, which only
// mutates caller-visible state when the variable is a receiver member, a
// global, or an argument.
func (c *cppMutationCollector) recordRootAssignment(name string, line int) {
	ownership, ok := c.resolver.resolve(name, line, nil, 0)
	if !ok || ownership.origin == originUnknown || ownership.target == "" {
		c.addUnresolved(line, "assignment", name)
		return
	}
	if ownership.target != targetReceiver && ownership.target != targetGlobal && ownership.target != targetArgument {
		return
	}
	c.resolveMutation(name, line, "assignment", name, "shared_state")
}

func (c *cppMutationCollector) recordCall(call support.ParsedCall) {
	effect := observableCallEffect(call.Callee)
	if effect == "" && !mutatingCallPattern.MatchString(call.Callee) {
		return
	}
	root := mutationCallTarget(call.Callee)
	if root == "" {
		c.addUnresolved(call.Line, "call", call.Callee)
		return
	}
	if effect == "" {
		effect = "shared_state"
	}
	c.resolveMutation(root, call.Line, "call", call.Callee, effect)
}

// cppEscapedLocals finds, per locally allocated name, the first line where it
// is stored into non-local state.
func cppEscapedLocals(r cppOwnershipResolver) map[string]int {
	escaped := map[string]int{}
	for _, statement := range directStatements(r.fn) {
		text := firstNonEmptyString(statement.Raw, statement.Text)
		for _, match := range cppEscapeStorePattern.FindAllStringSubmatch(text, -1) {
			left, leftOK := r.resolve(match[1], statement.Line, nil, 0)
			right, rightOK := r.resolve(match[2], statement.Line, nil, 0)
			if !leftOK || !rightOK || right.origin != originLocal || left.origin == originLocal {
				continue
			}
			if escaped[right.name] == 0 || statement.Line < escaped[right.name] {
				escaped[right.name] = statement.Line
			}
		}
	}
	return escaped
}

func cppDeclarationMoreSpecific(candidate support.ParsedDeclaration, current support.ParsedDeclaration) bool {
	candidateWidth := candidate.ScopeEnd - candidate.ScopeStart
	currentWidth := current.ScopeEnd - current.ScopeStart
	if candidateWidth != currentWidth {
		return candidateWidth < currentWidth
	}
	priority := func(kind string) int {
		switch kind {
		case "capture":
			return 0
		case "local":
			return 1
		case "parameter":
			return 2
		case "member":
			return 3
		default:
			return 4
		}
	}
	if priority(candidate.Kind) != priority(current.Kind) {
		return priority(candidate.Kind) < priority(current.Kind)
	}
	return candidate.Line >= current.Line
}

func cppSameDeclaration(left support.ParsedDeclaration, right support.ParsedDeclaration) bool {
	return left.Name == right.Name && left.Kind == right.Kind && left.Line == right.Line && left.ScopeStart == right.ScopeStart && left.ScopeEnd == right.ScopeEnd && left.ReferenceShape == right.ReferenceShape
}

func cppDeclarationExcluded(candidate support.ParsedDeclaration, excluded *support.ParsedDeclaration) bool {
	if excluded == nil {
		return false
	}
	if cppSameDeclaration(candidate, *excluded) {
		return true
	}
	return candidate.Kind == "capture" && excluded.Kind == "capture" && candidate.ScopeStart == excluded.ScopeStart && candidate.ScopeEnd == excluded.ScopeEnd
}

func cppDefaultCaptureOverrides(capture support.ParsedDeclaration, declaration support.ParsedDeclaration) bool {
	// Globals and members are not captured automatic variables. Member access
	// remains rooted at the captured this pointer, including under [=].
	if declaration.Kind == "global" || declaration.Kind == "member" {
		return false
	}
	// A declaration whose lexical scope is the lambda body belongs to the
	// lambda; the default capture only supplies names from an outer scope.
	if declaration.Kind == "local" && declaration.Line >= capture.Line &&
		declaration.ScopeStart >= capture.ScopeStart && declaration.ScopeEnd <= capture.ScopeEnd {
		return false
	}
	captureWidth := capture.ScopeEnd - capture.ScopeStart
	declarationWidth := declaration.ScopeEnd - declaration.ScopeStart
	if captureWidth != declarationWidth {
		return captureWidth < declarationWidth
	}
	switch declaration.Kind {
	case "parameter":
		return true
	default:
		return false
	}
}

func cppInitializerAllocatesLocal(initializer string) bool {
	lower := strings.ToLower(strings.TrimSpace(initializer))
	return strings.HasPrefix(lower, "new ") || strings.HasPrefix(lower, "std::make_") || strings.HasPrefix(lower, "{")
}

func cppAutoCallResultIsUnknown(declaration support.ParsedDeclaration) bool {
	if strings.TrimSpace(declaration.Type) != "auto" {
		return false
	}
	initializer := strings.TrimSpace(declaration.Initializer)
	if initializer == "" || strings.HasPrefix(initializer, "{") || strings.HasPrefix(initializer, "[") || strings.HasPrefix(initializer, "std::move") {
		return false
	}
	return cppAutoCallResultPattern.MatchString(initializer)
}
