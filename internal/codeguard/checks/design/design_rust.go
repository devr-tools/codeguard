package design

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type rustFileScan struct {
	env          support.Context
	file         string
	findings     []core.Finding
	active       *rustBlock
	depth        int
	methodCounts map[string]rustCountSummary
}

func rustTargetFindings(env support.Context, target core.TargetConfig) []core.Finding {
	return env.ScanTargetFiles(target, "design", func(rel string) bool {
		return strings.HasSuffix(rel, ".rs")
	}, func(file string, data []byte) []core.Finding {
		return rustFindingsForFile(env, file, data)
	})
}

// RustFindingsForFile exposes the Rust-native design heuristics independently
// of shared dispatch so focused tests can exercise them directly.
func RustFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	return rustFindingsForFile(env, file, data)
}

func rustFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	scan := rustFileScan{
		env:          env,
		file:         file,
		findings:     rustGenericModuleNameFindings(env, file),
		methodCounts: map[string]rustCountSummary{},
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	for idx, raw := range lines {
		line := stripLineComment(raw)
		scan.active = nextRustBlock(scan.active, scan.depth, line, idx+1)
		updateRustBlockHeader(scan.active, line)
		countRustBlockMember(scan.active, scan.depth, line)
		scan.depth += braceDelta(line)
		openRustBlock(scan.active, scan.depth, line)
		closeRustBlock(&scan)
	}

	if scan.active != nil && !scan.active.waiting {
		scan.findings = append(scan.findings, finalizeRustBlock(scan.env, scan.file, *scan.active, scan.methodCounts)...)
	}

	scan.findings = append(scan.findings, rustMethodFindings(scan.env, scan.file, scan.methodCounts)...)
	return scan.findings
}

func rustGenericModuleNameFindings(env support.Context, file string) []core.Finding {
	moduleName := normalizedRustModuleName(file)
	if moduleName == "" {
		return nil
	}
	for _, forbidden := range env.Config.Checks.DesignRules.ForbiddenPackageNames {
		if strings.EqualFold(moduleName, forbidden) {
			return []core.Finding{env.NewFinding(support.FindingInput{
				RuleID:  "design.rust.generic-module-name",
				Level:   "warn",
				Path:    file,
				Line:    1,
				Column:  1,
				Message: fmt.Sprintf("module name %q is too generic", moduleName),
			})}
		}
	}
	return nil
}

func normalizedRustModuleName(path string) string {
	base := strings.TrimSuffix(strings.ToLower(filepath.Base(path)), filepath.Ext(path))
	switch base {
	case "", "lib", "main":
		return ""
	case "mod":
		parent := strings.ToLower(filepath.Base(filepath.Dir(path)))
		if parent == "." || parent == "/" || parent == "src" {
			return ""
		}
		return parent
	default:
		return base
	}
}
