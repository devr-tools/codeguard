package codeguard

import (
	"context"

	"github.com/devr-tools/codeguard/internal/codeguard/contextgen"
)

// ContextFacts is what codeguard detected about a repository for an agent
// instruction file: languages, commands, layout, entrypoints, tests and
// configured tooling.
type ContextFacts = contextgen.Facts

// ContextCommand is one detected command and the file that proves it.
type ContextCommand = contextgen.Command

// ContextDocUpdate is the outcome of refreshing an agent instruction file's
// codeguard-managed blocks.
type ContextDocUpdate = contextgen.UpdateResult

// ContextCheckResult is the agent-context section's verdict on a repository:
// the repo_legibility score and every agent-context finding.
type ContextCheckResult = contextgen.CheckResult

// ErrContextNoMarkers is returned by UpdateContextDoc for a file without
// codeguard:begin/end markers, which it never rewrites.
var ErrContextNoMarkers = contextgen.ErrNoMarkers

// DetectContextFacts inspects the repository at root.
func DetectContextFacts(root string) (ContextFacts, error) {
	return contextgen.Detect(root)
}

// RenderContextDoc renders a complete AGENTS.md / CLAUDE.md from facts. Every
// line is a detected fact or an explicit TODO; detected content sits inside
// codeguard-managed blocks that UpdateContextDoc can refresh.
func RenderContextDoc(facts ContextFacts) string {
	return contextgen.Render(facts)
}

// UpdateContextDoc refreshes the managed blocks in existing from facts,
// leaving every byte outside the block bodies untouched. It returns
// ErrContextNoMarkers when existing has no managed blocks.
func UpdateContextDoc(existing string, facts ContextFacts) (ContextDocUpdate, error) {
	return contextgen.Update(existing, facts)
}

// ProposeContextSections returns existing followed by every managed section:
// what a file without markers would gain. Callers show it as a diff rather
// than writing it.
func ProposeContextSections(existing string, facts ContextFacts) string {
	return contextgen.Proposal(existing, facts)
}

// ContextDocDiff renders a unified diff between two versions of the file at
// path, or "" when they are equal.
func ContextDocDiff(path string, oldText string, newText string) string {
	return contextgen.UnifiedDiff(path, oldText, newText)
}

// ContextBlockNames returns the codeguard-managed block names in render
// order. Each block is delimited by "<!-- codeguard:begin NAME -->" and
// "<!-- codeguard:end NAME -->".
func ContextBlockNames() []string {
	return contextgen.BlockNames()
}

// CheckContextDocs runs only the agent-context section against root; rules
// tunes it (the zero value means the built-in defaults).
func CheckContextDocs(ctx context.Context, root string, rules ContextRulesConfig) (ContextCheckResult, error) {
	return contextgen.Check(ctx, root, rules)
}
