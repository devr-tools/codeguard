package support

import (
	"context"
	"go/ast"
	"go/token"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type FindingInput struct {
	RuleID  string `json:"rule_id"`
	Level   string `json:"level"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
	// Confidence is "high", "medium", or "low"; empty means unspecified and is
	// treated as medium by consumers.
	Confidence string `json:"confidence,omitempty"`
	// Metadata carries machine-readable, non-sensitive attributes for report
	// consumers. Check implementations must not put source values into it.
	Metadata map[string]string `json:"metadata,omitempty"`
}

type CPPToolIssue struct {
	Path    string
	Message string
}

type CPPToolResult struct {
	Issues      []CPPToolIssue
	Unavailable bool
	Err         error
}

type GovulncheckResult struct {
	Findings    []core.Finding
	Diagnostics []core.Diagnostic
}

// SDKFacade identifies the package(s) that make up the scanned repository's
// published API surface. It is resolved per target from
// design_rules.sdk_facade_paths, or discovered from the repository layout when
// that is unset - never from a path literal baked into codeguard.
type SDKFacade interface {
	// MatchesFile reports whether a target-root-relative path is inside the
	// facade.
	MatchesFile(rel string) bool
	// MatchesImport reports whether a Go import path refers to the facade.
	MatchesImport(importPath string) bool
	// CacheKey fingerprints the resolved facade, so per-file findings that
	// depend on it are invalidated when go.mod or the pkg/ layout changes.
	CacheKey() string
}

type emptySDKFacade struct{}

func (emptySDKFacade) MatchesFile(string) bool   { return false }
func (emptySDKFacade) MatchesImport(string) bool { return false }
func (emptySDKFacade) CacheKey() string          { return "" }

// SDKFacadeForTarget resolves env's facade for target, returning a matcher
// that matches nothing when the hook is unwired.
func SDKFacadeForTarget(env Context, target core.TargetConfig) SDKFacade {
	if env.SDKFacadeFor == nil {
		return emptySDKFacade{}
	}
	if facade := env.SDKFacadeFor(target); facade != nil {
		return facade
	}
	return emptySDKFacade{}
}

type Context struct {
	Config           core.Config
	AIEnabled        bool
	Mode             core.ScanMode
	BaseRef          string
	DiffText         string
	ScanTime         time.Time
	ChangedFiles     []string
	ListChangedFiles func(target core.TargetConfig) ([]core.ChangedFile, error)
	ReadBaseFile     func(target core.TargetConfig, rel string) ([]byte, error)
	DiffScope        func() map[string]core.ChangedLineRanges
	VisitTargetFiles func(target core.TargetConfig, include func(string) bool, visit func(rel string, data []byte))
	// ListTargetFiles returns every non-excluded file under the target root from
	// the shared per-scan corpus walk (the same listing VisitTargetFiles
	// iterates). Nil in unit-test contexts; callers fall back to a direct walk.
	ListTargetFiles func(target core.TargetConfig) ([]string, error)
	// ReadTargetFile reads target-root-relative rel through the shared per-scan
	// file corpus, so a file inspected by several checks is read from disk at
	// most once per scan. Nil in unit-test contexts; callers fall back to a
	// direct read.
	ReadTargetFile  func(target core.TargetConfig, rel string) ([]byte, error)
	ScanTargetFiles func(target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) []core.Finding) []core.Finding
	// ScanTargetFilesWithDependencies is ScanTargetFiles for evaluators that
	// read more than the scanned file; see FileScanDependencies. Call it via
	// the package-level ScanTargetFilesWithDependencies helper, which falls
	// back to ScanTargetFiles when this hook is nil.
	ScanTargetFilesWithDependencies func(target core.TargetConfig, sectionID string, include func(string) bool, deps FileScanDependencies, evaluator func(string, []byte) []core.Finding) []core.Finding
	ParseGoFile                     func(path string, data []byte) (*token.FileSet, *ast.File, error)
	// ParseScriptFile parses one supported non-Go file through the tree-sitter
	// substrate. It is nil unless parsers.treesitter is "auto"; checks treat
	// nil (and any error) as "use the native fallback path".
	ParseScriptFile                func(path string, data []byte, lang ScriptLanguage) (*SyntaxTree, error)
	NewFinding                     func(FindingInput) core.Finding
	FinalizeSection                func(id string, name string, findings []core.Finding) core.SectionResult
	FinalizeSectionWithDiagnostics func(id string, name string, findings []core.Finding, diagnostics []core.Diagnostic) core.SectionResult
	PutArtifact                    func(core.Artifact)
	GetArtifact                    func(string) (core.Artifact, bool)
	CountLines                     func(data []byte) int
	CyclomaticComplexity           func(body *ast.BlockStmt) int
	TypeName                       func(expr ast.Expr) string
	IsInternalOrCmdFile            func(path string) bool
	IsCmdFile                      func(path string) bool
	IsPublicPackageFile            func(path string) bool
	// SDKFacadeFor resolves the scanned repository's published facade package
	// for one target. Nil in unit-test contexts; use SDKFacadeForTarget, which
	// degrades to a matcher that matches nothing.
	SDKFacadeFor           func(target core.TargetConfig) SDKFacade
	IsPromptFile           func(rel string) bool
	RunGovulncheck         func(ctx context.Context, dir string, cmdName string) GovulncheckResult
	RunCPPFormat           func(ctx context.Context, dir string, cfg core.CPPToolingConfig, files []string) CPPToolResult
	RunCPPSyntax           func(ctx context.Context, dir string, cfg core.CPPToolingConfig) CPPToolResult
	RunCommandCheck        func(ctx context.Context, dir string, check core.CommandCheckConfig) (string, error)
	RunCommandCheckWithEnv func(ctx context.Context, dir string, check core.CommandCheckConfig, env []string) (string, error)
	RunDiffCommandCheck    func(ctx context.Context, dir string, baseRef string, check core.CommandCheckConfig) (string, error)
	NormalizedSeverity     func(level string) string
}
