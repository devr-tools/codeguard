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

// withFindingFilterProperties adds the filter and paging inputs shared by the
// findings-returning tools (scan, validate_patch) to a tool's own properties.
func withFindingFilterProperties(props map[string]any) map[string]any {
	stringList := func(description string) map[string]any {
		return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": description}
	}
	props["sections"] = stringList("Only return findings from these section ids (e.g. security, quality, prompts).")
	props["rules"] = stringList("Only return findings whose rule id matches. An entry ending in \".\" or \"*\" is a prefix (e.g. \"security.\"); anything else is an exact rule id.")
	props["min_level"] = map[string]any{"type": "string", "enum": []string{"warn", "fail"}, "description": "Drop findings below this level."}
	props["min_confidence"] = map[string]any{
		"type":        "string",
		"enum":        []string{"low", "medium", "high"},
		"description": "Drop findings below this confidence. Sets checks.min_confidence.default like the CLI -min-confidence flag; per-section overrides still apply. Unspecified confidence counts as medium.",
	}
	props["paths"] = stringList("Only return findings whose path matches one of these globs (\"*\" within a path segment, \"**\" across segments).")
	props["max_findings"] = map[string]any{
		"type":        "integer",
		"minimum":     0,
		"maximum":     mcpMaxFindingsCeiling,
		"default":     mcpDefaultMaxFindings,
		"description": "Maximum findings in this page. 0 returns only the summary (plus next_cursor when findings matched).",
	}
	props["cursor"] = map[string]any{
		"type":        "string",
		"description": "next_cursor from a previous call. Pages through that call's cached report without re-running it; filters must match the original call.",
	}
	props["verbose"] = map[string]any{
		"type":        "boolean",
		"description": "Include long evidence per finding (title, severity, why, column, fingerprint, metadata), non-operational diagnostics, and report artifacts.",
	}
	return props
}

func findingsOutputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":         map[string]any{"type": "string"},
			"profile":      map[string]any{"type": "string"},
			"generated_at": map[string]any{"type": "string"},
			"scan_id":      map[string]any{"type": "string"},
			"summary": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"total_findings":    map[string]any{"type": "integer"},
					"matched_findings":  map[string]any{"type": "integer"},
					"returned_findings": map[string]any{"type": "integer"},
					"filtered_out":      map[string]any{"type": "object"},
					"top_rules":         map[string]any{"type": "array"},
					"sections":          map[string]any{"type": "array"},
				},
			},
			"findings":    map[string]any{"type": "array"},
			"offset":      map[string]any{"type": "integer"},
			"truncated":   map[string]any{"type": "boolean"},
			"next_cursor": map[string]any{"type": "string"},
			"filters":     map[string]any{"type": "object"},
		},
		"required": []string{"summary", "findings", "truncated"},
	}
}
