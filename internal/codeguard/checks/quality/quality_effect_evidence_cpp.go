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
}

var (
	cppFieldMutationPattern = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*(?:(?:\.|->)\s*[A-Za-z_]\w*|\[[^]]*\])\s*(?:=\s|\+\+|--|\+=|-=|\*=|/=)`)
	cppBareMutationPattern  = regexp.MustCompile(`(?:^|[;{}])[ \t]*(?:\*[ \t]*)?([A-Za-z_]\w*)[ \t]*(?:=\s|\+\+|--|\+=|-=|\*=|/=)`)
	cppEscapeStorePattern   = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*(?:(?:\.|->)\s*[A-Za-z_]\w*)?[ \t]*=[ \t]*&?([A-Za-z_]\w*)\b`)
	cppCallInitializer      = regexp.MustCompile(`^\s*([A-Za-z_]\w*(?:::[A-Za-z_]\w*)*)\s*\(`)
)

// cppFunctionMutationEvidence resolves mutation ownership from bounded
// declaration metadata. Unknown roots remain diagnostic-only evidence.
func cppFunctionMutationEvidence(fn precisionFunction) mutationAnalysis {
	resolver := cppOwnershipResolver{fn: fn}
	escapedAt := resolver.escapedLocals()
	var analysis mutationAnalysis
	seen := map[string]struct{}{}
	unresolvedSeen := map[string]struct{}{}
	addMutation := func(item mutationEvidence) {
		key := item.Target + "|" + item.Effect + "|" + item.Origin + "|" + item.Detail + "|" + strconv.Itoa(item.Line)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		analysis.Mutations = append(analysis.Mutations, item)
	}
	addUnresolved := func(line int, operation string, symbol string) {
		key := operation + "|" + symbol + "|" + strconv.Itoa(line)
		if _, ok := unresolvedSeen[key]; ok {
			return
		}
		unresolvedSeen[key] = struct{}{}
		analysis.Unresolved = append(analysis.Unresolved, unresolvedMutationEvidence{
			Language: "c++", Line: line, Operation: operation, Symbol: symbol,
			Reason: "symbol ownership could not be resolved",
		})
	}
	resolveMutation := func(name string, line int, operation string, detail string, effect string) {
		ownership, ok := resolver.resolve(name, line, nil, 0)
		if !ok || ownership.origin == originUnknown || ownership.target == "" {
			addUnresolved(line, operation, name)
			return
		}
		if ownership.origin == originLocal {
			if escapedAt[ownership.name] == 0 || line < escapedAt[ownership.name] {
				return
			}
			ownership.origin, ownership.target = originShared, targetEscaped
		}
		addMutation(mutationEvidence{Target: ownership.target, Effect: effect, Origin: ownership.origin, Line: line, Detail: detail})
	}

	for _, statement := range directStatements(fn) {
		text := firstNonEmptyString(statement.Raw, statement.Text)
		for _, match := range cppFieldMutationPattern.FindAllStringSubmatch(text, -1) {
			resolveMutation(match[1], statement.Line, "assignment", match[1], "shared_state")
		}
		for _, match := range cppBareMutationPattern.FindAllStringSubmatch(text, -1) {
			name := match[1]
			ownership, ok := resolver.resolve(name, statement.Line, nil, 0)
			if !ok || (ownership.target != targetReceiver && ownership.target != targetGlobal && ownership.target != targetArgument) {
				continue
			}
			resolveMutation(name, statement.Line, "assignment", name, "shared_state")
		}
	}

	for _, call := range directCalls(fn) {
		effect := observableCallEffect(call.Callee)
		root := mutationCallTarget(call.Callee)
		if effect == "" && (!mutatingCallPattern.MatchString(call.Callee) || isConstructionOrHydrationCall(call.Callee)) {
			continue
		}
		if root == "" {
			addUnresolved(call.Line, "call", call.Callee)
			continue
		}
		if effect == "" {
			effect = "shared_state"
		}
		resolveMutation(root, call.Line, "call", call.Callee, effect)
	}
	return analysis
}

type cppOwnershipResolver struct {
	fn precisionFunction
}

func (r cppOwnershipResolver) resolve(name string, line int, excluded *support.ParsedDeclaration, depth int) (cppResolvedOwnership, bool) {
	if depth >= 8 || name == "" {
		return cppResolvedOwnership{origin: originUnknown, name: name}, false
	}
	if name == "this" && r.fn.QualifiedOwner != "" {
		return cppResolvedOwnership{origin: originCaller, target: targetReceiver, name: name}, true
	}
	candidates := make([]support.ParsedDeclaration, 0)
	for idx := range r.fn.Declarations {
		declaration := r.fn.Declarations[idx]
		if declaration.Name != name || (excluded != nil && cppSameDeclaration(declaration, *excluded)) {
			continue
		}
		if declaration.Kind != "global" && declaration.Kind != "member" {
			if declaration.Line > line || line < declaration.ScopeStart || line > declaration.ScopeEnd {
				continue
			}
		}
		candidates = append(candidates, declaration)
	}
	if len(candidates) == 0 {
		if capture, ok := r.defaultCapture(line); ok {
			if capture.ReferenceShape == "reference" {
				return r.resolve(name, capture.Line, &capture, depth+1)
			}
			return cppResolvedOwnership{origin: originLocal, target: targetLocal, name: name}, true
		}
		return cppResolvedOwnership{origin: originUnknown, name: name}, false
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if cppDeclarationMoreSpecific(candidate, best) {
			best = candidate
		}
	}
	return r.resolveDeclaration(best, depth)
}

func (r cppOwnershipResolver) resolveDeclaration(declaration support.ParsedDeclaration, depth int) (cppResolvedOwnership, bool) {
	switch declaration.Kind {
	case "parameter":
		if declaration.ReferenceShape == "reference" || declaration.ReferenceShape == "pointer" {
			return cppResolvedOwnership{origin: originCaller, target: targetArgument, name: declaration.Name}, true
		}
		return cppResolvedOwnership{origin: originLocal, target: targetLocal, name: declaration.Name}, true
	case "capture":
		if declaration.ReferenceShape != "reference" {
			return cppResolvedOwnership{origin: originLocal, target: targetLocal, name: declaration.Name}, true
		}
		source := firstNonEmptyString(declaration.AliasSource, declaration.Name)
		return r.resolve(source, declaration.Line, &declaration, depth+1)
	case "member":
		return cppResolvedOwnership{origin: originCaller, target: targetReceiver, name: declaration.Name}, true
	case "global":
		return cppResolvedOwnership{origin: originShared, target: targetGlobal, name: declaration.Name}, true
	case "local":
		if declaration.ReferenceShape == "reference" || declaration.ReferenceShape == "pointer" {
			if declaration.AliasSource != "" {
				return r.resolve(declaration.AliasSource, declaration.Line, &declaration, depth+1)
			}
			if cppInitializerAllocatesLocal(declaration.Initializer) {
				return cppResolvedOwnership{origin: originLocal, target: targetLocal, name: declaration.Name}, true
			}
			return cppResolvedOwnership{origin: originUnknown, name: declaration.Name}, false
		}
		if cppAutoInitializerIsUnknown(declaration) {
			return cppResolvedOwnership{origin: originUnknown, name: declaration.Name}, false
		}
		return cppResolvedOwnership{origin: originLocal, target: targetLocal, name: declaration.Name}, true
	default:
		return cppResolvedOwnership{origin: originUnknown, name: declaration.Name}, false
	}
}

func (r cppOwnershipResolver) defaultCapture(line int) (support.ParsedDeclaration, bool) {
	var best support.ParsedDeclaration
	found := false
	for _, declaration := range r.fn.Declarations {
		if declaration.Kind != "capture" || declaration.Name != "*" || line < declaration.ScopeStart || line > declaration.ScopeEnd {
			continue
		}
		if !found || declaration.ScopeStart >= best.ScopeStart {
			best, found = declaration, true
		}
	}
	return best, found
}

func (r cppOwnershipResolver) escapedLocals() map[string]int {
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

func cppInitializerAllocatesLocal(initializer string) bool {
	lower := strings.ToLower(strings.TrimSpace(initializer))
	return strings.HasPrefix(lower, "new ") || strings.HasPrefix(lower, "std::make_") || strings.HasPrefix(lower, "{")
}

func cppAutoInitializerIsUnknown(declaration support.ParsedDeclaration) bool {
	if strings.TrimSpace(declaration.Type) != "auto" {
		return false
	}
	initializer := strings.TrimSpace(declaration.Initializer)
	if initializer == "" || strings.HasPrefix(initializer, "{") || strings.HasPrefix(initializer, "[") || strings.HasPrefix(initializer, "std::move") {
		return false
	}
	match := cppCallInitializer.FindStringSubmatch(initializer)
	if len(match) != 2 {
		return false
	}
	callee := match[1]
	lower := strings.ToLower(callee)
	if containsAny(lower, []string{"make", "create", "build", "builder", "integer"}) {
		return false
	}
	if cut := strings.Index(callee, "::"); cut > 0 && callee[0] >= 'A' && callee[0] <= 'Z' {
		return false
	}
	return true
}
