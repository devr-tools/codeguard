package quality

import (
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
)

func isReactComponentOrHookBoundary(file string, fn precisionFunction) bool {
	if !isScriptLikeSourcePath(file) {
		return false
	}
	if isReactHookName(fn.Name) || isReactHookFile(file) {
		return true
	}
	if isTSXLikeSourcePath(file) && isReactComponentName(fn.Name) {
		return true
	}
	body := strings.ToLower(fn.Body)
	return isTSXLikeSourcePath(file) && (strings.Contains(body, "jsx") || strings.Contains(body, "return <") || strings.Contains(body, "react."))
}

func isTSXLikeSourcePath(file string) bool {
	lowered := strings.ToLower(file)
	return strings.HasSuffix(lowered, ".tsx") || strings.HasSuffix(lowered, ".jsx")
}

func isReactComponentName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	first := rune(name[0])
	return first >= 'A' && first <= 'Z'
}

func isUIConventionalAmbiguousName(file string, fn precisionFunction, name string, typ string, line int) bool {
	if !isScriptLikeSourcePath(file) {
		return false
	}
	normalized := strings.ToLower(strings.Trim(name, "_$"))
	if !isUIConventionalAmbiguousToken(normalized) {
		return false
	}
	if isReactComponentOrHookBoundary(file, fn) {
		return true
	}
	if strings.HasPrefix(strings.ToLower(fn.Name), "on") || strings.Contains(strings.ToLower(fn.Name), "render") {
		return true
	}
	if strings.Contains(strings.ToLower(typ), "reactnode") || strings.Contains(strings.ToLower(typ), "jsx") {
		return true
	}
	return nearbyUICallbackStatement(fn, line)
}

func isUIConventionalAmbiguousToken(name string) bool {
	switch name {
	case "value", "values", "item", "items", "data", "utils":
		return true
	default:
		return false
	}
}

func nearbyUICallbackStatement(fn precisionFunction, line int) bool {
	for _, statement := range fn.Statements {
		if statement.Line < line-2 || statement.Line > line+2 {
			continue
		}
		lowered := strings.ToLower(statement.Raw + " " + statement.Text)
		if strings.Contains(lowered, ".map(") || strings.Contains(lowered, "onchange") ||
			strings.Contains(lowered, "onsave") || strings.Contains(lowered, "onclick") ||
			strings.Contains(lowered, "render") || strings.Contains(lowered, "form") {
			return true
		}
	}
	return false
}

func isAllowedBooleanUIName(file string, fn precisionFunction, name string) bool {
	if !isReactComponentOrHookBoundary(file, fn) {
		return false
	}
	switch strings.ToLower(strings.Trim(name, "_$")) {
	case "open", "loading", "active", "pending", "checked", "selected", "expanded", "collapsed":
		return true
	default:
		return false
	}
}

func conventionalCardinalityName(name string) bool {
	switch strings.ToLower(strings.Trim(name, "_$")) {
	case "args", "rows", "ids", "next", "out", "props", "searchparams", "params", "item", "items", "status":
		return true
	default:
		return strings.HasSuffix(name, "params") ||
			strings.HasSuffix(name, "props") ||
			strings.HasSuffix(name, "args")
	}
}

func moduleStatementLooksTopLevel(statement support.ParsedStatement) bool {
	return statement.Indent == 0
}

func scriptSourceLineAtModuleScope(source string, targetIdx int) bool {
	depth := 0
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	for idx, line := range lines {
		if idx == targetIdx {
			return depth == 0 && len(line) == len(strings.TrimLeft(line, " \t"))
		}
		depth += braceDelta(maskCommentSuffix(line))
		if depth < 0 {
			depth = 0
		}
	}
	return true
}

func maskCommentSuffix(line string) string {
	if idx := strings.Index(line, "//"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func braceDelta(line string) int {
	delta := 0
	for _, r := range line {
		switch r {
		case '{':
			delta++
		case '}':
			delta--
		}
	}
	return delta
}
