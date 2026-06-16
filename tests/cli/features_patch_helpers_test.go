package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePromptPolicyFixture(t *testing.T, name string, format string, prompt string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "codeguard.json")
	promptPath := filepath.Join(dir, "prompts", "system.prompt")
	if err := os.MkdirAll(filepath.Dir(promptPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(promptPath, []byte(prompt), 0o644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	config := `{
  "name": "` + name + `",
  "targets": [{"name": "repo", "path": "` + dir + `", "language": "go"}],
  "checks": {"quality": false, "design": false, "security": false, "prompts": true, "ci": false},
  "output": {"format": "` + format + `"}
}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath, promptPath
}

func promptSecretPatchDiff() string {
	return strings.Join([]string{
		"diff --git a/prompts/system.prompt b/prompts/system.prompt",
		"index 6d6dd26..9a4f7f4 100644",
		"--- a/prompts/system.prompt",
		"+++ b/prompts/system.prompt",
		"@@ -1 +1 @@",
		"-Keep prompts generic.",
		"+Use ${OPENAI_API_KEY} for downstream calls.",
		"",
	}, "\n")
}

func decodeValidatePatchReport(t *testing.T, body []byte, text string) struct {
	Summary struct {
		FailedSections int `json:"failed_sections"`
		TotalFindings  int `json:"total_findings"`
	} `json:"summary"`
	Sections []struct {
		ID       string `json:"id"`
		Findings []struct {
			RuleID string `json:"rule_id"`
			Path   string `json:"path"`
		} `json:"findings"`
	} `json:"sections"`
} {
	t.Helper()
	var report struct {
		Summary struct {
			FailedSections int `json:"failed_sections"`
			TotalFindings  int `json:"total_findings"`
		} `json:"summary"`
		Sections []struct {
			ID       string `json:"id"`
			Findings []struct {
				RuleID string `json:"rule_id"`
				Path   string `json:"path"`
			} `json:"findings"`
		} `json:"sections"`
	}
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatalf("expected valid report json, got err=%v body=%s", err, text)
	}
	return report
}

func assertPatchedContentFinding(t *testing.T, report struct {
	Summary struct {
		FailedSections int `json:"failed_sections"`
		TotalFindings  int `json:"total_findings"`
	} `json:"summary"`
	Sections []struct {
		ID       string `json:"id"`
		Findings []struct {
			RuleID string `json:"rule_id"`
			Path   string `json:"path"`
		} `json:"findings"`
	} `json:"sections"`
}) {
	t.Helper()
	if report.Summary.FailedSections == 0 || report.Summary.TotalFindings == 0 {
		t.Fatalf("expected failing findings from patched content, got %#v", report.Summary)
	}
	if len(report.Sections) == 0 || len(report.Sections[0].Findings) == 0 {
		t.Fatalf("expected finding details, got %#v", report.Sections)
	}
	if report.Sections[0].Findings[0].RuleID != "prompts.secret-interpolation" {
		t.Fatalf("unexpected rule from patched content: %#v", report.Sections[0].Findings[0])
	}
	if report.Sections[0].Findings[0].Path != "prompts/system.prompt" {
		t.Fatalf("unexpected finding path: %#v", report.Sections[0].Findings[0])
	}
}

func assertPromptFileUnchanged(t *testing.T, promptPath string) {
	t.Helper()
	data, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	if strings.Contains(string(data), "OPENAI_API_KEY") {
		t.Fatalf("working tree file was modified: %s", string(data))
	}
}
