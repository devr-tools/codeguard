package support

import (
	"path/filepath"
	"strings"
)

type ScriptFlavor string

const (
	ScriptFlavorTypeScript ScriptFlavor = "typescript"
	ScriptFlavorJavaScript ScriptFlavor = "javascript"
)

func ScriptFlavorForPath(path string) ScriptFlavor {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".tsx", ".mts", ".cts":
		return ScriptFlavorTypeScript
	case ".js", ".jsx", ".mjs", ".cjs":
		return ScriptFlavorJavaScript
	default:
		return ""
	}
}

func IsTypeScriptLikeFile(path string) bool {
	return ScriptFlavorForPath(path) != ""
}

func RuleIDForScript(path string, typeScriptRuleID string, javaScriptRuleID string) string {
	if ScriptFlavorForPath(path) == ScriptFlavorJavaScript && strings.TrimSpace(javaScriptRuleID) != "" {
		return javaScriptRuleID
	}
	return typeScriptRuleID
}

func ScriptLabelForPath(path string) string {
	if ScriptFlavorForPath(path) == ScriptFlavorJavaScript {
		return "JavaScript"
	}
	return "TypeScript"
}
