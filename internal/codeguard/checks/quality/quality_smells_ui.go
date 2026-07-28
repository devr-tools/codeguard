package quality

import "strings"

func isLikelyUIFile(file string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	return strings.HasSuffix(normalized, ".tsx") || strings.HasSuffix(normalized, ".jsx") ||
		strings.Contains(normalized, ".native.") ||
		strings.Contains(normalized, "/_components/") || strings.Contains(normalized, "/components/") ||
		strings.Contains(normalized, "/screens/") || strings.Contains(normalized, "/navigation/") ||
		strings.Contains(normalized, "/react-native/") || strings.Contains(normalized, "/apps/mobile/") ||
		strings.Contains(normalized, "/packages/mobile/") ||
		strings.Contains(normalized, "/app/") && strings.Contains(normalized, "web/")
}

func isStructuralUIRenderingContext(file string, fn structuralFunction) bool {
	precisionFn := precisionFunction{Name: fn.Name, StartLine: fn.StartLine, Body: fn.Body}
	if isUIHelperOrMappingContext(file, precisionFn) {
		return true
	}
	if !isScriptLikeSourcePath(file) || !isLikelyUIFile(file) {
		return false
	}
	return isUIRenderHelperName(fn.Name) || isUIRenderMappingBody(fn.Body)
}

func isUIRenderHelperName(name string) bool {
	lowered := strings.ToLower(strings.Trim(name, "_$"))
	for _, token := range []string{
		"build", "card", "cell", "collect", "derive", "display", "filter", "format",
		"header", "item", "label", "map", "option", "props", "render", "row",
		"screen", "section", "select", "style", "theme", "view",
	} {
		if strings.Contains(lowered, token) {
			return true
		}
	}
	return false
}

func isUIRenderMappingBody(body string) bool {
	lowered := strings.ToLower(body)
	return strings.Contains(lowered, "return <") ||
		strings.Contains(lowered, "jsx") ||
		strings.Contains(lowered, ".map(") ||
		strings.Contains(lowered, "stylesheet.") ||
		strings.Contains(lowered, "classname") ||
		strings.Contains(lowered, "navigation.") ||
		strings.Contains(lowered, "route.params") ||
		strings.Contains(lowered, "props.") && (strings.Contains(lowered, "theme") || strings.Contains(lowered, "style"))
}
