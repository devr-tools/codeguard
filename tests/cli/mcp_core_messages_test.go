package cli_test

import "testing"

func joinMCPListAndCallMessages(t *testing.T, configPath string, diff string) string {
	t.Helper()
	return joinMCPMessages(t,
		initializeMessage(1, "2025-06-18"),
		map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"},
		map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/list"},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      3,
			"method":  "tools/call",
			"params": map[string]any{
				"name":      "explain",
				"arguments": map[string]any{"rule_id": "security.hardcoded-secret"},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      4,
			"method":  "tools/call",
			"params": map[string]any{
				"name": "validate_patch",
				"arguments": map[string]any{
					"config_path": configPath,
					"diff":        diff,
				},
			},
		},
	)
}

func initializeMessage(id any, version string) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": version,
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test-client", "version": "1.0.0"},
		},
	}
}

func joinCancellationMessages(t *testing.T, configPath string) string {
	t.Helper()
	return joinMCPMessages(t,
		initializeMessage(1, "2025-06-18"),
		map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params": map[string]any{
				"_meta": map[string]any{"progressToken": "cancel-tok"},
				"name":  "scan",
				"arguments": map[string]any{
					"config_path": configPath,
					"mode":        "full",
				},
			},
		},
		map[string]any{
			"jsonrpc": "2.0",
			"method":  "notifications/cancelled",
			"params":  map[string]any{"requestId": 2, "reason": "test cancellation"},
		},
	)
}
