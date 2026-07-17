package design

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	rustTraitPattern       = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:unsafe\s+)?trait\s+([A-Za-z_]\w*)\b`)
	rustImplPattern        = regexp.MustCompile(`^\s*impl\b`)
	rustMethodPattern      = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:default\s+)?(?:async\s+)?(?:const\s+)?(?:unsafe\s+)?fn\s+([A-Za-z_]\w*)\b`)
	rustTraitMemberPattern = regexp.MustCompile(`^\s*(?:(?:default\s+)?(?:async\s+)?(?:const\s+)?(?:unsafe\s+)?fn\s+[A-Za-z_]\w*\b|type\s+[A-Za-z_]\w*\b|const\s+[A-Za-z_]\w*\b)`)
)

type rustBlockKind int

const (
	rustBlockImpl rustBlockKind = iota
	rustBlockTrait
)

type rustBlock struct {
	kind      rustBlockKind
	name      string
	header    string
	line      int
	bodyDepth int
	waiting   bool
	count     int
}

type rustCountSummary struct {
	line  int
	count int
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
	findings := rustGenericModuleNameFindings(env, file)
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	methodCounts := map[string]rustCountSummary{}

	depth := 0
	var active *rustBlock

	for idx, raw := range lines {
		line := stripLineComment(raw)
		active = nextRustBlock(active, depth, line, idx+1)
		updateRustBlockHeader(active, line)
		countRustBlockMember(active, depth, line)
		depth += braceDelta(line)
		openRustBlock(active, depth, line)
		findings, active = closeRustBlock(findings, env, file, active, depth, methodCounts)
	}

	if active != nil && !active.waiting {
		findings = append(findings, finalizeRustBlock(env, file, *active, methodCounts)...)
	}

	findings = append(findings, rustMethodFindings(env, file, methodCounts)...)
	return findings
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

func nextRustBlock(active *rustBlock, depth int, line string, lineNo int) *rustBlock {
	if active != nil || depth != 0 {
		return active
	}
	if match := rustTraitPattern.FindStringSubmatch(line); len(match) == 2 {
		return &rustBlock{kind: rustBlockTrait, name: match[1], line: lineNo, waiting: true}
	}
	if rustImplPattern.MatchString(line) {
		return &rustBlock{kind: rustBlockImpl, line: lineNo, waiting: true}
	}
	return nil
}

func updateRustBlockHeader(active *rustBlock, line string) {
	if active == nil || !active.waiting {
		return
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return
	}
	if active.header != "" {
		active.header += " "
	}
	active.header += trimmed
}

func countRustBlockMember(active *rustBlock, depth int, line string) {
	if active == nil || active.waiting || depth != active.bodyDepth {
		return
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || trimmed == "}" {
		return
	}
	switch active.kind {
	case rustBlockImpl:
		if rustMethodPattern.MatchString(trimmed) {
			active.count++
		}
	case rustBlockTrait:
		if rustTraitMemberPattern.MatchString(trimmed) {
			active.count++
		}
	}
}

func openRustBlock(active *rustBlock, depth int, line string) {
	if active == nil || !active.waiting || !strings.Contains(line, "{") {
		return
	}
	if active.kind == rustBlockImpl {
		active.name = rustImplTargetName(active.header)
	}
	active.waiting = false
	active.bodyDepth = depth
}

func closeRustBlock(findings []core.Finding, env support.Context, file string, active *rustBlock, depth int, methodCounts map[string]rustCountSummary) ([]core.Finding, *rustBlock) {
	if active == nil || active.waiting || depth >= active.bodyDepth {
		return findings, active
	}
	findings = append(findings, finalizeRustBlock(env, file, *active, methodCounts)...)
	return findings, nil
}

func finalizeRustBlock(env support.Context, file string, block rustBlock, methodCounts map[string]rustCountSummary) []core.Finding {
	switch block.kind {
	case rustBlockImpl:
		if block.name == "" || block.count == 0 {
			return nil
		}
		summary := methodCounts[block.name]
		if summary.line == 0 {
			summary.line = block.line
		}
		summary.count += block.count
		methodCounts[block.name] = summary
	case rustBlockTrait:
		if block.name == "" || block.count <= env.Config.Checks.DesignRules.MaxInterfaceMethods {
			return nil
		}
		return []core.Finding{env.NewFinding(support.FindingInput{
			RuleID:  "design.rust.max-trait-members",
			Level:   "warn",
			Path:    file,
			Line:    block.line,
			Column:  1,
			Message: fmt.Sprintf("trait %s exposes %d members; max is %d", block.name, block.count, env.Config.Checks.DesignRules.MaxInterfaceMethods),
		})}
	}
	return nil
}

func rustMethodFindings(env support.Context, file string, methodCounts map[string]rustCountSummary) []core.Finding {
	findings := make([]core.Finding, 0)
	for typeName, summary := range methodCounts {
		if summary.count <= env.Config.Checks.DesignRules.MaxMethodsPerType {
			continue
		}
		line := summary.line
		if line == 0 {
			line = 1
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:  "design.rust.max-methods-per-type",
			Level:   "warn",
			Path:    file,
			Line:    line,
			Column:  1,
			Message: fmt.Sprintf("type %s has %d impl methods in this file; max is %d", typeName, summary.count, env.Config.Checks.DesignRules.MaxMethodsPerType),
		}))
	}
	return findings
}

func rustImplTargetName(header string) string {
	header = strings.Join(strings.Fields(strings.Split(header, "{")[0]), " ")
	header = strings.TrimSpace(strings.TrimPrefix(header, "impl"))
	header = strings.TrimSpace(trimRustGenericPrefix(header))
	if header == "" {
		return ""
	}
	if idx := strings.LastIndex(header, " for "); idx >= 0 {
		header = header[idx+5:]
	}
	if idx := strings.Index(header, " where "); idx >= 0 {
		header = header[:idx]
	}
	return rustPrimaryTypeName(strings.TrimSpace(header))
}

func trimRustGenericPrefix(header string) string {
	if !strings.HasPrefix(header, "<") {
		return header
	}
	depth := 0
	for idx, char := range header {
		switch char {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				return strings.TrimSpace(header[idx+1:])
			}
		}
	}
	return header
}

func rustPrimaryTypeName(target string) string {
	target = strings.TrimSpace(target)
	for {
		target = strings.TrimSpace(strings.TrimPrefix(target, "&"))
		target = strings.TrimSpace(strings.TrimPrefix(target, "mut "))
		target = strings.TrimSpace(strings.TrimPrefix(target, "dyn "))
		fields := strings.Fields(target)
		if len(fields) > 0 && strings.HasPrefix(fields[0], "'") {
			target = strings.TrimSpace(strings.Join(fields[1:], " "))
			continue
		}
		break
	}
	for _, sep := range []string{"<", " ", "(", "[", "{"} {
		if idx := strings.Index(target, sep); idx >= 0 {
			target = target[:idx]
		}
	}
	if idx := strings.LastIndex(target, "::"); idx >= 0 {
		target = target[idx+2:]
	}
	return strings.TrimSpace(target)
}
