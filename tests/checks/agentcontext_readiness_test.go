package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestAgentContextFlagsUndocumentedCommands(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	// High-signal targets: build, test, fmt. internal-sync is not high-signal.
	writeFile(t, filepath.Join(dir, "Makefile"),
		".PHONY: build test fmt internal-sync\nbuild:\n\techo build\ntest:\n\techo test\nfmt:\n\techo fmt\ninternal-sync:\n\techo sync\n")
	// High-signal scripts: lint, dev.
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"fixture","scripts":{"lint":"echo lint","dev":"echo dev"}}`)
	// make build documented via inline code, make test via plain prose (no
	// backticks), npm run lint via a README shell fence. make fmt and the dev
	// script are documented nowhere.
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), strings.Join([]string{
		"# CLAUDE.md",
		"",
		"Build with `make build`.",
		"Before pushing, run make test to execute the suite.",
	}, "\n")+"\n")
	writeFile(t, filepath.Join(dir, "README.md"), strings.Join([]string{
		"# fixture",
		"",
		"```bash",
		"npm run lint",
		"```",
	}, "\n")+"\n")

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "undocumented-commands"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "warn")
	messages := agentContextRuleMessages(report, "context.undocumented-commands")
	if len(messages) != 2 {
		t.Fatalf("undocumented-commands findings = %d, want 2: %v", len(messages), messages)
	}
	joined := strings.Join(messages, "\n")
	if !strings.Contains(joined, `"make fmt"`) || !strings.Contains(joined, `"npm run dev"`) {
		t.Fatalf("unexpected undocumented-commands messages: %v", messages)
	}
	if strings.Contains(joined, "internal-sync") {
		t.Fatalf("non-high-signal target must not demand documentation: %v", messages)
	}
}

func TestAgentContextUndocumentedCommandsExactNameMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "Makefile"), "fmt:\n\techo fmt\nfmt-check:\n\techo fmt-check\n")
	// The doc mentions make fmt-check, which must NOT count as a mention of
	// make fmt; fmt-check itself is not on the high-signal allowlist.
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# CLAUDE.md\n\nRun `make fmt-check` in CI.\n")

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "undocumented-exact-match"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	messages := agentContextRuleMessages(report, "context.undocumented-commands")
	if len(messages) != 1 || !strings.Contains(messages[0], `"make fmt"`) {
		t.Fatalf("undocumented-commands findings = %v, want exactly the make fmt finding", messages)
	}
}

func TestAgentContextUndocumentedCommandsSilentWithoutAgentDocs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "Makefile"), "build:\n\techo build\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# fixture\n")

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "undocumented-no-agent-docs"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Agent Context", "context.agent-docs-missing")
	assertFindingRuleAbsent(t, report, "Agent Context", "context.undocumented-commands")
}

func TestAgentContextUndocumentedCommandsToggleOff(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "Makefile"), "build:\n\techo build\n")
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# CLAUDE.md\n")

	off := false
	cfg := agentContextTestConfig(dir, "undocumented-toggle-off")
	cfg.Checks.ContextRules.DetectUndocumentedCommands = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Agent Context", "context.undocumented-commands")
}

func TestAgentContextFlagsOversizedAgentDoc(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), markdownFileWithLines(40))
	writeFile(t, filepath.Join(dir, "AGENTS.md"), markdownFileWithLines(10))
	// A long README is reference material, not an agent doc; it must not be
	// measured against the agent-doc budget.
	writeFile(t, filepath.Join(dir, "README.md"), markdownFileWithLines(80))

	cfg := agentContextTestConfig(dir, "oversized-agent-doc")
	cfg.Checks.ContextRules.MaxAgentDocLines = 30

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "warn")
	messages := agentContextRuleMessages(report, "context.oversized-agent-doc")
	if len(messages) != 1 {
		t.Fatalf("oversized-agent-doc findings = %d, want 1 (only CLAUDE.md is over budget): %v", len(messages), messages)
	}
	if !strings.Contains(messages[0], "30-line agent doc budget") {
		t.Fatalf("unexpected oversized-agent-doc message: %q", messages[0])
	}
}

func TestAgentContextOversizedAgentDocToggleOff(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), markdownFileWithLines(40))
	writeFile(t, filepath.Join(dir, "README.md"), "# fixture\n")

	off := false
	cfg := agentContextTestConfig(dir, "oversized-agent-doc-off")
	cfg.Checks.ContextRules.MaxAgentDocLines = 30
	cfg.Checks.ContextRules.DetectOversizedAgentDocs = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "pass")
	assertFindingRuleAbsent(t, report, "Agent Context", "context.oversized-agent-doc")
}

func markdownFileWithLines(lines int) string {
	var b strings.Builder
	b.WriteString("# doc\n")
	for i := 0; i < lines; i++ {
		b.WriteString("Plain filler prose with no repository references at all.\n")
	}
	return b.String()
}

func TestAgentContextFlagsDocLinkRot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "docs", "real.md"), "# real\n")
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), strings.Join([]string{
		"# CLAUDE.md",
		"",
		"- [existing doc](docs/real.md)",
		"- [existing with anchor](docs/real.md#usage)",
		"- [missing doc](docs/missing-guide.md)",
		"- [missing with anchor](docs/gone.md#setup)",
		"- [pure anchor](#conventions)",
		"- [external](https://example.com/missing/page.md)",
		"- [templated](docs/<topic>.md)",
		"",
		"```markdown",
		"- [inside a fence](docs/fenced-away.md)",
		"```",
	}, "\n")+"\n")
	writeFile(t, filepath.Join(dir, "README.md"), strings.Join([]string{
		"# fixture",
		"",
		"- [abs rot](/Users/nobody/code/thing.md)",
		"- [abs ok](/docs/real.md)",
	}, "\n")+"\n")

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "doc-link-rot"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "warn")
	messages := agentContextRuleMessages(report, "context.doc-link-rot")
	if len(messages) != 3 {
		t.Fatalf("doc-link-rot findings = %d, want 3: %v", len(messages), messages)
	}
	joined := strings.Join(messages, "\n")
	for _, needle := range []string{"docs/missing-guide.md", "docs/gone.md#setup", "/Users/nobody/code/thing.md"} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("doc-link-rot messages missing %q: %v", needle, messages)
		}
	}
	for _, absent := range []string{"docs/real.md#usage", "#conventions", "example.com", "<topic>", "fenced-away"} {
		if strings.Contains(joined, absent) {
			t.Fatalf("doc-link-rot flagged an exempt link %q: %v", absent, messages)
		}
	}
}

func TestAgentContextDocLinkRotResolvesRelativeToDocDirectory(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, ".github", "workflows-guide.md"), "# guide\n")
	// copilot-instructions.md lives in .github/, so a sibling link must
	// resolve against the doc's own directory.
	writeFile(t, filepath.Join(dir, ".github", "copilot-instructions.md"),
		"# instructions\n\nSee [the guide](workflows-guide.md).\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# fixture\n")

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "doc-link-rot-docdir"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRuleAbsent(t, report, "Agent Context", "context.doc-link-rot")
}

func TestAgentContextDocLinkRotToggleOff(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# CLAUDE.md\n\n[gone](docs/gone.md)\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# fixture\n")

	off := false
	cfg := agentContextTestConfig(dir, "doc-link-rot-off")
	cfg.Checks.ContextRules.DetectDocLinkRot = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "pass")
	assertFindingRuleAbsent(t, report, "Agent Context", "context.doc-link-rot")
}
