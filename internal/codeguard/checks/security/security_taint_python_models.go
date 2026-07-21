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
}

func newPyModelBindings(imports []support.ParsedImport) pyModelBindings {
	b := pyModelBindings{types: map[string]string{}}
	for _, imp := range imports {
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
