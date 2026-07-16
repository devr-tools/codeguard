package compdb

import (
	"path/filepath"
	"strings"
)

type metadataKind uint8

const (
	metadataUnknown metadataKind = iota
	metadataInclude
	metadataDefine
	metadataUndefine
	metadataStandard
)

type metadataPrefix struct {
	prefix string
	kind   metadataKind
}

var separatedMetadata = map[string]metadataKind{
	"-I": metadataInclude, "-isystem": metadataInclude, "-iquote": metadataInclude, "/I": metadataInclude,
	"-D": metadataDefine, "/D": metadataDefine,
	"-U": metadataUndefine, "/U": metadataUndefine,
}

var prefixedMetadata = []metadataPrefix{
	{"-isystem", metadataInclude}, {"-iquote", metadataInclude},
	{"-std=", metadataStandard}, {"/std:", metadataStandard},
	{"-I", metadataInclude}, {"/I", metadataInclude},
	{"-D", metadataDefine}, {"/D", metadataDefine},
	{"-U", metadataUndefine}, {"/U", metadataUndefine},
}

func (entry *Entry) extractMetadata(root string, arguments []string) {
	entry.Compiler = compilerFromArguments(arguments)
	for index := 1; index < len(arguments); index++ {
		kind, value, separated := classifyMetadata(arguments[index])
		if separated && index+1 >= len(arguments) {
			continue
		}
		if separated {
			index++
			value = arguments[index]
		}
		entry.applyMetadata(root, kind, value)
	}
}

func compilerFromArguments(arguments []string) string {
	if len(arguments) == 0 {
		return ""
	}
	if len(arguments) > 1 && isCompilerWrapper(arguments[0]) {
		return arguments[1]
	}
	return arguments[0]
}

func classifyMetadata(argument string) (metadataKind, string, bool) {
	if kind, ok := separatedMetadata[argument]; ok {
		return kind, "", true
	}
	for _, spec := range prefixedMetadata {
		if strings.HasPrefix(argument, spec.prefix) && len(argument) > len(spec.prefix) {
			return spec.kind, argument[len(spec.prefix):], false
		}
	}
	return metadataUnknown, "", false
}

func (entry *Entry) applyMetadata(root string, kind metadataKind, value string) {
	switch kind {
	case metadataInclude:
		entry.addInclude(root, value)
	case metadataDefine:
		entry.Defines = append(entry.Defines, value)
	case metadataUndefine:
		entry.Undefines = append(entry.Undefines, value)
	case metadataStandard:
		entry.Standard = value
	}
}

func isCompilerWrapper(command string) bool {
	switch strings.ToLower(filepath.Base(command)) {
	case "ccache", "sccache", "distcc", "icecc":
		return true
	default:
		return false
	}
}
