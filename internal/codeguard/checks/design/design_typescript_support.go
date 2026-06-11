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
		return &typeScriptBlock{kind: typeScriptBlockClass, name: match[1], line: lineNo, waiting: true}
	}
	if match := typeScriptInterfacePattern.FindStringSubmatch(line); len(match) == 2 {
		return &typeScriptBlock{kind: typeScriptBlockInterface, name: match[1], line: lineNo, waiting: true}
	}
	if match := typeScriptObjectTypePattern.FindStringSubmatch(line); len(match) == 2 {
		return &typeScriptBlock{kind: typeScriptBlockObjectType, name: match[1], line: lineNo, waiting: true}
	}
	return nil
}

func typeScriptMethodName(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if match := typeScriptMethodPattern.FindStringSubmatch(trimmed); len(match) == 2 {
		return match[1], true
	}
	match := typeScriptArrowMethodPattern.FindStringSubmatch(trimmed)
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
