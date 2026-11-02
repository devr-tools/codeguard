package quality

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

type cppOwnershipResolver struct {
	fn precisionFunction
}

func (r cppOwnershipResolver) resolve(name string, line int, excluded *support.ParsedDeclaration, depth int) (cppResolvedOwnership, bool) {
	if depth >= 8 || name == "" {
		return cppResolvedOwnership{origin: originUnknown, name: name}, false
	}
	candidates := cppVisibleDeclarations(r.fn.Declarations, name, line, excluded)
	best, found := mostSpecificCppDeclaration(candidates)
	if capture, ok := r.defaultCapture(line, excluded); ok && (!found || cppDefaultCaptureOverrides(capture, best)) {
		return r.resolveDefaultCapture(name, capture, depth)
	}
	if !found {
		if name == "this" && r.fn.QualifiedOwner != "" {
			return cppResolvedOwnership{origin: originCaller, target: targetReceiver, name: name, shape: "pointer"}, true
		}
		return cppResolvedOwnership{origin: originUnknown, name: name}, false
	}
	if best.Kind == "member" {
		if capture, ok := r.thisCapture(line, excluded); ok {
			if capture.ReferenceShape == "object" {
				return cppResolvedOwnership{origin: originLocal, target: targetLocal, name: name}, true
			}
			return cppResolvedOwnership{origin: originCaller, target: targetReceiver, name: name, shape: "pointer"}, true
		}
	}
	return r.resolveDeclaration(best, depth)
}

// resolveDefaultCapture resolves a name captured by a lambda's default
// capture: by reference it keeps the outer ownership; by copy only a copied
// pointer still reaches the outer object.
func (r cppOwnershipResolver) resolveDefaultCapture(name string, capture support.ParsedDeclaration, depth int) (cppResolvedOwnership, bool) {
	if capture.ReferenceShape == "reference" {
		return r.resolve(name, capture.Line, &capture, depth+1)
	}
	if source, sourceOK := r.resolve(name, capture.Line, &capture, depth+1); sourceOK && source.shape == "pointer" {
		return source, true
	}
	return cppResolvedOwnership{origin: originLocal, target: targetLocal, name: name}, true
}

func (r cppOwnershipResolver) resolveDeclaration(declaration support.ParsedDeclaration, depth int) (cppResolvedOwnership, bool) {
	switch declaration.Kind {
	case "parameter":
		if declaration.ReferenceShape == "reference" || declaration.ReferenceShape == "pointer" {
			return cppResolvedOwnership{origin: originCaller, target: targetArgument, name: declaration.Name, shape: declaration.ReferenceShape}, true
		}
		return cppLocalOwnership(declaration.Name), true
	case "capture":
		return r.resolveCapture(declaration, depth)
	case "member":
		return cppResolvedOwnership{origin: originCaller, target: targetReceiver, name: declaration.Name, shape: declaration.ReferenceShape}, true
	case "global":
		return cppResolvedOwnership{origin: originShared, target: targetGlobal, name: declaration.Name, shape: declaration.ReferenceShape}, true
	case "local":
		return r.resolveLocal(declaration, depth)
	default:
		return cppResolvedOwnership{origin: originUnknown, name: declaration.Name}, false
	}
}

// resolveCapture resolves an explicit lambda capture: by-value and
// init-captures are local copies unless they copy a pointer; by-reference
// captures keep the captured variable's ownership.
func (r cppOwnershipResolver) resolveCapture(declaration support.ParsedDeclaration, depth int) (cppResolvedOwnership, bool) {
	if declaration.ReferenceShape == "object" || (declaration.Initializer != "" && declaration.AliasSource == "") {
		return cppLocalOwnership(declaration.Name), true
	}
	source := firstNonEmptyString(declaration.AliasSource, declaration.Name)
	resolved, ok := r.resolve(source, declaration.Line, &declaration, depth+1)
	if !ok {
		return resolved, false
	}
	switch {
	case declaration.ReferenceShape == "reference":
		return resolved, true
	case declaration.ReferenceShape == "pointer":
		resolved.shape = "pointer"
		return resolved, true
	case resolved.shape == "pointer":
		return resolved, true
	}
	return cppLocalOwnership(declaration.Name), true
}

// resolveLocal resolves a local variable: references and pointers follow
// their alias source, auto copies follow pointer sources, and other locals
// are owned by the function unless initialized from an unknown call.
func (r cppOwnershipResolver) resolveLocal(declaration support.ParsedDeclaration, depth int) (cppResolvedOwnership, bool) {
	if declaration.ReferenceShape == "reference" || declaration.ReferenceShape == "pointer" {
		return r.resolveLocalReference(declaration, depth)
	}
	initializer := strings.TrimSpace(declaration.Initializer)
	if strings.TrimSpace(declaration.Type) == "auto" && declaration.AliasSource != "" && !strings.HasPrefix(initializer, "std::move") {
		source, ok := r.resolve(declaration.AliasSource, declaration.Line, &declaration, depth+1)
		if !ok {
			return cppResolvedOwnership{origin: originUnknown, name: declaration.Name}, false
		}
		if strings.HasPrefix(initializer, "&") {
			source.shape = "pointer"
			return source, true
		}
		if source.shape == "pointer" {
			return source, true
		}
		return cppLocalOwnership(declaration.Name), true
	}
	if cppAutoCallResultIsUnknown(declaration) {
		return cppResolvedOwnership{origin: originUnknown, name: declaration.Name}, false
	}
	return cppLocalOwnership(declaration.Name), true
}

// resolveLocalReference resolves a local reference or pointer through its
// alias source, or as local storage when it points at a fresh allocation.
func (r cppOwnershipResolver) resolveLocalReference(declaration support.ParsedDeclaration, depth int) (cppResolvedOwnership, bool) {
	if declaration.AliasSource != "" {
		source, ok := r.resolve(declaration.AliasSource, declaration.Line, &declaration, depth+1)
		if !ok {
			return source, false
		}
		source.shape = declaration.ReferenceShape
		return source, true
	}
	if cppInitializerAllocatesLocal(declaration.Initializer) {
		return cppLocalOwnership(declaration.Name), true
	}
	return cppResolvedOwnership{origin: originUnknown, name: declaration.Name}, false
}

func cppLocalOwnership(name string) cppResolvedOwnership {
	return cppResolvedOwnership{origin: originLocal, target: targetLocal, name: name}
}

// cppVisibleDeclarations returns the declarations of name visible at line:
// globals and members anywhere, other declarations only within their scope
// and after their declaration line.
func cppVisibleDeclarations(declarations []support.ParsedDeclaration, name string, line int, excluded *support.ParsedDeclaration) []support.ParsedDeclaration {
	candidates := make([]support.ParsedDeclaration, 0)
	for idx := range declarations {
		declaration := declarations[idx]
		if declaration.Name != name || cppDeclarationExcluded(declaration, excluded) {
			continue
		}
		scoped := declaration.Kind != "global" && declaration.Kind != "member"
		if scoped && (declaration.Line > line || line < declaration.ScopeStart || line > declaration.ScopeEnd) {
			continue
		}
		candidates = append(candidates, declaration)
	}
	return candidates
}

func mostSpecificCppDeclaration(candidates []support.ParsedDeclaration) (support.ParsedDeclaration, bool) {
	if len(candidates) == 0 {
		return support.ParsedDeclaration{}, false
	}
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if cppDeclarationMoreSpecific(candidate, best) {
			best = candidate
		}
	}
	return best, true
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

func (r cppOwnershipResolver) thisCapture(line int, excluded *support.ParsedDeclaration) (support.ParsedDeclaration, bool) {
	var best support.ParsedDeclaration
	found := false
	for _, declaration := range r.fn.Declarations {
		if declaration.Kind != "capture" || declaration.Name != "this" || line < declaration.ScopeStart || line > declaration.ScopeEnd || cppDeclarationExcluded(declaration, excluded) {
			continue
		}
		if !found || cppDeclarationMoreSpecific(declaration, best) {
			best, found = declaration, true
		}
	}
	return best, found
}
