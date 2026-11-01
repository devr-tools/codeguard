package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func ruleLines(t *testing.T, args ...string) []string {
	t.Helper()
	stdout, stderr, code := runDiscoveryCLI(t, append([]string{"rules"}, args...)...)
	if code != 0 {
		t.Fatalf("rules %v exit %d: %s", args, code, stderr)
	}
	return nonEmptyLines(stdout)
}

func TestRulesDefaultOutputIsUnfiltered(t *testing.T) {
	lines := ruleLines(t)
	if len(lines) != len(service.Rules()) {
		t.Fatalf("rules printed %d lines, catalog has %d rules", len(lines), len(service.Rules()))
	}
	for _, line := range lines {
		if fields := strings.Split(line, "\t"); len(fields) < 6 {
			t.Fatalf("line %q lost its tab-separated columns", line)
		}
	}
}

func TestRulesSectionFilter(t *testing.T) {
	for _, section := range []string{"prompts", "AI Prompts", "supply-chain", "supply_chain"} {
		lines := ruleLines(t, "-section", section)
		if len(lines) == 0 {
			t.Fatalf("-section %q matched nothing", section)
		}
		for _, line := range lines {
			id := strings.Split(line, "\t")[0]
			if section == "prompts" || section == "AI Prompts" {
				if !strings.HasPrefix(id, "prompts.") {
					t.Fatalf("-section %q returned %s", section, id)
				}
			} else if !strings.HasPrefix(id, "supply_chain.") {
				t.Fatalf("-section %q returned %s", section, id)
			}
		}
	}
	_, stderr, code := runDiscoveryCLI(t, "rules", "-section", "securty")
	if code == 0 || !strings.Contains(stderr, `unknown -section "securty"`) || !strings.Contains(stderr, "security") {
		t.Fatalf("unknown section: exit %d stderr %q", code, stderr)
	}
}

func TestRulesSearchMatchesIDTitleAndWhy(t *testing.T) {
	byID := ruleLines(t, "-search", "TAINT")
	if len(byID) == 0 {
		t.Fatal("-search TAINT matched nothing")
	}
	for _, line := range byID {
		lowered := strings.ToLower(line)
		id := strings.Split(line, "\t")[0]
		meta, _ := service.ExplainRule(id)
		if !strings.Contains(lowered, "taint") && !strings.Contains(strings.ToLower(meta.Description), "taint") {
			t.Fatalf("-search TAINT returned unrelated %s", id)
		}
	}
	// "source-to-sink chain" only appears in taint rule descriptions (the why).
	byWhy := ruleLines(t, "-search", "source-to-sink chain")
	if len(byWhy) == 0 {
		t.Fatal("-search should match the description text")
	}
}

func TestRulesLevelFilter(t *testing.T) {
	for _, level := range []string{"fail", "warn"} {
		for _, line := range ruleLines(t, "-level", level) {
			if got := strings.Split(line, "\t")[1]; got != level {
				t.Fatalf("-level %s returned a %s rule: %s", level, got, line)
			}
		}
	}
	_, stderr, code := runDiscoveryCLI(t, "rules", "-level", "error")
	if code == 0 || !strings.Contains(stderr, "invalid -level") {
		t.Fatalf("bad level: exit %d stderr %q", code, stderr)
	}
}

func TestRulesEnabledFilterUsesResolvedConfig(t *testing.T) {
	configPath := writeExplainConfig(t)
	lines := ruleLines(t, "-config", configPath, "-enabled")
	ids := map[string]bool{}
	for _, line := range lines {
		ids[strings.Split(line, "\t")[0]] = true
	}
	if !ids["quality.max-file-lines"] {
		t.Fatal("quality.max-file-lines should be enabled with checks.quality=true")
	}
	if ids["function.excessive-parameters"] {
		t.Fatal("function.excessive-parameters should be disabled by local_precision=false")
	}
	if ids["security.taint.go"] {
		t.Fatal("security rules should be disabled when checks.security is off")
	}

	if got := ruleLines(t, "-config", configPath, "-enabled", "-section", "reliability"); len(got) != 0 {
		t.Fatalf("reliability is off by default, got %v", got)
	}
	strictPath := filepath.Join(t.TempDir(), "codeguard.yaml")
	strictBody := "name: strict\nprofile: strict\ntargets:\n  - name: repo\n    path: " + filepath.Dir(strictPath) + "\n    language: go\n"
	if err := os.WriteFile(strictPath, []byte(strictBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if got := ruleLines(t, "-config", strictPath, "-enabled", "-section", "reliability"); len(got) == 0 {
		t.Fatal("the strict profile enables reliability rules")
	}
}

func TestRulesJSONFormat(t *testing.T) {
	stdout, stderr, code := runDiscoveryCLI(t, "rules", "-format", "json", "-section", "ci", "-search", "workflow", "-level", "fail")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	var payload struct {
		Rules []struct {
			ID          string `json:"id"`
			SectionKey  string `json:"section_key"`
			Level       string `json:"level"`
			HasExamples bool   `json:"has_examples"`
			Enabled     *bool  `json:"enabled"`
		} `json:"rules"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("decode: %v\n%s", err, stdout)
	}
	if len(payload.Rules) == 0 {
		t.Fatal("no rules in json output")
	}
	for _, rule := range payload.Rules {
		meta, _ := service.ExplainRule(rule.ID)
		matchesSearch := strings.Contains(strings.ToLower(rule.ID+meta.Title+meta.Description), "workflow")
		if rule.SectionKey != "ci" || rule.Level != "fail" || !matchesSearch {
			t.Fatalf("filters not applied to json output: %+v", rule)
		}
		if rule.Enabled != nil {
			t.Fatalf("enabled should be omitted without a resolved config: %+v", rule)
		}
	}

	configPath := writeExplainConfig(t)
	stdout, _, code = runDiscoveryCLI(t, "rules", "-format", "json", "-config", configPath, "-section", "quality", "-search", "excessive-parameters")
	if code != 0 || !strings.Contains(stdout, `"enabled": false`) {
		t.Fatalf("json with a config should report enabled: exit %d\n%s", code, stdout)
	}
}
