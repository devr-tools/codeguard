package checks

import (
	"context"
	"errors"

	checksupport "github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/cpp/compdb"
	cpptoolingrunner "github.com/devr-tools/codeguard/internal/codeguard/runner/cpptooling"
)

func runCPPFormat(ctx context.Context, dir string, cfg core.CPPToolingConfig, files []string) checksupport.CPPToolResult {
	issues, err := cpptoolingrunner.CheckFormat(ctx, dir, cfg, files)
	return cppToolResult(issues, err)
}

func runCPPSyntax(ctx context.Context, dir string, cfg core.CPPToolingConfig) checksupport.CPPToolResult {
	issues, err := cpptoolingrunner.CheckSyntax(ctx, dir, cfg)
	return cppToolResult(issues, err)
}

func cppToolResult(issues []cpptoolingrunner.Issue, err error) checksupport.CPPToolResult {
	result := checksupport.CPPToolResult{Err: err}
	result.Unavailable = errors.Is(err, cpptoolingrunner.ErrToolUnavailable) || errors.Is(err, compdb.ErrNotFound)
	result.Issues = make([]checksupport.CPPToolIssue, 0, len(issues))
	for _, issue := range issues {
		result.Issues = append(result.Issues, checksupport.CPPToolIssue{Path: issue.Path, Message: issue.Message})
	}
	return result
}
