package cli

import (
	"encoding/json"
	"strings"
)

func toolSuccessResult(payload any) map[string]any {
	text := mustJSON(payload)
	return map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": text,
		}},
		"structuredContent": payload,
		"isError":           false,
	}
}

func toolErrorResult(message string) map[string]any {
	return map[string]any{
		"content": []map[string]any{{
			"type": "text",
			"text": message,
		}},
		"isError": true,
	}
}

// toolErrorResultData is toolErrorResult with machine-readable structuredContent
// so callers can act on the failure (e.g. a fix that did not verify carries the
// attempted diff and remaining findings).
func toolErrorResultData(message string, data any) map[string]any {
	result := toolErrorResult(message)
	if data != nil {
		result["structuredContent"] = data
	}
	return result
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func mcpTools() []mcpTool {
	tools := []mcpTool{
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
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
		},
		{
			Name:        "verify_fix",
			Title:       "Verify Fix",
			Description: "Verify a candidate unified diff against a finding: apply it in an isolated workspace, re-scan the changed lines, run the nearest inferred tests, and return the result only if it passes. Does not modify the working tree.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"config_path":       map[string]any{"type": "string"},
					"profile":           map[string]any{"type": "string"},
					"finding":           fixFindingSchema(),
					"diff":              map[string]any{"type": "string", "description": "Candidate unified diff to verify."},
					"base_ref":          map[string]any{"type": "string"},
					"max_nearest_tests": map[string]any{"type": "integer"},
					"test_commands":     map[string]any{"type": "array"},
				},
				"required": []string{"diff"},
			},
			OutputSchema: objectOutputSchema(),
		},
		{
			Name:        "propose_fix",
			Title:       "Propose Fix",
			Description: "Generate a candidate fix for a finding (via the client's LLM when MCP sampling is supported, else a configured AI provider), verify it in an isolated workspace with re-scan and nearest tests, and return it only if it passes. Does not modify the working tree.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"config_path":       map[string]any{"type": "string"},
					"profile":           map[string]any{"type": "string"},
					"finding":           fixFindingSchema(),
					"base_ref":          map[string]any{"type": "string"},
					"max_nearest_tests": map[string]any{"type": "integer"},
					"test_commands":     map[string]any{"type": "array"},
				},
				"required": []string{"finding"},
			},
			OutputSchema: objectOutputSchema(),
		},
		{
			Name:        "apply_fix",
			Title:       "Apply Fix",
			Description: "Verify a candidate unified diff and, only if it passes, write it to the working tree. Asks the user to confirm first when the client supports elicitation. This tool modifies files on disk.",
			InputSchema: map[string]any{
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
				"required": []string{"diff"},
			},
			OutputSchema: objectOutputSchema(),
			Annotations:  writeToolAnnotations("Apply Fix"),
		},
	}

	// Most codeguard tools only read the repository; tools that already declared
	// annotations (e.g. the destructive apply_fix) keep them.
	for i := range tools {
		if tools[i].Annotations == nil {
			tools[i].Annotations = readOnlyToolAnnotations(tools[i].Title)
		}
	}
	return tools
}

// writeToolAnnotations marks a tool that mutates the working tree.
func writeToolAnnotations(title string) map[string]any {
	return map[string]any{
		"title":           title,
		"readOnlyHint":    false,
		"destructiveHint": true,
		"idempotentHint":  false,
		"openWorldHint":   false,
	}
}

// fixFindingSchema describes the finding object accepted by the fix tools.
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

// readOnlyToolAnnotations returns MCP tool annotations marking a tool as
// read-only and non-destructive.
func readOnlyToolAnnotations(title string) map[string]any {
	return map[string]any{
		"title":           title,
		"readOnlyHint":    true,
		"destructiveHint": false,
		"idempotentHint":  true,
		"openWorldHint":   false,
	}
}

// objectOutputSchema is a permissive output schema for tools that return an
// arbitrary JSON object in structuredContent.
func objectOutputSchema() map[string]any {
	return map[string]any{"type": "object"}
}

// reportOutputSchema describes the codeguard report object returned by scan and
// validate_patch in structuredContent.
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

func negotiateMCPProtocolVersion(raw json.RawMessage) string {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return mcpProtocolVersionCompat
	}
	switch strings.TrimSpace(params.ProtocolVersion) {
	case mcpProtocolVersionCurrent, mcpProtocolVersionCompat:
		return params.ProtocolVersion
	default:
		return mcpProtocolVersionCompat
	}
}

func normalizeMCPArguments(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		return json.RawMessage([]byte("{}"))
	}
	return raw
}
