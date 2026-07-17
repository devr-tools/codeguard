package design

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
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

func closeRustBlock(scan *rustFileScan) {
	if scan.active == nil || scan.active.waiting || scan.depth >= scan.active.bodyDepth {
		return
	}
	scan.findings = append(scan.findings, finalizeRustBlock(scan.env, scan.file, *scan.active, scan.methodCounts)...)
	scan.active = nil
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
