package rules_test

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/rules"
)

// exampleTargetLanguages maps a snippet language onto the target language the
// scan config declares. Repository-wide snippets (workflows, agent settings,
// prompt docs) need no source language, so they ride on a go target.
var exampleTargetLanguages = map[string]string{
	"go":         "go",
	"python":     "python",
	"typescript": "typescript",
	"javascript": "javascript",
	"cpp":        "cpp",
}

// exampleScan is one finding-relevant slice of a scan: the findings for the
// rule under test and the section id each was reported in.
type exampleScan struct {
	findings []core.Finding
	sections []string
}

// scanExample writes snippet at example.Path in a fresh repository, scans it
// through the real `codeguard scan` command with only the rule's section
// enabled plus the example's settings and extra overrides, and returns the
// findings for ruleID.
func scanExample(t *testing.T, meta core.RuleMetadata, example core.RuleExample, snippet string, extra ...string) exampleScan {
	t.Helper()
	repo := t.TempDir()
	path := filepath.Join(repo, filepath.FromSlash(example.Path))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(snippet), 0o600); err != nil {
		t.Fatalf("write snippet: %v", err)
	}
	language := exampleTargetLanguages[example.Language]
	if language == "" {
		language = "go"
	}
	if language == "go" && example.Language == "go" {
		if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/app\n\ngo 1.22\n"), 0o600); err != nil {
			t.Fatalf("write go.mod: %v", err)
		}
	}
	configPath := filepath.Join(t.TempDir(), "codeguard.yaml")
	config := "name: rule-example\n" +
		"targets:\n  - name: repo\n    path: " + strconvQuote(repo) + "\n    language: " + language + "\n" +
		"cache:\n  enabled: false\n" +
		"checks:\n  security_rules:\n    govulncheck_mode: \"off\"\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	args := []string{"scan", "-config", configPath, "-format", "json", "-quiet"}
	toggle := rules.SectionToggleKey(rules.SectionKey(meta))
	if toggle == "" {
		t.Fatalf("%s has no section toggle", meta.ID)
	}
	args = append(args, "-set", toggle+"=true")
	for _, setting := range append(append([]string(nil), example.Overrides...), extra...) {
		args = append(args, "-set", setting)
	}

	var stdout, stderr bytes.Buffer
	code := cli.Run(args, strings.NewReader(""), &stdout, &stderr)
	if code != 0 && code != 1 {
		t.Fatalf("scan exit %d: %s", code, stderr.String())
	}
	var report core.Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode report: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	var scan exampleScan
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID == meta.ID {
				scan.findings = append(scan.findings, finding)
				scan.sections = append(scan.sections, section.ID)
			}
		}
	}
	return scan
}

func strconvQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func rulesWithExamples() []core.RuleMetadata {
	out := make([]core.RuleMetadata, 0)
	for _, meta := range rules.Catalog() {
		if len(meta.Examples) > 0 {
			out = append(out, meta)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// TestRuleExamplesAreVerified is the guarantee behind every example `codeguard
// explain` prints: the bad snippet, scanned on its own, makes the rule fire in
// the section the catalog says it belongs to; the good snippet does not; and
// switching off any rule-specific enable key silences the bad snippet, which
// proves the key really controls the rule.
func TestRuleExamplesAreVerified(t *testing.T) {
	for _, meta := range rulesWithExamples() {
		t.Run(meta.ID, func(t *testing.T) {
			t.Parallel()
			if semanticRuntimeRules[meta.ID] && !typeScriptSemanticRuntimeAvailable() {
				t.Skip("TypeScript semantic runtime (node + typescript.js) not available; " + meta.ID + " is only reported by it")
			}
			for idx, example := range meta.Examples {
				assertExampleShape(t, meta.ID, idx, example)
				bad := scanExample(t, meta, example, example.Bad)
				if len(bad.findings) == 0 {
					t.Fatalf("example %d: bad snippet did not trigger %s", idx, meta.ID)
				}
				for _, section := range bad.sections {
					if section != rules.SectionKey(meta) {
						t.Fatalf("example %d: %s reported in section %q, catalog maps it to %q", idx, meta.ID, section, rules.SectionKey(meta))
					}
				}
				if meta.Confidence != "" {
					for _, finding := range bad.findings {
						if got := normalizedConfidence(finding.Confidence); got != meta.Confidence {
							t.Fatalf("example %d: finding confidence %q, catalog says %q", idx, got, meta.Confidence)
						}
					}
				}
				if good := scanExample(t, meta, example, example.Good); len(good.findings) != 0 {
					t.Fatalf("example %d: good snippet triggered %s at line %d: %s", idx, meta.ID, good.findings[0].Line, good.findings[0].Message)
				}
				assertEnableKeysSilenceExample(t, meta, idx, example)
			}
		})
	}
}

func assertEnableKeysSilenceExample(t *testing.T, meta core.RuleMetadata, idx int, example core.RuleExample) {
	t.Helper()
	toggle := rules.SectionToggleKey(rules.SectionKey(meta))
	for _, key := range meta.ConfigKeys {
		if key.Role != core.RuleConfigKeyRoleEnable || key.Key == toggle {
			continue
		}
		if off := scanExample(t, meta, example, example.Bad, key.Key+"=false"); len(off.findings) != 0 {
			t.Fatalf("example %d: %s=false did not silence %s", idx, key.Key, meta.ID)
		}
	}
}

func assertExampleShape(t *testing.T, ruleID string, idx int, example core.RuleExample) {
	t.Helper()
	if strings.TrimSpace(example.Language) == "" || strings.TrimSpace(example.Path) == "" {
		t.Fatalf("%s example %d needs a language and a path", ruleID, idx)
	}
	if strings.TrimSpace(example.Bad) == "" || strings.TrimSpace(example.Good) == "" {
		t.Fatalf("%s example %d needs both bad and good snippets", ruleID, idx)
	}
	if example.Bad == example.Good {
		t.Fatalf("%s example %d: bad and good snippets are identical", ruleID, idx)
	}
	if lines := strings.Count(example.Bad, "\n"); lines > 40 {
		t.Fatalf("%s example %d: bad snippet is %d lines; keep examples short", ruleID, idx, lines)
	}
}

// semanticRuntimeRules are only reported by the Node-hosted TypeScript
// semantic engine, so their examples can only be verified where it runs.
var semanticRuntimeRules = map[string]bool{
	"security.typescript.taint-flow": true,
	"security.javascript.taint-flow": true,
}

// typeScriptSemanticRuntimeAvailable mirrors the engine's discovery: node on
// PATH plus a typescript.js from CODEGUARD_TYPESCRIPT_LIB_PATH, an ancestor
// node_modules, or the VS Code bundle.
func typeScriptSemanticRuntimeAvailable() bool {
	if _, err := exec.LookPath("node"); err != nil {
		return false
	}
	candidates := make([]string, 0, 8)
	if value := os.Getenv("CODEGUARD_TYPESCRIPT_LIB_PATH"); value != "" {
		candidates = append(candidates, value)
	}
	if cwd, err := os.Getwd(); err == nil {
		for dir := cwd; ; dir = filepath.Dir(dir) {
			candidates = append(candidates, filepath.Join(dir, "node_modules", "typescript", "lib", "typescript.js"))
			if filepath.Dir(dir) == dir {
				break
			}
		}
	}
	candidates = append(candidates, "/Applications/Visual Studio Code.app/Contents/Resources/app/extensions/node_modules/typescript/lib/typescript.js")
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func normalizedConfidence(value string) string {
	if normalized := core.NormalizedConfidence(value); normalized != "" {
		return normalized
	}
	return core.ConfidenceMedium
}
