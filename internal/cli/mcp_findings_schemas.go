package cli

// withFindingFilterProperties adds the format, filter and paging inputs shared
// by the findings-returning tools (scan, validate_patch) to a tool's own
// properties.
func withFindingFilterProperties(props map[string]any) map[string]any {
	stringList := func(description string) map[string]any {
		return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": description}
	}
	props["format"] = map[string]any{
		"type": "string",
		"enum": []string{mcpFormatReport, mcpFormatSummary},
		"description": "Response shape. \"summary\" (recommended) returns a compact summary plus one page of findings (default 50) and is selected automatically by any filter or paging input. " +
			"\"report\" returns the full report (top-level sections[] with every finding and its evidence); it is the default when no filter or paging input is passed, for compatibility with 1.10 clients, and may stop being the default in the next major version.",
	}
	props["sections"] = stringList("Summary mode. Only return findings from these section ids (e.g. security, quality, prompts).")
	props["rules"] = stringList("Summary mode. Only return findings whose rule id matches. An entry ending in \".\" or \"*\" is a prefix (e.g. \"security.\"); anything else is an exact rule id.")
	props["min_level"] = map[string]any{"type": "string", "enum": []string{"warn", "fail"}, "description": "Summary mode. Drop findings below this level."}
	props["min_confidence"] = map[string]any{
		"type":        "string",
		"enum":        []string{"low", "medium", "high"},
		"description": "Summary mode. Drop findings below this confidence. Sets checks.min_confidence.default like the CLI -min-confidence flag; per-section overrides still apply. Unspecified confidence counts as medium.",
	}
	props["paths"] = stringList("Summary mode. Only return findings whose path matches one of these globs (\"*\" within a path segment, \"**\" across segments).")
	props["max_findings"] = map[string]any{
		"type":        "integer",
		"minimum":     0,
		"maximum":     mcpMaxFindingsCeiling,
		"default":     mcpDefaultMaxFindings,
		"description": "Summary mode. Maximum findings in this page. 0 returns only the summary (plus next_cursor when findings matched).",
	}
	props["cursor"] = map[string]any{
		"type":        "string",
		"description": "Summary mode. next_cursor from a previous call. Pages through that call's cached report without re-running it; filters must match the original call.",
	}
	props["verbose"] = map[string]any{
		"type":        "boolean",
		"description": "Summary mode. Include long evidence per finding (title, severity, why, column, fingerprint, metadata), non-operational diagnostics, and report artifacts.",
	}
	return props
}

// findingsOutputSchema describes both response shapes of scan and
// validate_patch. The top-level properties are a superset of the schema 1.10
// advertised (name, profile, sections, summary, all loosely typed), so a
// client that ignores combinators still validates either shape; oneOf then
// states precisely which fields each shape carries. The two branches are
// mutually exclusive: only the report has top-level sections, and only the
// summary shape has findings and truncated.
func findingsOutputSchema() map[string]any {
	return map[string]any{
		"type":        "object",
		"description": "Full report (format \"report\", the default without filter or paging inputs) or summary-first page (format \"summary\", or any filter or paging input).",
		"properties": map[string]any{
			"name":                map[string]any{"type": "string"},
			"profile":             map[string]any{"type": "string"},
			"generated_at":        map[string]any{"type": "string"},
			"sections":            map[string]any{"type": "array"},
			"artifacts":           map[string]any{"type": "array"},
			"summary":             map[string]any{"type": "object"},
			"suppressed_findings": map[string]any{"type": "array"},
			"scan_id":             map[string]any{"type": "string"},
			"findings":            map[string]any{"type": "array"},
			"offset":              map[string]any{"type": "integer"},
			"truncated":           map[string]any{"type": "boolean"},
			"next_cursor":         map[string]any{"type": "string"},
			"filters":             map[string]any{"type": "object"},
		},
		"oneOf": []any{reportShapeSchema(), summaryShapeSchema()},
	}
}

// reportShapeSchema is the full report: the shape scan and validate_patch
// returned in 1.10, and still return when no summary input is passed.
func reportShapeSchema() map[string]any {
	return map[string]any{
		"title":       "report",
		"description": "Full codeguard report: every section with all of its findings and evidence.",
		"type":        "object",
		"properties": map[string]any{
			"name":         map[string]any{"type": "string"},
			"profile":      map[string]any{"type": "string"},
			"generated_at": map[string]any{"type": "string"},
			"sections": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":       map[string]any{"type": "string"},
						"name":     map[string]any{"type": "string"},
						"status":   map[string]any{"type": "string"},
						"findings": map[string]any{"type": "array"},
					},
				},
			},
			"artifacts": map[string]any{"type": "array"},
			"summary": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"passed_sections":     map[string]any{"type": "integer"},
					"warned_sections":     map[string]any{"type": "integer"},
					"failed_sections":     map[string]any{"type": "integer"},
					"total_findings":      map[string]any{"type": "integer"},
					"suppressed_findings": map[string]any{"type": "integer"},
				},
			},
		},
		"required": []string{"sections", "summary"},
	}
}

// summaryShapeSchema is the summary-first, filtered, paginated page.
func summaryShapeSchema() map[string]any {
	return map[string]any{
		"title":       "summary",
		"description": "Summary-first page: per-section counts, filtered-out counts and top rules, then at most max_findings compact findings.",
		"type":        "object",
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
			"artifacts":   map[string]any{"type": "array"},
			"filters":     map[string]any{"type": "object"},
		},
		"required": []string{"summary", "findings", "truncated"},
	}
}
