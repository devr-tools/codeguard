package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func TestBaselineAuditAndPruneDoNotAcceptNewFindings(t *testing.T) {
	dir, configPath, baselinePath := governanceFixture(t)
	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"baseline", "-config", configPath, "-output", baselinePath}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("create exit=%d stderr=%s", code, stderr.String())
	}
	file := readBaselineFixture(t, baselinePath)
	file.Entries = append(file.Entries, core.BaselineEntry{Fingerprint: "stale", RuleID: "quality.stale", Path: "old.go"})
	writeBaselineFixture(t, baselinePath, file)
	original, _ := os.ReadFile(baselinePath)

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"baseline", "audit", "-config", configPath, "-baseline", baselinePath, "-format", "json"}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("audit exit=%d stderr=%s", code, stderr.String())
	}
	var audit struct {
		Counts struct {
			Stale int `json:"stale"`
		} `json:"counts"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &audit); err != nil || audit.Counts.Stale != 1 {
		t.Fatalf("audit=%#v err=%v body=%s", audit, err, stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"baseline", "prune", "-config", configPath, "-baseline", baselinePath, "-check"}, strings.NewReader(""), &stdout, &stderr); code == 0 {
		t.Fatal("prune --check should fail for stale entries")
	}
	afterCheck, _ := os.ReadFile(baselinePath)
	if !bytes.Equal(original, afterCheck) {
		t.Fatal("prune --check modified baseline")
	}

	candidate := filepath.Join(dir, "candidate.json")
	stdout.Reset()
	stderr.Reset()
	if code := cli.Run([]string{"baseline", "prune", "-config", configPath, "-baseline", baselinePath, "-write", "-output", candidate}, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("prune --write exit=%d stderr=%s", code, stderr.String())
	}
	pruned := readBaselineFixture(t, candidate)
	if len(pruned.Entries) != len(file.Entries)-1 {
		t.Fatalf("entries=%d want=%d", len(pruned.Entries), len(file.Entries)-1)
	}
	for _, entry := range pruned.Entries {
		if entry.Fingerprint == "stale" {
			t.Fatal("stale entry retained")
		}
	}
}

func TestBaselinePolicyRejectsGrowthAndProhibitedAddition(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "current.json")
	comparisonPath := filepath.Join(dir, "comparison.json")
	configPath := filepath.Join(dir, "codeguard.json")
	writeBaselineFixture(t, comparisonPath, core.BaselineFile{Entries: []core.BaselineEntry{{Fingerprint: "old", RuleID: "security.old"}}})
	writeBaselineFixture(t, currentPath, core.BaselineFile{Entries: []core.BaselineEntry{{Fingerprint: "old", RuleID: "security.old"}, {Fingerprint: "new", RuleID: "security.new"}}})
	config := `{"name":"policy","targets":[{"name":"repo","path":"` + dir + `","language":"go"}],"baseline":{"path":"current.json","governance":{"forbid_growth":true,"prohibited_new_rule_prefixes":["security."]}}}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := cli.Run([]string{"baseline", "policy", "-config", configPath, "-baseline", currentPath, "-compare-baseline", comparisonPath, "-format", "json"}, strings.NewReader(""), &stdout, &stderr); code == 0 {
		t.Fatalf("policy unexpectedly passed: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), `"prohibited_rule"`) || !strings.Contains(stdout.String(), `"growth"`) {
		t.Fatalf("policy output=%s stderr=%s", stdout.String(), stderr.String())
	}
}

func governanceFixture(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	baselinePath := filepath.Join(dir, "baseline.json")
	promptPath := filepath.Join(dir, "prompts", "system.prompt")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(promptPath, []byte("Use ${OPENAI_API_KEY}.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	config := `{"name":"governance","targets":[{"name":"repo","path":"` + dir + `","language":"go"}],"checks":{"quality":false,"design":false,"security":false,"prompts":true,"ci":false},"baseline":{"path":"baseline.json","governance":{"require_no_stale_entries":true}},"output":{"format":"json"}}`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir, configPath, baselinePath
}

func readBaselineFixture(t *testing.T, path string) core.BaselineFile {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var file core.BaselineFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	return file
}

func writeBaselineFixture(t *testing.T, path string, file core.BaselineFile) {
	t.Helper()
	data, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
