package support

import (
	"context"
	"go/ast"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type FindingInput struct {
	RuleID  string
	Level   string
	Path    string
	Line    int
	Column  int
	Message string
}

type Context struct {
	Config               core.Config
	ScanTargetFiles      func(target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) []core.Finding) []core.Finding
	NewFinding           func(FindingInput) core.Finding
	FinalizeSection      func(id string, name string, findings []core.Finding) core.SectionResult
	CountLines           func(data []byte) int
	CyclomaticComplexity func(body *ast.BlockStmt) int
	TypeName             func(expr ast.Expr) string
	IsInternalOrCmdFile  func(path string) bool
	IsCmdFile            func(path string) bool
	IsPublicPackageFile  func(path string) bool
	IsSDKFacadeFile      func(path string) bool
	IsPromptFile         func(rel string) bool
	RunGovulncheck       func(ctx context.Context, dir string, cmdName string) ([]core.Finding, error)
	NormalizedSeverity   func(level string) string
}
