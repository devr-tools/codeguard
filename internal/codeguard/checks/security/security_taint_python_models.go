package security

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

// pyModelBindings holds the import-gated Python framework models. A local
// object named request cannot enable one of these models by itself.
type pyModelBindings struct {
	django  bool
	fastAPI bool
	types   map[string]string
	// callees maps a locally bound import name to the qualified name it
	// stands for, so `from urllib.request import urlopen` or
	// `import subprocess as sp` cannot hide a sink behind a short name.
	callees map[string]string
}

func newPyModelBindings(imports []support.ParsedImport) pyModelBindings {
	b := pyModelBindings{types: map[string]string{}, callees: map[string]string{}}
	for _, imp := range imports {
		b.bindCallee(imp)
		module := strings.ToLower(imp.Module)
		switch {
		case strings.HasPrefix(module, "django"):
			b.django = true
			if imp.Name == "HttpRequest" {
				b.types[imp.Alias] = "django"
			}
		case strings.HasPrefix(module, "fastapi"):
			b.fastAPI = true
			if imp.Name == "Request" {
				b.types[imp.Alias] = "fastapi"
			}
		}
	}
	return b
}

func (b pyModelBindings) bindCallee(imp support.ParsedImport) {
	if imp.Alias == "" || imp.Module == "" || strings.HasPrefix(imp.Module, ".") {
		return
	}
	if imp.Name != "" {
		if imp.Name != "*" {
			b.callees[imp.Alias] = imp.Module + "." + imp.Name
		}
		return
	}
	if head, _, _ := strings.Cut(imp.Module, "."); imp.Alias != head {
		b.callees[imp.Alias] = imp.Module
	}
}

// canonicalCallee rewrites the leading import-bound name of a dotted callee
// to its qualified module path; callees that do not start with an imported
// name are returned unchanged.
func (b pyModelBindings) canonicalCallee(callee string) string {
	head, rest, dotted := strings.Cut(callee, ".")
	qualified, ok := b.callees[head]
	if !ok {
		return callee
	}
	if dotted {
		return qualified + "." + rest
	}
	return qualified
}

func (b pyModelBindings) requestModel(param support.ParsedParam) string {
	if model := b.types[strings.TrimSpace(param.Type)]; model != "" {
		return model
	}
	if param.Name == "request" && b.django {
		return "django"
	}
	if param.Name == "request" && b.fastAPI {
		return "fastapi"
	}
	return ""
}

func (b pyModelBindings) djangoRawSink(callee string) bool {
	return b.django && strings.HasSuffix(callee, ".objects.raw")
}
