package cli_test

import "testing"

type mcpCompatCase struct {
	name      string
	messages  []map[string]any
	assertion func(*testing.T, []string)
}

func TestServeMCPCompatibilityMatrix(t *testing.T) {
	configPath := writeMCPConfig(t, `{
  "name": "mcp-compat-test",
  "targets": [{"name": "repo", "path": "`+t.TempDir()+`", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": false, "ci": false},
  "output": {"format": "json"}
}`)

	cases := mcpCompatibilityCases(configPath)
	if len(cases) == 0 {
		t.Fatal("expected compatibility cases")
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines := runMCPServer(t, configPath, joinMCPMessages(t, tc.messages...))
			tc.assertion(t, lines)
		})
	}
}

func mcpCompatibilityCases(configPath string) []mcpCompatCase {
	cases := compatibilityCoreCases(configPath)
	cases = append(cases, compatibilityValidationCases(configPath)...)
	return append(cases, compatibilityNotificationCases()...)
}

func compatibilityCoreCases(configPath string) []mcpCompatCase {
	_ = configPath
	return []mcpCompatCase{
		{
			name: "supports current protocol version and string ids",
			messages: []map[string]any{
				initializeMessage("init-1", "2025-11-25"),
				map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"},
				map[string]any{"jsonrpc": "2.0", "id": "ping-1", "method": "ping"},
			},
			assertion: assertCurrentProtocolCompatibility,
		},
		{
			name: "rejects tools list before initialize",
			messages: []map[string]any{
				{"jsonrpc": "2.0", "id": 1, "method": "tools/list"},
			},
			assertion: assertPreInitializeError,
		},
		{
			name: "unknown protocol version falls back to compat version",
			messages: []map[string]any{
				{
					"jsonrpc": "2.0",
					"id":      1,
					"method":  "initialize",
					"params": map[string]any{
						"protocolVersion": "2099-01-01",
						"capabilities":    map[string]any{"experimental": map[string]any{"host": true}},
						"clientInfo":      map[string]any{"name": "compat-client", "version": "1.0.0", "extra": "ignored"},
					},
				},
				map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"},
				map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"},
			},
			assertion: assertFallbackProtocolCompatibility,
		},
	}
}

func compatibilityValidationCases(configPath string) []mcpCompatCase {
	return []mcpCompatCase{
		{
			name: "validate config tool succeeds",
			messages: []map[string]any{
				initializeMessage(1, "2025-06-18"),
				map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"},
				{
					"jsonrpc": "2.0",
					"id":      2,
					"method":  "tools/call",
					"params":  map[string]any{"name": "validate_config", "arguments": map[string]any{"config_path": configPath}},
				},
			},
			assertion: assertValidateConfigCompatibility,
		},
		{
			name: "list rules tool returns catalog",
			messages: []map[string]any{
				initializeMessage(1, "2025-06-18"),
				map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"},
				{
					"jsonrpc": "2.0",
					"id":      2,
					"method":  "tools/call",
					"params":  map[string]any{"name": "list_rules", "arguments": map[string]any{}},
				},
			},
			assertion: assertListRulesCompatibility,
		},
		{
			name: "empty arguments are accepted for argument-light tools",
			messages: []map[string]any{
				initializeMessage(1, "2025-06-18"),
				map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"},
				map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{"name": "list_rules"}},
				{
					"jsonrpc": "2.0",
					"id":      3,
					"method":  "tools/call",
					"params":  map[string]any{"name": "validate_config", "arguments": map[string]any{"config_path": configPath}},
				},
			},
			assertion: assertEmptyArgumentCompatibility,
		},
		{
			name: "unknown tool becomes tool error result",
			messages: []map[string]any{
				initializeMessage(1, "2025-06-18"),
				map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"},
				{
					"jsonrpc": "2.0",
					"id":      2,
					"method":  "tools/call",
					"params":  map[string]any{"name": "missing_tool", "arguments": map[string]any{}},
				},
			},
			assertion: assertUnknownToolCompatibility,
		},
	}
}

func compatibilityNotificationCases() []mcpCompatCase {
	return []mcpCompatCase{
		{
			name: "ping and unknown notifications produce no response",
			messages: []map[string]any{
				initializeMessage(1, "2025-06-18"),
				map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"},
				map[string]any{"jsonrpc": "2.0", "method": "ping"},
				map[string]any{"jsonrpc": "2.0", "method": "notifications/unknown"},
				map[string]any{"jsonrpc": "2.0", "id": 2, "method": "ping"},
			},
			assertion: assertNotificationCompatibility,
		},
	}
}
