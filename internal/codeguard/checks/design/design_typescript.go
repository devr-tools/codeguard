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
	typeScriptClassPattern       = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:abstract\s+)?class\s+([A-Za-z_$][\w$]*)\b`)
	typeScriptInterfacePattern   = regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?interface\s+([A-Za-z_$][\w$]*)\b`)
	typeScriptObjectTypePattern  = regexp.MustCompile(`^\s*(?:export\s+)?type\s+([A-Za-z_$][\w$]*)\s*=\s*{`)
	typeScriptMethodPattern      = regexp.MustCompile(`^\s*(?:public|private|protected|static|readonly|abstract|override|async|get|set|\s)*(#?[A-Za-z_$][\w$]*)\s*(?:<[^>]+>)?\s*\(`)
	typeScriptArrowMethodPattern = regexp.MustCompile(`^\s*(?:public|private|protected|static|readonly|abstract|override|async|\s)*(#?[A-Za-z_$][\w$]*)\s*=\s*(?:async\s*)?(?:\([^)]*\)|[A-Za-z_$][\w$]*)\s*=>`)
	typeScriptMemberPattern      = regexp.MustCompile(`^(?:readonly\s+)?(?:\[[^\]]+\]\??\s*:\s*.+|#?[A-Za-z_$][\w$?]*\s*(?:\([^)]*\)|:))`)
)

type typeScriptBlockKind int

const (
	typeScriptBlockClass typeScriptBlockKind = iota
	typeScriptBlockInterface
	typeScriptBlockObjectType
)

type typeScriptBlock struct {
	kind      typeScriptBlockKind
	name      string
	line      int
	bodyDepth int
	waiting   bool
	count     int
}

func typeScriptTargetFindingsImpl(env support.Context, target core.TargetConfig) []core.Finding {
	return env.ScanTargetFiles(target, "design", isTypeScriptLikeFile, func(file string, data []byte) []core.Finding {
		return typeScriptFindingsForFile(env, file, data)
	})
}

func typeScriptFindingsForFile(env support.Context, file string, data []byte) []core.Finding {
	findings := forbiddenTypeScriptModuleNameFindings(env, file)
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

	depth := 0
	var active *typeScriptBlock

	for idx, line := range lines {
		if active == nil {
			active = newTypeScriptBlock(line, idx+1)
		}

		if active != nil && !active.waiting && depth == active.bodyDepth {
			switch active.kind {
			case typeScriptBlockClass:
				if name, ok := typeScriptMethodName(line); ok && name != "constructor" {
					active.count++
				}
			case typeScriptBlockInterface, typeScriptBlockObjectType:
				if isTypeScriptMemberLine(line) {
					active.count++
				}
			}
		}

		depth += braceDelta(line)

		if active != nil && active.waiting && strings.Contains(line, "{") {
			active.waiting = false
			active.bodyDepth = depth
		}

		if active != nil && !active.waiting && depth < active.bodyDepth {
			findings = append(findings, typeScriptBlockFindings(env, file, *active)...)
			active = nil
		}
	}

	if active != nil && !active.waiting {
		findings = append(findings, typeScriptBlockFindings(env, file, *active)...)
	}

	return findings
}

func forbiddenTypeScriptModuleNameFindings(env support.Context, file string) []core.Finding {
	moduleName := normalizedTypeScriptModuleName(file)
	for _, forbidden := range env.Config.Checks.DesignRules.ForbiddenPackageNames {
		if strings.EqualFold(moduleName, forbidden) {
			return []core.Finding{env.NewFinding(support.FindingInput{
				RuleID:  "design.typescript.generic-module-name",
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

func normalizedTypeScriptModuleName(path string) string {
	name := strings.ToLower(filepath.Base(path))
	for _, ext := range []string{".d.ts", ".tsx", ".ts", ".jsx", ".js", ".mjs", ".cjs", ".mts", ".cts"} {
		if strings.HasSuffix(name, ext) {
			return strings.TrimSuffix(name, ext)
		}
	}
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func newTypeScriptBlock(line string, lineNo int) *typeScriptBlock {
	if match := typeScriptClassPattern.FindStringSubmatch(line); len(match) == 2 {
		return &typeScriptBlock{
			kind:    typeScriptBlockClass,
			name:    match[1],
			line:    lineNo,
			waiting: true,
		}
	}
	if match := typeScriptInterfacePattern.FindStringSubmatch(line); len(match) == 2 {
		return &typeScriptBlock{
			kind:    typeScriptBlockInterface,
			name:    match[1],
			line:    lineNo,
			waiting: true,
		}
	}
	if match := typeScriptObjectTypePattern.FindStringSubmatch(line); len(match) == 2 {
		return &typeScriptBlock{
			kind:    typeScriptBlockObjectType,
			name:    match[1],
			line:    lineNo,
			waiting: true,
		}
	}
	return nil
}

func typeScriptMethodName(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	match := typeScriptMethodPattern.FindStringSubmatch(trimmed)
	if len(match) == 2 {
		return match[1], true
	}
	match = typeScriptArrowMethodPattern.FindStringSubmatch(trimmed)
	if len(match) != 2 {
		return "", false
	}
	return match[1], true
}

func isTypeScriptMemberLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || trimmed == "}" {
		return false
	}
	if strings.HasSuffix(trimmed, "{") {
		return false
	}
	return typeScriptMemberPattern.MatchString(trimmed)
}

func braceDelta(line string) int {
	line = stripLineComment(line)
	return strings.Count(line, "{") - strings.Count(line, "}")
}

func stripLineComment(line string) string {
	if idx := strings.Index(line, "//"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func typeScriptBlockFindings(env support.Context, file string, block typeScriptBlock) []core.Finding {
	switch block.kind {
	case typeScriptBlockClass:
		if block.count > env.Config.Checks.DesignRules.MaxMethodsPerType {
			return []core.Finding{env.NewFinding(support.FindingInput{
				RuleID:  "design.typescript.max-methods-per-type",
				Level:   "warn",
				Path:    file,
				Line:    block.line,
				Column:  1,
				Message: fmt.Sprintf("class %s has %d methods; max is %d", block.name, block.count, env.Config.Checks.DesignRules.MaxMethodsPerType),
			})}
		}
	case typeScriptBlockInterface, typeScriptBlockObjectType:
		if block.count > env.Config.Checks.DesignRules.MaxInterfaceMethods {
			kind := "interface"
			if block.kind == typeScriptBlockObjectType {
				kind = "type"
			}
			return []core.Finding{env.NewFinding(support.FindingInput{
				RuleID:  "design.typescript.max-interface-members",
				Level:   "warn",
				Path:    file,
				Line:    block.line,
				Column:  1,
				Message: fmt.Sprintf("%s %s has %d members; max is %d", kind, block.name, block.count, env.Config.Checks.DesignRules.MaxInterfaceMethods),
			})}
		}
	}
	return nil
}

func isTypeScriptLikeFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts":
		return true
	default:
		return false
	}
}
