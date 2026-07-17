package support

import (
	"regexp"
	"strings"
)

var (
	CPPModuleDeclarationPattern = regexp.MustCompile(`(?m)^[ \t]*(?:export[ \t]+)?module[ \t]+([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*(?::[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)?)[ \t]*;`)
	CPPModuleImportPattern      = regexp.MustCompile(`(?m)^[ \t]*(?:export[ \t]+)?import[ \t]+((?:[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*(?::[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)?)|(?::[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*))[ \t]*;`)
)

func QualifyCPPModuleImport(specifier string, declaredModule string) string {
	if !strings.HasPrefix(specifier, ":") {
		return specifier
	}
	primary := cppPrimaryModuleName(declaredModule)
	if primary == "" {
		return ""
	}
	return primary + specifier
}

func cppPrimaryModuleName(module string) string {
	module = strings.TrimSpace(module)
	if module == "" {
		return ""
	}
	if cut := strings.IndexByte(module, ':'); cut >= 0 {
		return module[:cut]
	}
	return module
}
