package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
)

type explainDiscoveryPayload struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Level      string `json:"level"`
	Confidence string `json:"confidence"`
	Why        string `json:"why"`
	HowToFix   string `json:"how_to_fix"`
	SectionKey string `json:"section_key"`
	Examples   []struct {
		Language  string   `json:"language"`
		Path      string   `json:"path"`
		Bad       string   `json:"bad"`
		Good      string   `json:"good"`
		Overrides []string `json:"overrides"`
	} `json:"examples"`
	ConfigKeys []struct {
		Key   string `json:"key"`
		Role  string `json:"role"`
		Value any    `json:"value"`
		Unset bool   `json:"unset"`
	} `json:"config_keys"`
	ConfigSource string `json:"config_source"`
	Enabled      *bool  `json:"enabled"`
	Suppression  struct {
		Inline string `json:"inline"`
		Waiver string `json:"waiver"`
	} `json:"suppression"`
	Profiles []string `json:"profiles"`
	Aliases  []string `json:"aliases"`
	Docs     string   `json:"docs"`
	DocsURL  string   `json:"docs_url"`
}

func runDiscoveryCLI(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := cli.Run(args, strings.NewReader(""), &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

// writeExplainConfig writes a minimal config with quality on and local
// precision off, so explain must report function.* rules as disabled and
// show the configured threshold.
func writeExplainConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "codeguard.yaml")
	body := "name: explain-test\n" +
		"targets:\n  - name: repo\n    path: " + dir + "\n    language: go\n" +
		"checks:\n  quality: true\n  quality_rules:\n    max_parameters: 7\n    local_precision: false\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func decodeExplainDiscovery(t *testing.T, raw string) explainDiscoveryPayload {
	t.Helper()
	var payload explainDiscoveryPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("decode explain json: %v\n%s", err, raw)
	}
	return payload
}

func TestExplainJSONCarriesDiscoveryFields(t *testing.T) {
	stdout, stderr, code := runDiscoveryCLI(t, "explain", "-format", "json", "security.taint.go")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	payload := decodeExplainDiscovery(t, stdout)
	if payload.Title == "" || payload.Level != "fail" || payload.Confidence != "high" || payload.Why == "" || payload.HowToFix == "" {
		t.Fatalf("missing header fields: %+v", payload)
	}
	if len(payload.Examples) == 0 || payload.Examples[0].Language != "go" || !strings.Contains(payload.Examples[0].Bad, "exec.Command") {
		t.Fatalf("expected a go example, got %+v", payload.Examples)
	}
	keys := map[string]string{}
	for _, key := range payload.ConfigKeys {
		keys[key.Key] = key.Role
	}
	if keys["checks.security"] != "enable" || keys["checks.security_rules.taint_go"] != "enable" {
		t.Fatalf("config keys = %+v", payload.ConfigKeys)
	}
	if payload.Suppression.Inline != "// codeguard:ignore security.taint.go -- <reason>" {
		t.Fatalf("inline suppression = %q", payload.Suppression.Inline)
	}
	if !strings.Contains(payload.Suppression.Waiver, "- rule: security.taint.go") || !strings.Contains(payload.Suppression.Waiver, "expires_on:") {
		t.Fatalf("waiver yaml = %q", payload.Suppression.Waiver)
	}
	if !slices.Contains(payload.Profiles, "default") || !slices.Contains(payload.Profiles, "strict") {
		t.Fatalf("profiles = %v", payload.Profiles)
	}
	if payload.Docs != "docs/rule-index.md#rule-security-taint-go" || !strings.HasSuffix(payload.DocsURL, "/docs/rule-index.md#rule-security-taint-go") {
		t.Fatalf("docs = %q, %q", payload.Docs, payload.DocsURL)
	}
	if payload.Aliases == nil {
		t.Fatal("aliases should be an empty list, not null")
	}
}

func TestExplainAgentFormatIsTheJSONPayload(t *testing.T) {
	agent, _, code := runDiscoveryCLI(t, "explain", "-format", "agent", "prompts.mcp-unpinned-server")
	if code != 0 {
		t.Fatalf("agent exit %d", code)
	}
	jsonOut, _, code := runDiscoveryCLI(t, "explain", "-format", "json", "prompts.mcp-unpinned-server")
	if code != 0 || agent != jsonOut {
		t.Fatalf("agent and json formats differ (exit %d)", code)
	}
}

func TestExplainReportsResolvedConfigValues(t *testing.T) {
	configPath := writeExplainConfig(t)
	stdout, stderr, code := runDiscoveryCLI(t, "explain", "-config", configPath, "-format", "json", "function.excessive-parameters")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	payload := decodeExplainDiscovery(t, stdout)
	if payload.ConfigSource != configPath {
		t.Fatalf("config source = %q", payload.ConfigSource)
	}
	values := map[string]any{}
	for _, key := range payload.ConfigKeys {
		values[key.Key] = key.Value
	}
	if values["checks.quality"] != true || values["checks.quality_rules.local_precision"] != false || values["checks.quality_rules.max_parameters"] != float64(7) {
		t.Fatalf("config values = %+v", values)
	}
	if payload.Enabled == nil || *payload.Enabled {
		t.Fatalf("enabled = %v, want false under local_precision=false", payload.Enabled)
	}

	text, _, code := runDiscoveryCLI(t, "explain", "-config", configPath, "function.excessive-parameters")
	if code != 0 {
		t.Fatalf("text exit %d", code)
	}
	for _, want := range []string{
		"config (values from " + configPath + "):",
		"checks.quality_rules.max_parameters   tune = 7",
		"enabled under this config: no",
		"enable for one run: codeguard scan -set checks.quality_rules.local_precision=true",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("text output missing %q:\n%s", want, text)
		}
	}
}

func TestExplainTextShowsExamplesSuppressionAndProfiles(t *testing.T) {
	stdout, stderr, code := runDiscoveryCLI(t, "explain", "quality.max-file-lines")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, want := range []string{
		"quality.max-file-lines\ntitle: ",
		"confidence: medium",
		"example (go, internal/orders/status.go):\n  bad:\n",
		"  good:\n",
		"reproduce with: codeguard scan -set checks.quality_rules.max_file_lines=6",
		"checks.quality_rules.max_file_lines",
		"inline (same or previous line): // codeguard:ignore quality.max-file-lines -- <reason>",
		"waiver (codeguard.yaml):\n    waivers:\n      - rule: quality.max-file-lines",
		"profiles that enable it: default, ai-safe, enterprise, startup, strict",
		"docs: docs/rule-index.md#rule-quality-max-file-lines",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("explain output missing %q:\n%s", want, stdout)
		}
	}
}

func TestExplainUsesCommentSyntaxOfTheExample(t *testing.T) {
	cases := map[string]string{
		"ci.workflow-script-injection":  "# codeguard:ignore ci.workflow-script-injection -- <reason>",
		"prompts.secret-interpolation":  "<!-- codeguard:ignore prompts.secret-interpolation -- <reason> -->",
		"security.taint.python":         "# codeguard:ignore security.taint.python -- <reason>",
		"quality.cyclomatic-complexity": "// codeguard:ignore quality.cyclomatic-complexity -- <reason>",
	}
	for ruleID, want := range cases {
		stdout, _, code := runDiscoveryCLI(t, "explain", "-format", "json", ruleID)
		if code != 0 {
			t.Fatalf("%s exit %d", ruleID, code)
		}
		if got := decodeExplainDiscovery(t, stdout).Suppression.Inline; got != want {
			t.Errorf("%s inline = %q, want %q", ruleID, got, want)
		}
	}
}

func TestExplainShowsHiddenUnicodeVisibly(t *testing.T) {
	stdout, _, code := runDiscoveryCLI(t, "explain", "prompts.hidden-unicode")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(stdout, "<U+202E>") || strings.ContainsRune(stdout, '\u202e') {
		t.Fatalf("hidden character should render as <U+202E>:\n%s", stdout)
	}
}

func TestExplainAliasListsRetiredID(t *testing.T) {
	stdout, _, code := runDiscoveryCLI(t, "explain", "-format", "json", "quality.mixed-abstraction-levels")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	payload := decodeExplainDiscovery(t, stdout)
	if payload.ID != "function.mixed-abstraction-level" || !slices.Contains(payload.Aliases, "quality.mixed-abstraction-levels") {
		t.Fatalf("alias explain = id %q aliases %v", payload.ID, payload.Aliases)
	}
}

func TestExplainRejectsUnknownFormat(t *testing.T) {
	_, stderr, code := runDiscoveryCLI(t, "explain", "-format", "yaml", "security.taint.go")
	if code == 0 || !strings.Contains(stderr, "invalid explain format") {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
}

func TestMCPExplainToolAndResourceMatchCLIJSON(t *testing.T) {
	configPath := writeExplainConfig(t)
	const ruleID = "function.excessive-parameters"
	lines := runMCPServer(t, configPath, joinMCPMessages(t,
		initializeMessage(1, "2025-06-18"),
		map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"},
		map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{
			"name": "explain", "arguments": map[string]any{"rule_id": ruleID},
		}},
		map[string]any{"jsonrpc": "2.0", "id": 3, "method": "resources/read", "params": map[string]any{
			"uri": "codeguard://rules/" + ruleID,
		}},
	))

	var tool struct {
		Result struct {
			StructuredContent json.RawMessage `json:"structuredContent"`
		} `json:"result"`
	}
	decodeMCPLine(t, findResponseLineByID(t, lines, "2"), &tool)
	var resource struct {
		Result struct {
			Contents []struct {
				Text string `json:"text"`
			} `json:"contents"`
		} `json:"result"`
	}
	decodeMCPLine(t, findResponseLineByID(t, lines, "3"), &resource)
	if len(resource.Result.Contents) != 1 {
		t.Fatalf("resource contents = %+v", resource)
	}
	cliOut, stderr, code := runDiscoveryCLI(t, "explain", "-config", configPath, "-format", "json", ruleID)
	if code != 0 {
		t.Fatalf("cli exit %d: %s", code, stderr)
	}

	want := canonicalJSON(t, cliOut)
	if got := canonicalJSON(t, string(tool.Result.StructuredContent)); got != want {
		t.Fatalf("MCP explain tool differs from CLI json:\n tool %s\n cli  %s", got, want)
	}
	if got := canonicalJSON(t, resource.Result.Contents[0].Text); got != want {
		t.Fatalf("MCP rule resource differs from CLI json:\n resource %s\n cli      %s", got, want)
	}
}

func canonicalJSON(t *testing.T, raw string) string {
	t.Helper()
	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		t.Fatalf("decode json: %v\n%s", err, raw)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encode json: %v", err)
	}
	return string(encoded)
}
