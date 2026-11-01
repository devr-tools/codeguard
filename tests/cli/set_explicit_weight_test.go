package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
)

// `-set <weight>=0` must be an explicit 0 (the signal is switched off and
// left out of file_risk), the same as writing the key as 0 in the config
// file, not "use the default".

func TestSetOverrideZeroWeightIsExplicit(t *testing.T) {
	configPath, _ := writePromptPolicyFixture(t, "set-zero-weight", "json", "Keep prompts generic.\n")

	base := fileRiskLabels(t, runPatchForFileRisk(t, configPath))
	if !base["changed_file"] || !base["fail_finding"] {
		t.Fatalf("baseline file_risk components = %v, want changed_file and fail_finding", base)
	}

	zeroed := fileRiskLabels(t, runPatchForFileRisk(t, configPath,
		"-set", "checks.quality_rules.risk_scoring.changed_file_weight=0",
		"-set", "checks.quality_rules.risk_scoring.FailFindingWeight=0",
	))
	if zeroed["changed_file"] || zeroed["fail_finding"] {
		t.Fatalf("file_risk components after -set ...=0 = %v, want changed_file and fail_finding omitted", zeroed)
	}
}

func TestProfileOverrideKeepsFileExplicitZeroWeight(t *testing.T) {
	configPath, _ := writePromptPolicyFixture(t, "profile-zero-weight", "json", "Keep prompts generic.\n")
	data, err := os.ReadFile(configPath) //nolint:gosec // test-owned temp file
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	withZero := strings.Replace(string(data), `"ci": false}`, `"ci": false, "quality_rules": {"risk_scoring": {"changed_file_weight": 0}}}`, 1)
	if withZero == string(data) {
		t.Fatalf("fixture shape changed; could not add risk_scoring")
	}
	if err := os.WriteFile(configPath, []byte(withZero), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	labels := fileRiskLabels(t, runPatchForFileRisk(t, configPath, "-profile", "strict"))
	if labels["changed_file"] {
		t.Fatalf("file_risk components with -profile strict = %v, want the file's explicit 0 changed_file_weight kept", labels)
	}
	if !labels["fail_finding"] {
		t.Fatalf("file_risk components = %v, want fail_finding still present", labels)
	}
}

func runPatchForFileRisk(t *testing.T, configPath string, extra ...string) []byte {
	t.Helper()
	args := append([]string{"validate-patch", "-config", configPath, "-format", "json"}, extra...)
	var stdout, stderr bytes.Buffer
	code := cli.Run(args, strings.NewReader(promptSecretPatchDiff()), &stdout, &stderr)
	if code != 1 {
		t.Fatalf("validate-patch %v exit = %d, want 1 (failing patch); stderr=%s", extra, code, stderr.String())
	}
	return stdout.Bytes()
}

func fileRiskLabels(t *testing.T, body []byte) map[string]bool {
	t.Helper()
	var report struct {
		Artifacts []struct {
			FileRisk *struct {
				Files []struct {
					Components []struct {
						Label string `json:"label"`
					} `json:"components"`
				} `json:"files"`
			} `json:"file_risk"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(body, &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, body)
	}
	labels := map[string]bool{}
	found := false
	for _, artifact := range report.Artifacts {
		if artifact.FileRisk == nil {
			continue
		}
		found = true
		for _, file := range artifact.FileRisk.Files {
			for _, component := range file.Components {
				labels[component.Label] = true
			}
		}
	}
	if !found {
		t.Fatalf("report has no file_risk artifact:\n%s", body)
	}
	return labels
}
