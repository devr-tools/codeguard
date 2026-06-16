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

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func mcpTools() []mcpTool {
	return []mcpTool{
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
