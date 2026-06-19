package cli

func writeToolAnnotations(title string) map[string]any {
	return map[string]any{
		"title":           title,
		"readOnlyHint":    false,
		"destructiveHint": true,
		"idempotentHint":  false,
		"openWorldHint":   false,
	}
}

func fixFindingSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "The finding to fix, as returned in a scan/validate_patch report.",
		"properties": map[string]any{
			"rule_id":    map[string]any{"type": "string"},
			"path":       map[string]any{"type": "string"},
			"line":       map[string]any{"type": "integer"},
			"message":    map[string]any{"type": "string"},
			"why":        map[string]any{"type": "string"},
			"how_to_fix": map[string]any{"type": "string"},
		},
	}
}

func readOnlyToolAnnotations(title string) map[string]any {
	return map[string]any{
		"title":           title,
		"readOnlyHint":    true,
		"destructiveHint": false,
		"idempotentHint":  true,
		"openWorldHint":   false,
	}
}

func objectOutputSchema() map[string]any {
	return map[string]any{"type": "object"}
}

func reportOutputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":     map[string]any{"type": "string"},
			"profile":  map[string]any{"type": "string"},
			"sections": map[string]any{"type": "array"},
			"summary":  map[string]any{"type": "object"},
		},
	}
}
