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
	cppFieldMutationPattern  = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*(?:(?:\.|->)\s*[A-Za-z_]\w*|\[[^]]*\])\s*(?:=\s|\+\+|--|\+=|-=|\*=|/=)`)
	cppBareMutationPattern   = regexp.MustCompile(`(?:^|[;{}])[ \t]*(?:\*[ \t]*)?([A-Za-z_]\w*)[ \t]*(?:=\s|\+\+|--|\+=|-=|\*=|/=)`)
	cppPrefixMutationPattern = regexp.MustCompile(`(?:^|[;{}])[ \t]*(?:\+\+|--)[ \t]*(?:\*[ \t]*)?([A-Za-z_]\w*)\b`)
	cppEscapeStorePattern    = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*(?:(?:\.|->)\s*[A-Za-z_]\w*)?[ \t]*=[ \t]*&?([A-Za-z_]\w*)\b`)
	cppAutoCallResultPattern = regexp.MustCompile(`^\s*[A-Za-z_]\w*(?:::[A-Za-z_]\w*)*(?:\s*<[^(){};]+>)?\s*\(`)
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
			if !ok || ownership.origin == originUnknown || ownership.target == "" {
				addUnresolved(statement.Line, "assignment", name)
				continue
			}
			if ownership.target != targetReceiver && ownership.target != targetGlobal && ownership.target != targetArgument {
				continue
			}
			resolveMutation(name, statement.Line, "assignment", name, "shared_state")
		}
		for _, match := range cppPrefixMutationPattern.FindAllStringSubmatch(text, -1) {
			name := match[1]
			ownership, ok := resolver.resolve(name, statement.Line, nil, 0)
			if !ok || ownership.origin == originUnknown || ownership.target == "" {
				addUnresolved(statement.Line, "assignment", name)
				continue
			}
			if ownership.target != targetReceiver && ownership.target != targetGlobal && ownership.target != targetArgument {
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
		return cppResolvedOwnership{origin: originCaller, target: targetReceiver, name: name, shape: "pointer"}, true
	}
	candidates := make([]support.ParsedDeclaration, 0)
	for idx := range r.fn.Declarations {
		declaration := r.fn.Declarations[idx]
		if declaration.Name != name || cppDeclarationExcluded(declaration, excluded) {
			continue
		}
		if declaration.Kind != "global" && declaration.Kind != "member" {
			if declaration.Line > line || line < declaration.ScopeStart || line > declaration.ScopeEnd {
				continue
			}
		}
		candidates = append(candidates, declaration)
	}
	var best support.ParsedDeclaration
	if len(candidates) > 0 {
		best = candidates[0]
		for _, candidate := range candidates[1:] {
			if cppDeclarationMoreSpecific(candidate, best) {
				best = candidate
			}
		}
	}
	if capture, ok := r.defaultCapture(line, excluded); ok && (len(candidates) == 0 || cppDefaultCaptureOverrides(capture, best)) {
		if capture.ReferenceShape == "reference" {
			return r.resolve(name, capture.Line, &capture, depth+1)
		}
		return cppResolvedOwnership{origin: originLocal, target: targetLocal, name: name}, true
	}
	if len(candidates) == 0 {
		return cppResolvedOwnership{origin: originUnknown, name: name}, false
	}
	return r.resolveDeclaration(best, depth)
}

func (r cppOwnershipResolver) resolveDeclaration(declaration support.ParsedDeclaration, depth int) (cppResolvedOwnership, bool) {
	switch declaration.Kind {
	case "parameter":
		if declaration.ReferenceShape == "reference" || declaration.ReferenceShape == "pointer" {
			return cppResolvedOwnership{origin: originCaller, target: targetArgument, name: declaration.Name, shape: declaration.ReferenceShape}, true
		}
		return cppResolvedOwnership{origin: originLocal, target: targetLocal, name: declaration.Name}, true
	case "capture":
		if declaration.ReferenceShape != "reference" {
			return cppResolvedOwnership{origin: originLocal, target: targetLocal, name: declaration.Name}, true
		}
		source := firstNonEmptyString(declaration.AliasSource, declaration.Name)
		return r.resolve(source, declaration.Line, &declaration, depth+1)
	case "member":
		return cppResolvedOwnership{origin: originCaller, target: targetReceiver, name: declaration.Name, shape: declaration.ReferenceShape}, true
	case "global":
		return cppResolvedOwnership{origin: originShared, target: targetGlobal, name: declaration.Name, shape: declaration.ReferenceShape}, true
	case "local":
		if declaration.ReferenceShape == "reference" || declaration.ReferenceShape == "pointer" {
			if declaration.AliasSource != "" {
				source, ok := r.resolve(declaration.AliasSource, declaration.Line, &declaration, depth+1)
				if !ok {
					return source, false
				}
				source.shape = declaration.ReferenceShape
				return source, true
			}
			if cppInitializerAllocatesLocal(declaration.Initializer) {
				return cppResolvedOwnership{origin: originLocal, target: targetLocal, name: declaration.Name}, true
			}
			return cppResolvedOwnership{origin: originUnknown, name: declaration.Name}, false
		}
		if strings.TrimSpace(declaration.Type) == "auto" && declaration.AliasSource != "" && !strings.HasPrefix(strings.TrimSpace(declaration.Initializer), "std::move") {
			source, ok := r.resolve(declaration.AliasSource, declaration.Line, &declaration, depth+1)
			if !ok {
				return cppResolvedOwnership{origin: originUnknown, name: declaration.Name}, false
			}
			if strings.HasPrefix(strings.TrimSpace(declaration.Initializer), "&") {
				source.shape = "pointer"
				return source, true
			}
			if source.shape == "pointer" {
				return source, true
			}
			return cppResolvedOwnership{origin: originLocal, target: targetLocal, name: declaration.Name}, true
		}
		if cppAutoCallResultIsUnknown(declaration) {
			return cppResolvedOwnership{origin: originUnknown, name: declaration.Name}, false
		}
		return cppResolvedOwnership{origin: originLocal, target: targetLocal, name: declaration.Name}, true
	default:
		return cppResolvedOwnership{origin: originUnknown, name: declaration.Name}, false
	}
}

func (r cppOwnershipResolver) defaultCapture(line int, excluded *support.ParsedDeclaration) (support.ParsedDeclaration, bool) {
	var best support.ParsedDeclaration
	found := false
	for _, declaration := range r.fn.Declarations {
		if declaration.Kind != "capture" || declaration.Name != "*" || line < declaration.ScopeStart || line > declaration.ScopeEnd || cppDeclarationExcluded(declaration, excluded) {
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
