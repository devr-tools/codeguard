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
	if isReactNativeComponentOrScreenBoundary(file, fn) {
		return true
	}
	if isTSXLikeSourcePath(file) && isReactComponentName(fn.Name) {
		return true
	}
	body := strings.ToLower(fn.Body)
	return isTSXLikeSourcePath(file) && (strings.Contains(body, "jsx") || strings.Contains(body, "return <") || strings.Contains(body, "react."))
}

func isReactComponentOrNamedHookBoundary(file string, fn precisionFunction) bool {
	if !isScriptLikeSourcePath(file) {
		return false
	}
	if isReactHookName(fn.Name) {
		return true
	}
	if isReactNativeComponentOrScreenBoundary(file, fn) {
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

func isEventHandlerName(name string) bool {
	name = strings.TrimSpace(name)
	if len(name) > len("on") && strings.HasPrefix(name, "on") {
		next := rune(name[len("on")])
		return next >= 'A' && next <= 'Z'
	}
	if len(name) <= len("handle") || !strings.HasPrefix(name, "handle") {
		return false
	}
	next := rune(name[len("handle")])
	return next >= 'A' && next <= 'Z'
}

func isReactNativeComponentOrScreenBoundary(file string, fn precisionFunction) bool {
	if !isScriptLikeSourcePath(file) {
		return false
	}
	if !isReactNativeContext(file, fn) {
		return false
	}
	return isReactComponentName(fn.Name) || strings.Contains(strings.ToLower(fn.Name), "screen")
}

func isReactNativeContext(file string, fn precisionFunction) bool {
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	if strings.Contains(normalized, ".native.") ||
		strings.Contains(normalized, "/screens/") ||
		strings.Contains(normalized, "/screen/") {
		return true
	}
	body := strings.ToLower(fn.Body)
	return strings.Contains(body, "react-native") ||
		strings.Contains(body, "stylesheet.create") ||
		strings.Contains(body, "<flatlist") ||
		strings.Contains(body, "<pressable") ||
		strings.Contains(body, "<touchable")
}

func isUIConventionalAmbiguousName(file string, fn precisionFunction, name string, typ string, line int) bool {
	if !isScriptLikeSourcePath(file) {
		return false
	}
	normalized := strings.ToLower(strings.Trim(name, "_$"))
	if !isUIConventionalAmbiguousToken(normalized) {
		return false
	}
	if isFrameworkConventionalAmbiguousName(file, fn, name) {
		return true
	}
	if isReactComponentOrHookBoundary(file, fn) {
		return true
	}
	if isReactNativeContext(file, fn) {
		return true
	}
	loweredFnName := strings.ToLower(fn.Name)
	if strings.HasPrefix(loweredFnName, "on") || strings.HasPrefix(loweredFnName, "handle") ||
		strings.Contains(loweredFnName, "render") || strings.Contains(loweredFnName, "keyextractor") {
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
		if strings.Contains(lowered, ".map(") || strings.Contains(lowered, ".foreach(") ||
			strings.Contains(lowered, "onchange") || strings.Contains(lowered, "onsave") ||
			strings.Contains(lowered, "onclick") || strings.Contains(lowered, "onpress") ||
			strings.Contains(lowered, "onsubmit") || strings.Contains(lowered, "render") ||
			strings.Contains(lowered, "renderitem") || strings.Contains(lowered, "keyextractor") ||
			strings.Contains(lowered, "flatlist") || strings.Contains(lowered, "pressable") ||
			strings.Contains(lowered, "touchable") || strings.Contains(lowered, "form") {
			return true
		}
	}
	return false
}

func isAllowedBooleanUIName(file string, fn precisionFunction, name string) bool {
	if isEventHandlerName(name) {
		return true
	}
	if isConventionalNonPredicateName(name) {
		return true
	}
	if isResourceIdentifierName(name) {
		return true
	}
	if !isReactComponentOrHookBoundary(file, fn) {
		return false
	}
	normalized := strings.ToLower(strings.Trim(name, "_$"))
	for _, suffix := range []string{"active", "visible", "enabled", "disabled", "open", "closed", "expanded", "collapsed", "selected", "checked", "pending", "loading"} {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	switch normalized {
	case "open", "loading", "active", "pending", "checked", "selected", "expanded", "collapsed":
		return true
	default:
		return false
	}
}

func isConventionalNonPredicateName(name string) bool {
	switch strings.ToLower(strings.Trim(name, "_$")) {
	case "asrecord", "cached", "opts", "options", "message", "classname", "class", "icon", "submit", "compare", "parser", "parse", "builder", "build", "renderer", "render":
		return true
	default:
		return false
	}
}

func isResourceIdentifierName(name string) bool {
	lowered := strings.ToLower(strings.Trim(name, "_$"))
	for _, suffix := range []string{"arn", "url", "uri", "id", "ids", "key", "token", "secret", "rolearn"} {
		if strings.HasSuffix(lowered, suffix) {
			return true
		}
	}
	return false
}

func conventionalCardinalityName(name string) bool {
	base := strings.ToLower(strings.Trim(name, "_$"))
	switch base {
	case "all", "answers", "args", "claims", "columns", "contracts", "docs", "entries", "files", "filtered", "ids", "items", "k", "keys", "krs", "matters", "messages", "next", "out", "params", "props", "quarters", "records", "risks", "rows", "searchparams", "sections", "source", "status", "thresholds", "users", "versions", "v", "i", "j", "x", "y":
		return true
	default:
		return len(name) <= 2 ||
			strings.HasSuffix(base, "dates") ||
			strings.HasSuffix(base, "fields") ||
			strings.HasSuffix(base, "filters") ||
			strings.HasSuffix(base, "ids") ||
			strings.HasSuffix(base, "links") ||
			strings.HasSuffix(base, "options") ||
			strings.HasSuffix(base, "points") ||
			strings.HasSuffix(base, "rows") ||
			strings.HasSuffix(base, "items") ||
			strings.HasSuffix(base, "entries") ||
			strings.HasSuffix(base, "params") ||
			strings.HasSuffix(base, "props") ||
			strings.HasSuffix(base, "args") ||
			strings.HasSuffix(base, "columns") ||
			strings.HasSuffix(base, "sections") ||
			strings.HasSuffix(base, "snapshots") ||
			strings.HasSuffix(base, "tasks") ||
			strings.HasSuffix(base, "types") ||
			strings.HasSuffix(base, "weeks")
	}
}

func isUIHelperOrMappingContext(file string, fn precisionFunction) bool {
	if !isScriptLikeSourcePath(file) {
		return false
	}
	if isReactComponentOrHookBoundary(file, fn) {
		return true
	}
	loweredName := strings.ToLower(fn.Name)
	if strings.Contains(loweredName, "render") || strings.Contains(loweredName, "map") ||
		strings.Contains(loweredName, "format") || strings.Contains(loweredName, "filter") ||
		strings.Contains(loweredName, "columns") || strings.Contains(loweredName, "options") ||
		strings.Contains(loweredName, "screen") {
		return true
	}
	loweredFile := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	return strings.Contains(loweredFile, "/_components/") || strings.Contains(loweredFile, "/components/") ||
		strings.Contains(loweredFile, "/screens/") ||
		strings.Contains(loweredFile, "/app/") && strings.Contains(loweredFile, "web/")
}

func isUICommandHelperName(file string, name string) bool {
	normalizedFile := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	if strings.Contains(normalizedFile, "/api/") || !isScriptLikeSourcePath(file) {
		return false
	}
	if !strings.Contains(normalizedFile, "/_components/") &&
		!strings.Contains(normalizedFile, "/components/") &&
		!strings.Contains(normalizedFile, "/screens/") &&
		!isTSXLikeSourcePath(file) {
		return false
	}
	lowered := strings.ToLower(strings.Trim(name, "_$"))
	for _, prefix := range []string{"click", "confirm", "disarm", "finish", "mark", "pick", "prefill", "select", "start"} {
		if strings.HasPrefix(lowered, prefix) {
			return true
		}
	}
	return false
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
