package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSuggestionConfig(t *testing.T, extra string) string {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "suggestions",
  "targets": [{"name": "repo", "path": ".", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": false, "ci": false, "performance": false` + extra + `},
  "output": {"format": "text"}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func TestErrorSuggestions(t *testing.T) {
	configPath := writeSuggestionConfig(t, "")
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "explain partial rule id lists matching rules",
			args: []string{"explain", "taint"},
			want: []string{`unknown rule "taint"`, "did you mean one of", `"security.taint.go"`, "codeguard rules"},
		},
		{
			name: "explain typo suggests the rule",
			args: []string{"explain", "security.taint.goo"},
			want: []string{`did you mean "security.taint.go"?`},
		},
		{
			name: "scan profile typo",
			args: []string{"scan", "-config", configPath, "-profile", "strct"},
			want: []string{`unknown profile "strct"`, `did you mean "strict"?`, "valid profiles:"},
		},
		{
			name: "validate profile typo",
			args: []string{"validate", "-config", configPath, "-profile", "entreprise"},
			want: []string{`did you mean "enterprise"?`},
		},
		{
			name: "scan set key typo",
			args: []string{"scan", "-config", configPath, "-set", "checks.qualty=true"},
			want: []string{`unknown config field "qualty"`, `did you mean "checks.quality"?`},
		},
		{
			name: "set typo keeps the rest of the path",
			args: []string{"validate", "-config", configPath, "-set", "checks.qualty_rules.max_file_lines=3"},
			want: []string{`did you mean "checks.quality_rules.max_file_lines"?`},
		},
		{
			name: "scan format typo",
			args: []string{"scan", "-config", configPath, "-format", "sarf"},
			want: []string{`unknown output format "sarf"`, `did you mean "sarif"?`, "valid formats:"},
		},
		{
			name: "validate-patch format typo",
			args: []string{"validate-patch", "-config", configPath, "-format", "jsno"},
			want: []string{`did you mean "json"?`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(t, tc.args...)
			if code != 1 {
				t.Fatalf("exit = %d, want 1; stdout=%s stderr=%s", code, stdout, stderr)
			}
			for _, want := range tc.want {
				if !strings.Contains(stderr, want) {
					t.Fatalf("stderr missing %q:\n%s", want, stderr)
				}
			}
		})
	}
}

func TestErrorSuggestionsStayQuietWhenNothingIsClose(t *testing.T) {
	configPath := writeSuggestionConfig(t, "")
	for _, args := range [][]string{
		{"explain", "xyzzy"},
		{"scan", "-config", configPath, "-format", "pdf"},
		{"scan", "-config", configPath, "-set", "checks.zzzzzzzz=true"},
	} {
		code, _, stderr := runCLI(t, args...)
		if code != 1 {
			t.Fatalf("%v: exit = %d, want 1", args, code)
		}
		if strings.Contains(stderr, "did you mean") {
			t.Fatalf("%v: unexpected suggestion for an unrelated value:\n%s", args, stderr)
		}
	}
}

func TestConfigSelectionSuggestions(t *testing.T) {
	cases := []struct {
		name  string
		extra string
		want  string
	}{
		{"disabled section typo", `, "disabled": ["securty"]`, `did you mean "security"?`},
		{"min confidence section typo", `, "min_confidence": {"sections": {"qualty": "high"}}`, `did you mean "quality"?`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configPath := writeSuggestionConfig(t, tc.extra)
			code, _, stderr := runCLI(t, "validate", "-config", configPath)
			if code != 1 {
				t.Fatalf("exit = %d, want 1; stderr=%s", code, stderr)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Fatalf("stderr missing %q:\n%s", tc.want, stderr)
			}
		})
	}
}

func TestDoctorSuggestsRuleForMistypedWaiver(t *testing.T) {
	dir := t.TempDir()
	configPath := writeRuleHealthConfig(t, dir, `{"rule": "quality.gofmtt"}`, filepath.Join(dir, "cache", "scan.json"))
	out := runRuleHealthDoctor(t, configPath)
	if !strings.Contains(out, `did you mean "quality.gofmt"?`) {
		t.Fatalf("expected waiver rule suggestion, got:\n%s", out)
	}
}

// A bad -format must fail before the scan starts: no progress line on stderr
// and nothing on stdout, so CI logs show only the error.
func TestScanRejectsBadFormatBeforeAnyOutput(t *testing.T) {
	configPath := writeSuggestionConfig(t, "")
	for _, args := range [][]string{
		{"scan", "-config", configPath, "-format", "sarf"},
		{"scan", "-config", configPath, "-format", "sarf", "-quiet", "-fail-on", "warn"},
		{"scan", "-config", configPath, "-profile", "strct"},
	} {
		code, stdout, stderr := runCLI(t, args...)
		if code != 1 {
			t.Fatalf("%v: exit = %d, want 1", args, code)
		}
		if stdout != "" {
			t.Fatalf("%v: expected no stdout, got:\n%s", args, stdout)
		}
		if strings.Contains(stderr, "scan started") || strings.Contains(stderr, "completed") {
			t.Fatalf("%v: scan progress printed before validation failed:\n%s", args, stderr)
		}
		if lines := strings.Split(strings.TrimSpace(stderr), "\n"); len(lines) != 1 {
			t.Fatalf("%v: expected a single error line, got:\n%s", args, stderr)
		}
	}
}
