package action_test

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type actionInput struct {
	Default string `yaml:"default"`
}

type gatedStep struct {
	Name string            `yaml:"name"`
	If   string            `yaml:"if"`
	Env  map[string]string `yaml:"env"`
	Run  string            `yaml:"run"`
}

func loadActionDefinition(t *testing.T) (map[string]actionInput, []gatedStep) {
	t.Helper()
	var action struct {
		Inputs map[string]actionInput `yaml:"inputs"`
		Runs   struct {
			Steps []gatedStep `yaml:"steps"`
		} `yaml:"runs"`
	}
	if err := yaml.Unmarshal(readActionYML(t), &action); err != nil {
		t.Fatalf("parse action.yml: %v", err)
	}
	return action.Inputs, action.Runs.Steps
}

func findGatedStep(t *testing.T, steps []gatedStep, marker string) gatedStep {
	t.Helper()
	for _, step := range steps {
		if strings.Contains(step.Run, marker) {
			return step
		}
	}
	t.Fatalf("action.yml has no step whose run: contains %q", marker)
	return gatedStep{}
}

// Inline review comments are opt-in, and the cap has a finite default.
func TestActionReviewCommentInputsDefaultOff(t *testing.T) {
	inputs, _ := loadActionDefinition(t)
	if got := inputs["review-comments"].Default; got != "off" {
		t.Fatalf("review-comments default = %q, want off", got)
	}
	if got := inputs["review-comment-limit"].Default; got != "25" {
		t.Fatalf("review-comment-limit default = %q, want 25", got)
	}
}

// The review step runs only when enabled and only on the same pull request
// events the sticky comment step accepts.
func TestActionReviewStepGatedLikeCommentStep(t *testing.T) {
	_, steps := loadActionDefinition(t)
	review := findGatedStep(t, steps, "./cmd/codeguard-action-review")
	comment := findGatedStep(t, steps, "-format github-comment")
	if !strings.Contains(review.If, "inputs.review-comments == 'on'") {
		t.Fatalf("review step if: %q does not require review-comments == 'on'", review.If)
	}
	_, commentEvents, ok := strings.Cut(comment.If, "&&")
	if !ok {
		t.Fatalf("comment step if: %q has no event gate", comment.If)
	}
	if !strings.Contains(review.If, strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(commentEvents), exprClose))) {
		t.Fatalf("review step if: %q does not reuse the comment step's event gate %q", review.If, commentEvents)
	}
}

// Every value reaches the review step's shell through env:, and the scan it
// runs emits the JSON report the helper consumes.
func TestActionReviewStepPassesInputsThroughEnv(t *testing.T) {
	_, steps := loadActionDefinition(t)
	review := findGatedStep(t, steps, "./cmd/codeguard-action-review")
	if strings.Contains(review.Run, exprOpen) {
		t.Fatalf("review step run: interpolates an expression:\n%s", review.Run)
	}
	want := map[string]string{
		"CODEGUARD_REVIEW_COMMENT_LIMIT": exprOpen + " inputs.review-comment-limit " + exprClose,
		"CODEGUARD_REPOSITORY":           exprOpen + " github.repository " + exprClose,
		"GITHUB_TOKEN":                   exprOpen + " github.token " + exprClose,
	}
	for key, value := range want {
		if review.Env[key] != value {
			t.Fatalf("review step env %s = %q, want %q", key, review.Env[key], value)
		}
	}
	for _, fragment := range []string{"-format json", `-limit "$CODEGUARD_REVIEW_COMMENT_LIMIT"`, "::error", "exit 1"} {
		if !strings.Contains(review.Run, fragment) {
			t.Fatalf("review step run: lacks %q:\n%s", fragment, review.Run)
		}
	}
}
