package support

import (
	"context"
	"go/ast"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type FindingInput struct {
	RuleID  string `json:"rule_id"`
	Level   string `json:"level"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Message string `json:"message"`
}

type Context struct {
	Config               core.Config
	AIEnabled            bool
	Mode                 core.ScanMode
	BaseRef              string
	DiffText             string
	ScanTargetFiles      func(target core.TargetConfig, sectionID string, include func(string) bool, evaluator func(string, []byte) []core.Finding) []core.Finding
	NewFinding           func(FindingInput) core.Finding
	FinalizeSection      func(id string, name string, findings []core.Finding) core.SectionResult
	PutArtifact          func(core.Artifact)
	GetArtifact          func(string) (core.Artifact, bool)
	CountLines           func(data []byte) int
	CyclomaticComplexity func(body *ast.BlockStmt) int
	TypeName             func(expr ast.Expr) string
	IsInternalOrCmdFile  func(path string) bool
	IsCmdFile            func(path string) bool
	IsPublicPackageFile  func(path string) bool
	IsSDKFacadeFile      func(path string) bool
	IsPromptFile         func(rel string) bool
	RunGovulncheck       func(ctx context.Context, dir string, cmdName string) ([]core.Finding, error)
	RunCommandCheck      func(ctx context.Context, dir string, check core.CommandCheckConfig) (string, error)
	RunDiffCommandCheck  func(ctx context.Context, dir string, baseRef string, check core.CommandCheckConfig) (string, error)
	NormalizedSeverity   func(level string) string
}
