package cli_test

import "testing"

func TestServeMCPListsAndCallsTools(t *testing.T) {
	configPath, promptPath, diff := writePromptMCPFixture(t)
	lines := runMCPServer(t, configPath, joinMCPListAndCallMessages(t, configPath, diff))

	if len(lines) != 4 {
		t.Fatalf("expected 4 MCP responses, got %d: %q", len(lines), lines)
	}
	assertInitializeLine(t, findResponseLineByID(t, lines, "1"), "2025-06-18", "codeguard")
	assertToolCatalogLine(t, findResponseLineByID(t, lines, "2"), "scan", "validate_patch", "explain", "list_rules", "validate_config")
	assertExplainLine(t, findResponseLineByID(t, lines, "3"), "security.hardcoded-secret", "language-agnostic")
	assertValidatePatchLine(t, findResponseLineByID(t, lines, "4"))
	assertMCPPromptFileUnchanged(t, promptPath)
}

func TestServeMCPEmitsProgressNotifications(t *testing.T) {
	configPath := writeMCPConfig(t, `{
  "name": "mcp-progress-test",
  "targets": [{"name": "repo", "path": "`+t.TempDir()+`", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": false, "ci": false},
  "output": {"format": "json"}
}`)

	lines := runMCPServer(t, configPath, joinMCPMessages(t,
		initializeMessage(1, "2025-06-18"),
		map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"},
		map[string]any{
			"jsonrpc": "2.0",
			"id":      2,
			"method":  "tools/call",
			"params": map[string]any{
				"_meta": map[string]any{"progressToken": "tok-1"},
				"name":  "list_rules",
			},
		},
	))

	if len(lines) != 4 {
		t.Fatalf("expected 4 MCP messages, got %d: %q", len(lines), lines)
	}
	assertProgressValues(t, lines, "tok-1", []float64{0, 1})
}

func TestServeMCPCancellationSuppressesResponse(t *testing.T) {
	configPath := writeCancelableMCPFixture(t, "mcp-cancel-test")
	lines := runMCPServer(t, configPath, joinCancellationMessages(t, configPath))

	if len(lines) < 3 {
		t.Fatalf("expected initialize and progress output, got %d: %q", len(lines), lines)
	}
	assertCancellationBehavior(t, lines, 2, "cancel-tok")
}
