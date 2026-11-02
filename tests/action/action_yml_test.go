package action_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// exprOpen is the "${{" that opens a GitHub Actions expression. It is spelled
// with escapes because codeguard's own test-block scanner counts braces inside
// string literals, and an unbalanced "{{" would merge the tests below into one.
const exprOpen = "$\x7b\x7b"

type actionStep struct {
	Name             string            `yaml:"name"`
	WorkingDirectory string            `yaml:"working-directory"`
	Env              map[string]string `yaml:"env"`
	Run              string            `yaml:"run"`
	Uses             string            `yaml:"uses"`
}

func readActionYML(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "action.yml"))
	if err != nil {
		t.Fatalf("read action.yml: %v", err)
	}
	return data
}

func loadSteps(t *testing.T) []actionStep {
	t.Helper()
	var action struct {
		Runs struct {
			Steps []actionStep `yaml:"steps"`
		} `yaml:"runs"`
	}
	if err := yaml.Unmarshal(readActionYML(t), &action); err != nil {
		t.Fatalf("parse action.yml: %v", err)
	}
	if len(action.Runs.Steps) == 0 {
		t.Fatal("action.yml has no runs.steps")
	}
	return action.Runs.Steps
}

func loadCommentStep(t *testing.T) actionStep {
	t.Helper()
	for _, step := range loadSteps(t) {
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
	if strings.Contains(step.Run, exprOpen) {
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

// No run: script in any step may expand a ${{ }} expression. Inputs and event
// fields are caller-influenced, and even step outputs are safer passed through
// env: than spliced into shell source.
func TestActionRunStepsDoNotInterpolateExpressions(t *testing.T) {
	for i, step := range loadSteps(t) {
		if strings.Contains(step.Run, exprOpen) {
			t.Errorf("step %d (%q) run: script interpolates a ${{ }} expression; pass it through env:\n%s", i, step.Name, step.Run)
		}
	}
}

var shaPinnedUses = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@[0-9a-f]{40}$`)

// Every external action must be pinned to a full commit SHA so a moved tag
// cannot change what runs in the caller's job.
func TestActionUsesArePinnedToCommitSHA(t *testing.T) {
	for i, step := range loadSteps(t) {
		if step.Uses == "" || strings.HasPrefix(step.Uses, "./") {
			continue
		}
		if !shaPinnedUses.MatchString(step.Uses) {
			t.Errorf("step %d (%q) uses %q; pin it to a full 40-character commit SHA", i, step.Name, step.Uses)
		}
	}
}

var (
	usesLine       = regexp.MustCompile(`^\s*(?:-\s+)?uses:\s*(\S+)(.*)$`)
	versionComment = regexp.MustCompile(`^\s+#\s*v\d`)
)

// A SHA pin keeps a trailing "# vX.Y.Z" comment so reviewers and Dependabot
// can tell which release it is.
func TestActionUsesPinsCarryVersionComment(t *testing.T) {
	for n, line := range strings.Split(string(readActionYML(t)), "\n") {
		m := usesLine.FindStringSubmatch(line)
		if m == nil || strings.HasPrefix(m[1], "./") {
			continue
		}
		if !versionComment.MatchString(m[2]) {
			t.Errorf("action.yml:%d: %q lacks a \"# vX.Y.Z\" comment after the SHA", n+1, strings.TrimSpace(line))
		}
	}
}
