package fix

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type Generator interface {
	GenerateFix(context.Context, GenerateInput) (Candidate, error)
}

type GenerateInput struct {
	Config       core.Config
	Finding      core.Finding
	Analysis     string
	Instructions string
}

type Candidate struct {
	Summary string `json:"summary,omitempty"`
	Diff    string `json:"diff"`
}

type Options struct {
	BaseRef         string                `json:"base_ref,omitempty"`
	MaxNearestTests int                   `json:"max_nearest_tests,omitempty"`
	TestCommands    []VerificationCommand `json:"test_commands,omitempty"`
}

type VerificationCommand struct {
	TargetName string                  `json:"target_name,omitempty"`
	Check      core.CommandCheckConfig `json:"check"`
}

type CommandResult struct {
	TargetName string `json:"target_name,omitempty"`
	CheckName  string `json:"check_name,omitempty"`
	Command    string `json:"command,omitempty"`
	Output     string `json:"output,omitempty"`
}

type Result struct {
	Summary      string          `json:"summary,omitempty"`
	Diff         string          `json:"diff"`
	Report       core.Report     `json:"report"`
	ChangedFiles []string        `json:"changed_files,omitempty"`
	TestResults  []CommandResult `json:"test_results,omitempty"`
}

type testStep struct {
	target core.TargetConfig
	dir    string
	check  core.CommandCheckConfig
}
