package prompts

import (
	"path"
	"strings"
)

// agentInstructionBasenames are instruction files AI coding agents load into
// their context verbatim, wherever they sit in the tree (monorepos nest them
// per package). Matching is case-insensitive.
var agentInstructionBasenames = map[string]struct{}{
	"agents.md":               {},
	"agent.md":                {},
	"agents.override.md":      {},
	"agents.local.md":         {},
	"claude.md":               {},
	"claude.local.md":         {},
	"gemini.md":               {},
	".cursorrules":            {},
	".windsurfrules":          {},
	".clinerules":             {},
	"copilot-instructions.md": {},
}

// agentInstructionDirs are directories whose markdown files are agent
// instructions: subagent definitions, slash commands, rule packs. A file
// matches when its path contains the directory and ends in one of the listed
// extensions.
var agentInstructionDirs = []struct {
	dir  string
	exts []string
}{
	{"/.claude/agents/", []string{".md"}},
	{"/.claude/commands/", []string{".md"}},
	{"/.cursor/rules/", []string{".mdc", ".md"}},
	{"/.windsurf/rules/", []string{".md"}},
	{"/.clinerules/", []string{".md"}},
	{"/.github/instructions/", []string{".instructions.md"}},
	{"/.github/prompts/", []string{".prompt.md"}},
	{"/.github/chatmodes/", []string{".chatmode.md"}},
	{"/.gemini/commands/", []string{".toml", ".md"}},
}

// claudeSettingsBasenames are Claude Code's checked-in permission files. They
// decide what the agent may run without asking, so a wildcard there is a
// standing grant to everyone who opens the repository.
var claudeSettingsBasenames = map[string]struct{}{
	"settings.json":         {},
	"settings.local.json":   {},
	"managed-settings.json": {},
}

// isAgentConfigFile reports whether rel is any agent configuration file the
// prompts section governs: an instruction document or a settings file.
func isAgentConfigFile(rel string) bool {
	return isAgentInstructionFile(rel) || isClaudeSettingsFile(rel)
}

// isAgentInstructionFile reports whether rel is natural-language guidance an
// agent reads as instructions, as opposed to a structured settings file.
func isAgentInstructionFile(rel string) bool {
	lower := "/" + strings.TrimPrefix(strings.ToLower(normalizeRel(rel)), "/")
	base := path.Base(lower)
	if _, ok := agentInstructionBasenames[base]; ok {
		return base != "copilot-instructions.md" || strings.HasSuffix(lower, "/.github/copilot-instructions.md")
	}
	if base == "skill.md" && strings.Contains(lower, "/.claude/skills/") {
		return true
	}
	for _, entry := range agentInstructionDirs {
		if !strings.Contains(lower, entry.dir) {
			continue
		}
		for _, ext := range entry.exts {
			if strings.HasSuffix(base, ext) {
				return true
			}
		}
	}
	return false
}

func isClaudeSettingsFile(rel string) bool {
	lower := strings.ToLower(normalizeRel(rel))
	if path.Base(path.Dir("/"+lower)) != ".claude" {
		return false
	}
	_, ok := claudeSettingsBasenames[path.Base(lower)]
	return ok
}

func normalizeRel(rel string) string {
	return strings.ReplaceAll(rel, "\\", "/")
}
