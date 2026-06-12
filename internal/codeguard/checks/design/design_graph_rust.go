package design

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

var (
	rustUsePattern = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?use\s+(.+?);`)
	rustModPattern = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?mod\s+([A-Za-z_]\w*)\s*;`)
)

func buildRustImportGraph(env support.Context, target core.TargetConfig) *moduleGraph {
	graph := newModuleGraph("rust")
	pending := make([]pendingGraphEdge, 0)
	env.VisitTargetFiles(target, func(rel string) bool {
		return strings.HasSuffix(rel, ".rs")
	}, func(rel string, data []byte) {
		module := rustModulePath(rel)
		graph.addModule(module, rel)
		pending = append(pending, rustImportEdges(module, string(data))...)
	})
	for _, edge := range pending {
		resolved := resolveRustImport(graph, edge.to)
		if resolved != "" && resolved != edge.from {
			graph.addEdge(edge.from, resolved, edge.line)
		}
	}
	return graph
}

// rustModulePath maps a file path to its crate module path, for example
// src/io/reader.rs -> crate::io::reader and src/io/mod.rs -> crate::io.
func rustModulePath(rel string) string {
	trimmed := strings.TrimSuffix(filepath.ToSlash(rel), ".rs")
	trimmed = strings.TrimPrefix(trimmed, "src/")
	parts := strings.Split(trimmed, "/")
	if parts[len(parts)-1] == "mod" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 || (len(parts) == 1 && (parts[0] == "main" || parts[0] == "lib")) {
		return "crate"
	}
	return "crate::" + strings.Join(parts, "::")
}

func rustImportEdges(module string, source string) []pendingGraphEdge {
	edges := make([]pendingGraphEdge, 0)
	for idx, line := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		lineNo := idx + 1
		if match := rustModPattern.FindStringSubmatch(line); len(match) == 2 {
			edges = append(edges, pendingGraphEdge{from: module, to: module + "::" + match[1], line: lineNo})
			continue
		}
		match := rustUsePattern.FindStringSubmatch(line)
		if len(match) != 2 {
			continue
		}
		for _, used := range expandRustUseClause(strings.TrimSpace(match[1])) {
			absolute := resolveRustUsePath(module, used)
			if absolute == "" {
				continue
			}
			edges = append(edges, pendingGraphEdge{from: module, to: absolute, line: lineNo})
		}
	}
	return edges
}

// expandRustUseClause flattens grouped imports such as crate::a::{b, c::d}
// into crate::a::b and crate::a::c::d.
func expandRustUseClause(clause string) []string {
	open := strings.Index(clause, "{")
	if open < 0 {
		return []string{strings.TrimSpace(clause)}
	}
	close := strings.LastIndex(clause, "}")
	if close < open {
		return nil
	}
	prefix := strings.TrimSuffix(strings.TrimSpace(clause[:open]), "::")
	expanded := make([]string, 0)
	for _, part := range splitRustGroupItems(clause[open+1 : close]) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for _, nested := range expandRustUseClause(part) {
			if nested == "self" {
				expanded = append(expanded, prefix)
				continue
			}
			expanded = append(expanded, prefix+"::"+nested)
		}
	}
	return expanded
}

func splitRustGroupItems(group string) []string {
	items := make([]string, 0)
	depth := 0
	start := 0
	for idx, char := range group {
		switch char {
		case '{':
			depth++
		case '}':
			depth--
		case ',':
			if depth == 0 {
				items = append(items, group[start:idx])
				start = idx + 1
			}
		}
	}
	return append(items, group[start:])
}

// resolveRustUsePath converts crate/self/super-relative use paths to absolute
// crate paths; external crate imports return an empty string.
func resolveRustUsePath(module string, used string) string {
	used = strings.TrimSpace(strings.Split(used, " as ")[0])
	switch {
	case used == "crate" || strings.HasPrefix(used, "crate::"):
		return used
	case used == "self" || strings.HasPrefix(used, "self::"):
		return module + strings.TrimPrefix(used, "self")
	case used == "super" || strings.HasPrefix(used, "super::"):
		base := module
		for used == "super" || strings.HasPrefix(used, "super::") {
			base = rustParentModule(base)
			used = strings.TrimPrefix(strings.TrimPrefix(used, "super"), "::")
		}
		if used == "" {
			return base
		}
		return base + "::" + used
	default:
		return ""
	}
}

func rustParentModule(module string) string {
	if cut := strings.LastIndex(module, "::"); cut >= 0 {
		return module[:cut]
	}
	return "crate"
}

// resolveRustImport finds the longest known module prefix of an absolute use
// path, so crate::io::reader::Reader resolves to crate::io::reader.
func resolveRustImport(graph *moduleGraph, used string) string {
	for current := used; current != ""; current = rustImportPrefix(current) {
		if _, ok := graph.modules[current]; ok {
			return current
		}
	}
	return ""
}

func rustImportPrefix(used string) string {
	if cut := strings.LastIndex(used, "::"); cut >= 0 {
		return used[:cut]
	}
	return ""
}
