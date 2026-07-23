package support

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// ParseGoSource returns a Go AST for the given source, reusing the shared
// per-scan parse cache when the runner wired one in (ParseGoFile), and falling
// back to a fresh parse otherwise (e.g. in unit tests that build a Context
// directly). Callers must treat the result as read-only, since a cached tree is
// shared across sections.
func ParseGoSource(env Context, file string, data []byte) (*token.FileSet, *ast.File, error) {
	if env.ParseGoFile != nil {
		return env.ParseGoFile(file, data)
	}
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, data, parser.ParseComments)
	return fset, parsed, err
}

func NormalizedLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}

func TrimmedOutput(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	output = strings.Join(strings.Fields(output), " ")
	if len(output) > 240 {
		return output[:237] + "..."
	}
	return output
}

func FindingsFromInputs(env Context, inputs []FindingInput) []core.Finding {
	findings := make([]core.Finding, 0, len(inputs))
	for _, input := range inputs {
		findings = append(findings, env.NewFinding(input))
	}
	return findings
}

func CommandFailureMessage(section string, target core.TargetConfig, check core.CommandCheckConfig, output string, err error) string {
	message := fmt.Sprintf("target %q %s command %q failed", target.Name, section, check.Name)
	if output = TrimmedOutput(output); output != "" {
		return message + ": " + output
	}
	if err != nil {
		return message + ": " + err.Error()
	}
	return message
}

func DiffCommandFailureMessage(section string, target core.TargetConfig, check core.CommandCheckConfig, output string, err error) string {
	message := fmt.Sprintf("target %q %s diff command %q detected contract drift", target.Name, section, check.Name)
	if output = TrimmedOutput(output); output != "" {
		return message + ": " + output
	}
	if err != nil {
		return message + ": " + err.Error()
	}
	return message
}

func RunCommandChecks(ctx context.Context, env Context, target core.TargetConfig, checks []core.CommandCheckConfig, buildFinding func(core.CommandCheckConfig, string, error) core.Finding) []core.Finding {
	findings := make([]core.Finding, 0, len(checks))
	for _, check := range checks {
		output, err := env.RunCommandCheck(ctx, target.Path, check)
		if err == nil {
			continue
		}
		findings = append(findings, buildFinding(check, output, err))
	}
	return findings
}

func RunDiffCommandChecks(ctx context.Context, env Context, target core.TargetConfig, checks []core.CommandCheckConfig, buildFinding func(core.CommandCheckConfig, string, error) core.Finding) []core.Finding {
	if env.Mode != core.ScanModeDiff || env.RunDiffCommandCheck == nil {
		return nil
	}
	findings := make([]core.Finding, 0, len(checks))
	for _, check := range checks {
		output, err := env.RunDiffCommandCheck(ctx, target.Path, env.BaseRef, check)
		if err == nil {
			continue
		}
		findings = append(findings, buildFinding(check, output, err))
	}
	return findings
}

type TypeScriptTargetScan struct {
	SectionID string
	Extract   func(TypeScriptSemanticResults) []FindingInput
	Include   func(string) bool
	Evaluator func(string, []byte) []core.Finding
}

func TypeScriptTargetFindings(ctx context.Context, env Context, target core.TargetConfig, scan TypeScriptTargetScan) []core.Finding {
	results, ok, err := AnalyzeTypeScriptTargetForContext(ctx, env, target)
	if err == nil && ok {
		return FindingsFromInputs(env, scan.Extract(results))
	}
	return env.ScanTargetFiles(target, scan.SectionID, scan.Include, scan.Evaluator)
}
