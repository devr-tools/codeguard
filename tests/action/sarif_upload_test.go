package action_test

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type sarifActionStep struct {
	Name string            `yaml:"name"`
	ID   string            `yaml:"id"`
	If   string            `yaml:"if"`
	Uses string            `yaml:"uses"`
	With map[string]string `yaml:"with"`
	Env  map[string]string `yaml:"env"`
	Run  string            `yaml:"run"`
}

type sarifActionInput struct {
	Default string `yaml:"default"`
}

func loadSARIFAction(t *testing.T) (map[string]sarifActionInput, []sarifActionStep) {
	t.Helper()
	var action struct {
		Inputs map[string]sarifActionInput `yaml:"inputs"`
		Runs   struct {
			Steps []sarifActionStep `yaml:"steps"`
		} `yaml:"runs"`
	}
	if err := yaml.Unmarshal(readActionYML(t), &action); err != nil {
		t.Fatalf("parse action.yml: %v", err)
	}
	return action.Inputs, action.Runs.Steps
}

func expr(body string) string {
	return exprOpen + " " + body + " " + exprClose
}

// SARIF upload is opt-in: it needs security-events: write, which a caller
// must grant deliberately.
func TestActionSARIFInputsDefaults(t *testing.T) {
	inputs, _ := loadSARIFAction(t)
	want := map[string]string{"upload-sarif": "false", "sarif-category": "codeguard", "sarif-file": ""}
	for name, def := range want {
		input, ok := inputs[name]
		if !ok {
			t.Errorf("action.yml has no %s input", name)
			continue
		}
		if input.Default != def {
			t.Errorf("input %s default = %q, want %q", name, input.Default, def)
		}
	}
}

// The scan step itself writes the SARIF file through -sarif-output, so one
// scan feeds both the annotations and the upload. The inputs arrive through
// env:, and the file is only requested when upload-sarif is true.
func TestActionScanStepWritesSARIFThroughEnv(t *testing.T) {
	_, steps := loadSARIFAction(t)
	var scan sarifActionStep
	for _, step := range steps {
		if step.ID == "scan" {
			scan = step
		}
	}
	if scan.Run == "" {
		t.Fatal("action.yml has no step with id scan")
	}
	if got := scan.Env["CODEGUARD_UPLOAD_SARIF"]; got != expr("inputs.upload-sarif") {
		t.Errorf("scan step env CODEGUARD_UPLOAD_SARIF = %q, want the upload-sarif input", got)
	}
	if got := scan.Env["CODEGUARD_SARIF_FILE"]; got != expr("inputs.sarif-file") {
		t.Errorf("scan step env CODEGUARD_SARIF_FILE = %q, want the sarif-file input", got)
	}
	for _, want := range []string{
		`"$CODEGUARD_UPLOAD_SARIF"`,
		`scan_args+=(-sarif-output "$sarif_file")`,
		`echo "sarif_file=$sarif_file" >> "$GITHUB_OUTPUT"`,
		"exit 0",
	} {
		if !strings.Contains(scan.Run, want) {
			t.Errorf("scan step run: lacks %q:\n%s", want, scan.Run)
		}
	}
}

// The upload runs only when the caller opted in and the scan produced a file,
// also after a failing scan (the scan step exits 0 and records exit_code),
// and before the final step re-raises the scan's exit status.
func TestActionUploadSARIFStepIsGatedAndPinned(t *testing.T) {
	_, steps := loadSARIFAction(t)
	uploadAt, failAt := -1, -1
	for i, step := range steps {
		if strings.HasPrefix(step.Uses, "github/codeql-action/upload-sarif@") {
			uploadAt = i
		}
		if strings.Contains(step.Run, `exit "$CODEGUARD_EXIT_CODE"`) {
			failAt = i
		}
	}
	if uploadAt < 0 {
		t.Fatal("action.yml has no github/codeql-action/upload-sarif step")
	}
	upload := steps[uploadAt]
	if !shaPinnedUses.MatchString(upload.Uses) {
		t.Errorf("upload step uses %q; pin it to a full commit SHA", upload.Uses)
	}
	for _, want := range []string{"!cancelled()", "inputs.upload-sarif == 'true'", "steps.scan.outputs.sarif_file != ''"} {
		if !strings.Contains(upload.If, want) {
			t.Errorf("upload step if: %q lacks %q", upload.If, want)
		}
	}
	if got := upload.With["sarif_file"]; got != expr("steps.scan.outputs.sarif_file") {
		t.Errorf("upload step sarif_file = %q, want the scan step's sarif_file output", got)
	}
	if got := upload.With["category"]; got != expr("inputs.sarif-category") {
		t.Errorf("upload step category = %q, want the sarif-category input", got)
	}
	if failAt < 0 || uploadAt > failAt {
		t.Errorf("upload step (index %d) must run before the step that exits with the scan status (index %d)", uploadAt, failAt)
	}
}
