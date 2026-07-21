package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestFixBatchVerifiesExplicitDeterministicItemsWithoutChangingTarget(t *testing.T) {
	dir := t.TempDir()
	writeFixBatchFile(t, filepath.Join(dir, "go.mod"), "module example.com/fixbatchcli\n\ngo 1.23.0\n")
	servicePath := filepath.Join(dir, "service.go")
	writeFixBatchFile(t, servicePath, "package fixbatchcli\n\nfunc run(){ }\n")
	writeFixBatchFile(t, filepath.Join(dir, "service_test.go"), "package fixbatchcli\n\nimport \"testing\"\n\nfunc TestRun(t *testing.T) { run() }\n")

	configPath := filepath.Join(dir, "codeguard.json")
	config := `{
  "name": "fix-batch-cli",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go"}],
  "checks": {"quality": true, "design": false, "security": false, "prompts": false, "ci": false}
}`
	writeFixBatchFile(t, configPath, config)

	diff := strings.Join([]string{
		"diff --git a/service.go b/service.go",
		"--- a/service.go",
		"+++ b/service.go",
		"@@ -1,3 +1,3 @@",
		" package fixbatchcli",
		" ",
		"-func run(){ }",
		"+func run() {}",
		"",
	}, "\n")
	input, err := json.Marshal(struct {
		Items []service.FixBatchItem `json:"items"`
	}{Items: []service.FixBatchItem{{
		Finding:   service.Finding{RuleID: "quality.gofmt", Fingerprint: "format"},
		Candidate: service.FixCandidate{Diff: diff},
	}}})
	if err != nil {
		t.Fatalf("marshal input: %v", err)
	}
	inputPath := filepath.Join(dir, "fixes.json")
	writeFixBatchFile(t, inputPath, string(input))

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"fix-batch", "-config", configPath, "-base-ref", "", "-allow-config-commands", "-input", inputPath}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("fix-batch exit code = %d, stderr = %s", code, stderr.String())
	}
	var result service.FixBatchResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v\noutput: %s", err, stdout.String())
	}
	if len(result.Included) != 1 || !strings.Contains(result.Verification.Diff, "func run() {}") {
		t.Fatalf("unexpected batch result: %#v", result)
	}
	contents, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("read original target: %v", err)
	}
	if strings.Contains(string(contents), "func run() {}") {
		t.Fatalf("fix-batch modified the working tree: %q", contents)
	}
}

func TestFixBatchRequiresExplicitInput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"fix-batch"}, strings.NewReader(""), &stdout, &stderr)
	if code == 0 || !strings.Contains(stderr.String(), "requires -input") {
		t.Fatalf("expected explicit-input failure, code=%d stderr=%q", code, stderr.String())
	}
}

func writeFixBatchFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
