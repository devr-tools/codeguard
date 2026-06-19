package cli

func mcpTools() []mcpTool {
	tools := []mcpTool{
		scanTool(),
		validateConfigTool(),
		validatePatchTool(),
		explainTool(),
		listRulesTool(),
		verifyFixTool(),
		proposeFixTool(),
		applyFixTool(),
	}
	for i := range tools {
		if tools[i].Annotations == nil {
			tools[i].Annotations = readOnlyToolAnnotations(tools[i].Title)
		}
	}
	return tools
}

func scanTool() mcpTool {
	return mcpTool{
		Name:        "scan",
		Title:       "Scan Repository",
		Description: "Run codeguard against the configured repository targets and return a structured report.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"config_path": map[string]any{"type": "string"},
				"profile":     map[string]any{"type": "string"},
				"mode":        map[string]any{"type": "string", "enum": []string{"full", "diff"}},
				"base_ref":    map[string]any{"type": "string"},
			},
		},
		OutputSchema: reportOutputSchema(),
	}
}

func validateConfigTool() mcpTool {
	return mcpTool{
		Name:        "validate_config",
		Title:       "Validate Config",
		Description: "Validate the configured codeguard policy file and return a machine-readable result.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"config_path": map[string]any{"type": "string"},
				"profile":     map[string]any{"type": "string"},
			},
		},
		OutputSchema: objectOutputSchema(),
	}
}

func validatePatchTool() mcpTool {
	return mcpTool{
		Name:        "validate_patch",
		Title:       "Validate Patch",
		Description: "Evaluate a unified diff against policy without mutating the working tree and return a structured report.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"config_path": map[string]any{"type": "string"},
				"profile":     map[string]any{"type": "string"},
				"diff":        map[string]any{"type": "string"},
			},
			"required": []string{"diff"},
		},
		OutputSchema: reportOutputSchema(),
	}
}

func explainTool() mcpTool {
	return mcpTool{
		Name:        "explain",
		Title:       "Explain Rule",
		Description: "Return machine-first explanation metadata for a codeguard rule.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"config_path": map[string]any{"type": "string"},
				"profile":     map[string]any{"type": "string"},
				"rule_id":     map[string]any{"type": "string"},
			},
			"required": []string{"rule_id"},
		},
		OutputSchema: objectOutputSchema(),
	}
}

func listRulesTool() mcpTool {
	return mcpTool{
		Name:        "list_rules",
		Title:       "List Rules",
		Description: "Return the rule catalog that applies to the current or requested configuration.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"config_path": map[string]any{"type": "string"},
				"profile":     map[string]any{"type": "string"},
			},
		},
		OutputSchema: objectOutputSchema(),
	}
}

func verifyFixTool() mcpTool {
	return buildFixTool(
		"verify_fix",
		"Verify Fix",
		"Verify a candidate unified diff against a finding: apply it in an isolated workspace, re-scan the changed lines, run the nearest inferred tests, and return the result only if it passes. Does not modify the working tree.",
		true,
		false,
	)
}

func proposeFixTool() mcpTool {
	return buildFixTool(
		"propose_fix",
		"Propose Fix",
		"Generate a candidate fix for a finding (via the client's LLM when MCP sampling is supported, else a configured AI provider), verify it in an isolated workspace with re-scan and nearest tests, and return it only if it passes. Does not modify the working tree.",
		false,
		false,
	)
}

func applyFixTool() mcpTool {
	tool := buildFixTool(
		"apply_fix",
		"Apply Fix",
		"Verify a candidate unified diff and, only if it passes, write it to the working tree. Asks the user to confirm first when the client supports elicitation. This tool modifies files on disk.",
		true,
		true,
	)
	tool.Annotations = writeToolAnnotations("Apply Fix")
	return tool
}

func buildFixTool(name string, title string, description string, requireDiff bool, destructive bool) mcpTool {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"config_path":       map[string]any{"type": "string"},
			"profile":           map[string]any{"type": "string"},
			"finding":           fixFindingSchema(),
			"diff":              map[string]any{"type": "string", "description": "Candidate unified diff to verify and apply."},
			"base_ref":          map[string]any{"type": "string"},
			"max_nearest_tests": map[string]any{"type": "integer"},
			"test_commands":     map[string]any{"type": "array"},
		},
	}
	if requireDiff {
		schema["required"] = []string{"diff"}
	} else {
		schema["required"] = []string{"finding"}
	}
	tool := mcpTool{
		Name:         name,
		Title:        title,
		Description:  description,
		InputSchema:  schema,
		OutputSchema: objectOutputSchema(),
	}
	if destructive {
		tool.Annotations = writeToolAnnotations(title)
	}
	return tool
}
