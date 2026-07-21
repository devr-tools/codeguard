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

// BatchItem pairs a finding with its proposed mechanical fix. Batch
// verification accepts only catalogued deterministic fixes; guided fixes must
// continue through the single-fix workflow where a caller can review them.
type BatchItem struct {
	Finding   core.Finding `json:"finding"`
	Candidate Candidate    `json:"candidate"`
}

// BatchIssue is a machine-readable explanation for an item which was not
// included in the verified aggregate patch. Index is the item's position in
// BatchRequest.Items.
type BatchIssue struct {
	Index       int    `json:"index"`
	RuleID      string `json:"rule_id,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Reason      string `json:"reason"`
	Detail      string `json:"detail,omitempty"`
}

const (
	BatchReasonUnknownRule           = "unknown_rule"
	BatchReasonNonDeterministic      = "non_deterministic_fix"
	BatchReasonEmptyDiff             = "empty_diff"
	BatchReasonNoChangedFiles        = "no_changed_files"
	BatchReasonConflictingFiles      = "conflicting_files"
	BatchReasonAggregateVerification = "aggregate_verification_failed"
)

// BatchRequest describes proposed fixes to be verified as one patch. It never
// writes to configured targets: verification delegates to Verify, which uses
// an isolated materialized workspace.
type BatchRequest struct {
	Config  core.Config `json:"config"`
	Items   []BatchItem `json:"items"`
	Options Options     `json:"options,omitempty"`
}

// BatchResult contains the verified aggregate patch when Verification is
// present. Skipped and Failures deliberately remain structured so callers do
// not have to parse errors to explain why a requested fix was omitted.
type BatchResult struct {
	Verification Result       `json:"verification,omitempty"`
	Included     []int        `json:"included,omitempty"`
	Skipped      []BatchIssue `json:"skipped,omitempty"`
	Failures     []BatchIssue `json:"failures,omitempty"`
}

type GenerateRequest struct {
	Config    core.Config  `json:"config"`
	Finding   core.Finding `json:"finding"`
	Analysis  string       `json:"analysis,omitempty"`
	Generator Generator    `json:"-"`
	Options   Options      `json:"options,omitempty"`
}

type testStep struct {
	target core.TargetConfig
	dir    string
	check  core.CommandCheckConfig
}
