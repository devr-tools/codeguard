package action_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type actionStep struct {
	Name             string            `yaml:"name"`
	WorkingDirectory string            `yaml:"working-directory"`
	Env              map[string]string `yaml:"env"`
	Run              string            `yaml:"run"`
}

func loadCommentStep(t *testing.T) actionStep {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "action.yml"))
	if err != nil {
		t.Fatalf("read action.yml: %v", err)
	}
	var action struct {
		Runs struct {
			Steps []actionStep `yaml:"steps"`
		} `yaml:"runs"`
	}
	if err := yaml.Unmarshal(data, &action); err != nil {
		t.Fatalf("parse action.yml: %v", err)
	}
	for _, step := range action.Runs.Steps {
		if strings.Contains(step.Run, "-format github-comment") {
			return step
		}
	}
	t.Fatal("action.yml has no step that runs codeguard scan -format github-comment")
	return actionStep{}
}

// The comment step must scan the caller's checkout, not the action's own
// source, so it may not pin working-directory to the action path.
func TestActionCommentStepScansCallerWorkspace(t *testing.T) {
	step := loadCommentStep(t)
	if strings.Contains(step.WorkingDirectory, "action_path") {
		t.Fatalf("comment step working-directory = %q; it would scan codeguard's own source", step.WorkingDirectory)
	}
}

// Inputs are caller-controlled strings; expanding them inside run: lets a
// crafted value inject shell. They must arrive through env: instead.
func TestActionCommentStepDoesNotInterpolateExpressionsIntoShell(t *testing.T) {
	step := loadCommentStep(t)
	if strings.Contains(step.Run, "${{") {
		t.Fatalf("comment step run: script interpolates a ${{ }} expression:\n%s", step.Run)
	}
}

// An empty report means the scan broke; the step must surface that as an
// error instead of quietly skipping the comment.
func TestActionCommentStepFailsLoudlyOnEmptyReport(t *testing.T) {
	step := loadCommentStep(t)
	if !strings.Contains(step.Run, "::error") || !strings.Contains(step.Run, "exit 1") {
		t.Fatalf("comment step does not emit ::error and exit non-zero on an empty report:\n%s", step.Run)
	}
	if strings.Contains(step.Run, "skipping comment publish") {
		t.Fatalf("comment step still silently skips on an empty report:\n%s", step.Run)
	}
}
