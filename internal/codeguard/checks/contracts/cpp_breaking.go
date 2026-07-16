package contracts

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type cppSymbol struct {
	signature string
	line      int
}

var (
	cppPublicTypePattern = regexp.MustCompile(`(?m)^[ \t]*(?:template\s*<[^\n]+>\s*)?(?:class|struct|enum(?:\s+class)?)\s+(?:[A-Z_][A-Z0-9_]*\s+)?([A-Za-z_]\w*)\b`)
	cppUsingPattern      = regexp.MustCompile(`(?m)^[ \t]*using\s+([A-Za-z_]\w*)\s*=`)
	cppFunctionPattern   = regexp.MustCompile(`([~A-Za-z_]\w*)\s*\(([^()]*)\)\s*((?:const\s*)?(?:noexcept(?:\s*\([^)]*\))?\s*)?(?:override\s*)?(?:final\s*)?(?:=\s*(?:0|default|delete)\s*)?);$`)
	cppParamNamePattern  = regexp.MustCompile(`^(.+[\s*&])([A-Za-z_]\w*)(\s*\[[^]]*\])?$`)
)

func cppBreakingFindings(env support.Context, target core.TargetConfig, changed []core.ChangedFile) []core.Finding {
	if !enabled(env.Config.Checks.ContractRules.CPPPublicBreaking) {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, file := range changed {
		if file.Status == core.ChangedFileAdded || !isCPPPublicHeader(file.Path) {
			continue
		}
		findings = append(findings, cppFileBreakingFindings(env, target, file)...)
	}
	return findings
}

func isCPPPublicHeader(rel string) bool {
	switch strings.ToLower(filepath.Ext(rel)) {
	case ".h", ".hh", ".hpp", ".hxx", ".h++":
	default:
		return false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, part := range parts[:len(parts)-1] {
		switch strings.ToLower(part) {
		case "include", "public", "api":
			return true
		}
	}
	return false
}

func cppFileBreakingFindings(env support.Context, target core.TargetConfig, file core.ChangedFile) []core.Finding {
	baseSymbols := cppPublicSymbols(readBase(env, target, file.Path))
	if len(baseSymbols) == 0 {
		return nil
	}
	headSymbols := map[string][]cppSymbol{}
	if file.Status != core.ChangedFileDeleted {
		headSymbols = cppPublicSymbols(readHead(target, file.Path))
	}

	findings := make([]core.Finding, 0)
	for _, name := range sortedKeys(baseSymbols) {
		base := baseSymbols[name]
		head := headSymbols[name]
		if len(head) == 0 {
			findings = append(findings, newCPPBreakingFinding(env, file.Path, 0, fmt.Sprintf("public C++ %s was removed or renamed against the base ref", name)))
			continue
		}
		if strings.HasPrefix(name, "function ") {
			findings = append(findings, cppSignatureFindings(env, file.Path, name, base, head)...)
		}
	}
	return findings
}

func cppSignatureFindings(env support.Context, path string, name string, base []cppSymbol, head []cppSymbol) []core.Finding {
	headSignatures := make(map[string]cppSymbol, len(head))
	for _, symbol := range head {
		headSignatures[symbol.signature] = symbol
	}
	missing := make([]string, 0)
	for _, symbol := range base {
		if _, ok := headSignatures[symbol.signature]; !ok {
			missing = append(missing, symbol.signature)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	line := head[0].line
	if len(base) == 1 && len(head) == 1 {
		return []core.Finding{newCPPBreakingFinding(env, path, line, fmt.Sprintf("public C++ %s changed signature from %s to %s", name, base[0].signature, head[0].signature))}
	}
	findings := make([]core.Finding, 0, len(missing))
	for _, signature := range missing {
		findings = append(findings, newCPPBreakingFinding(env, path, line, fmt.Sprintf("public C++ overload %s %s was removed or changed", name, signature)))
	}
	return findings
}

func newCPPBreakingFinding(env support.Context, path string, line int, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  "contracts.cpp-public-breaking",
		Level:   "fail",
		Path:    path,
		Line:    line,
		Message: message,
	})
}

func cppPublicSymbols(src []byte) map[string][]cppSymbol {
	if len(src) == 0 {
		return nil
	}
	source := strings.ReplaceAll(string(src), "\r\n", "\n")
	masked := support.MaskCLikeSource(source, support.CLikeCPP)
	symbols := make(map[string][]cppSymbol)
	for _, match := range cppPublicTypePattern.FindAllStringSubmatchIndex(masked, -1) {
		name := masked[match[2]:match[3]]
		appendCPPSymbol(symbols, "type "+name, cppSymbol{line: support.LineNumberForOffset(masked, match[2])})
	}
	for _, match := range cppUsingPattern.FindAllStringSubmatchIndex(masked, -1) {
		name := masked[match[2]:match[3]]
		appendCPPSymbol(symbols, "alias "+name, cppSymbol{line: support.LineNumberForOffset(masked, match[2])})
	}
	for start, idx := 0, 0; idx < len(masked); idx++ {
		switch masked[idx] {
		case '{', '}':
			start = idx + 1
		case ';':
			collectCPPFunctionSymbol(symbols, masked[start:idx+1], start, masked)
			start = idx + 1
		}
	}
	return symbols
}

func collectCPPFunctionSymbol(symbols map[string][]cppSymbol, statement string, offset int, source string) {
	statement = strings.TrimSpace(statement)
	match := cppFunctionPattern.FindStringSubmatchIndex(statement)
	if match == nil {
		return
	}
	name := statement[match[2]:match[3]]
	switch name {
	case "if", "for", "while", "switch", "catch", "static_assert":
		return
	}
	prefix := strings.TrimSpace(statement[:match[2]])
	prefix = strings.TrimSpace(strings.TrimPrefix(prefix, "public:"))
	prefix = strings.TrimSpace(strings.TrimPrefix(prefix, "protected:"))
	prefix = strings.TrimSpace(strings.TrimPrefix(prefix, "private:"))
	params := canonicalCPPParams(statement[match[4]:match[5]])
	suffix := strings.Join(strings.Fields(statement[match[6]:match[7]]), " ")
	signature := strings.Join(strings.Fields(prefix), " ") + "(" + params + ")"
	if suffix != "" {
		signature += " " + suffix
	}
	lineOffset := offset + strings.Index(source[offset:], name)
	appendCPPSymbol(symbols, "function "+name, cppSymbol{signature: strings.TrimSpace(signature), line: support.LineNumberForOffset(source, lineOffset)})
}

func canonicalCPPParams(params string) string {
	parts := strings.Split(params, ",")
	for idx, part := range parts {
		part = strings.TrimSpace(part)
		if equal := strings.Index(part, "="); equal >= 0 {
			part = strings.TrimSpace(part[:equal])
		}
		if match := cppParamNamePattern.FindStringSubmatch(part); match != nil {
			candidate := strings.TrimSpace(match[1])
			if candidate != "const" && candidate != "volatile" {
				part = candidate + strings.TrimSpace(match[3])
			}
		}
		parts[idx] = strings.Join(strings.Fields(part), " ")
	}
	return strings.Join(parts, ", ")
}

func appendCPPSymbol(symbols map[string][]cppSymbol, key string, symbol cppSymbol) {
	for _, existing := range symbols[key] {
		if existing.signature == symbol.signature {
			return
		}
	}
	symbols[key] = append(symbols[key], symbol)
}
