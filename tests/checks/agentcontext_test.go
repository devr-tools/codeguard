package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestAgentContextWarnsWhenAgentDocsMissing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n\nA fixture repo.\n")

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "agent-docs-missing"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "warn")
	assertFindingRulePresent(t, report, "Agent Context", "context.agent-docs-missing")
	if messages := agentContextRuleMessages(report, "context.agent-docs-missing"); len(messages) != 1 {
		t.Fatalf("agent-docs-missing findings = %d, want 1", len(messages))
	}
}

func TestAgentContextPassesOnLegibleRepo(t *testing.T) {
	dir := t.TempDir()
	writeLegibleRepoFixture(t, dir)

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "legible-repo"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "pass")
	for _, section := range report.Sections {
		if section.ID == "context" && len(section.Findings) != 0 {
			t.Fatalf("expected no findings, got %+v", section.Findings)
		}
	}
}

// writeLegibleRepoFixture builds a repo whose docs are accurate and salted
// with the reference shapes the drift rules must NOT flag: URLs, placeholders,
// env vars, globs, module paths, output fences, cd-scoped commands, and
// make invocations that select another makefile.
func writeLegibleRepoFixture(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "scripts", "setup.sh"), "#!/bin/sh\necho ok\n")
	writeFile(t, filepath.Join(dir, "Makefile"), ".PHONY: build test\nbuild:\n\techo build\ntest:\n\techo test\n")
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"fixture","scripts":{"lint":"echo lint"}}`)
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), strings.Join([]string{
		"# CLAUDE.md",
		"",
		"Build with `make build` and lint with `npm run lint`.",
		"The entrypoint is `main.go` and setup lives in `scripts/setup.sh`.",
		"See https://example.com/missing/page.md and the module github.com/acme/tool/cmd.",
		"Use `<owner>/<repo>` and $HOME/config/settings.json as placeholders.",
		"Glob patterns like src/**/*.ts are ignored.",
		"",
		"```text",
		"./scripts/from-captured-output.sh",
		"make imaginary-target",
		"```",
		"",
		"```bash",
		"make -C other-repo deploy",
		"cd sub && ./missing-after-cd.sh",
		"cat <<EOF",
		"./scripts/inside-heredoc.sh",
		"EOF",
		"```",
	}, "\n")+"\n")
	writeFile(t, filepath.Join(dir, "README.md"), strings.Join([]string{
		"# fixture",
		"",
		"```bash",
		"./scripts/setup.sh",
		"make build && make test",
		"npm run lint",
		"```",
	}, "\n")+"\n")
}

func TestAgentContextFlagsAgentDocsDrift(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n")
	writeFile(t, filepath.Join(dir, "Makefile"), "build:\n\techo build\n")
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"fixture","scripts":{"build":"echo build"}}`)
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), strings.Join([]string{
		"# CLAUDE.md",
		"",
		"Edit `internal/server/router.go` before running `make deploy`.",
		"Lint with `npm run lint`.",
	}, "\n")+"\n")

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "agent-docs-drift"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "warn")
	messages := agentContextRuleMessages(report, "context.agent-docs-drift")
	if len(messages) != 3 {
		t.Fatalf("agent-docs-drift findings = %d, want 3: %v", len(messages), messages)
	}
	joined := strings.Join(messages, "\n")
	for _, needle := range []string{"internal/server/router.go", `make target "deploy"`, `npm script "lint"`} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("drift messages missing %q: %v", needle, messages)
		}
	}
	assertFindingRuleAbsent(t, report, "Agent Context", "context.agent-docs-missing")
}

func TestAgentContextFlagsReadmeDriftInProseAndShellFences(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "Makefile"), "build:\n\techo build\n")
	writeFile(t, filepath.Join(dir, "README.md"), strings.Join([]string{
		"# fixture",
		"",
		"Prose mention of `./scripts/not-checked-in-prose.sh` gets the same scrutiny as agent docs.",
		"Entry point is `cmd/missing/main.go`.",
		"",
		"```bash",
		"./scripts/setup.sh",
		"make bootstrap",
		"```",
		"",
		"```text",
		"./scripts/example-output.sh",
		"```",
	}, "\n")+"\n")

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "readme-drift"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "warn")
	messages := agentContextRuleMessages(report, "context.readme-drift")
	if len(messages) != 4 {
		t.Fatalf("readme-drift findings = %d, want 4: %v", len(messages), messages)
	}
	joined := strings.Join(messages, "\n")
	for _, needle := range []string{"./scripts/setup.sh", `make target "bootstrap"`, "not-checked-in-prose", "cmd/missing/main.go"} {
		if !strings.Contains(joined, needle) {
			t.Fatalf("readme drift messages missing %q: %v", needle, messages)
		}
	}
	if strings.Contains(joined, "example-output") {
		t.Fatalf("readme drift flagged references inside a non-shell fence: %v", messages)
	}
}

func TestAgentContextFlagsOversizedContextUnit(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# CLAUDE.md\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n")
	writeFile(t, filepath.Join(dir, "big.go"), goFileWithLines(40))
	writeFile(t, filepath.Join(dir, "generated.go"), "// Code generated by fixturegen. DO NOT EDIT.\n"+goFileWithLines(40))
	writeFile(t, filepath.Join(dir, "vendor", "dep", "huge.go"), goFileWithLines(60))

	cfg := agentContextTestConfig(dir, "oversized-unit")
	cfg.Checks.ContextRules.MaxFileLines = 30

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "warn")
	messages := agentContextRuleMessages(report, "context.oversized-context-unit")
	if len(messages) != 1 {
		t.Fatalf("oversized findings = %d, want 1 (generated and vendored files must be skipped): %v", len(messages), messages)
	}
	if !strings.Contains(messages[0], "30-line agent context budget") {
		t.Fatalf("unexpected oversized message: %q", messages[0])
	}
}

func goFileWithLines(lines int) string {
	var b strings.Builder
	b.WriteString("package fixture\n")
	for i := 0; i < lines; i++ {
		b.WriteString("// filler line to inflate the fixture beyond the context budget\n")
	}
	return b.String()
}

func TestAgentContextFlagsAmbiguousBasenames(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# CLAUDE.md\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n")
	for _, sub := range []string{"api", "web", "cli", "db", "auth", "billing", "search"} {
		writeFile(t, filepath.Join(dir, sub, "utils.ts"), "export const ns = \""+sub+"\";\n")
	}

	report, err := codeguard.Run(context.Background(), agentContextTestConfig(dir, "ambiguous-basenames"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "warn")
	messages := agentContextRuleMessages(report, "context.ambiguous-symbol")
	if len(messages) != 1 {
		t.Fatalf("ambiguous-symbol findings = %d, want 1: %v", len(messages), messages)
	}
	message := messages[0]
	if !strings.Contains(message, `7 files share the basename "utils.ts"`) || !strings.Contains(message, "and 2 more") {
		t.Fatalf("unexpected ambiguous-symbol message: %q", message)
	}
}

func TestAgentContextAmbiguousThresholdConfigurable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# CLAUDE.md\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n")
	for _, sub := range []string{"api", "web", "cli"} {
		writeFile(t, filepath.Join(dir, sub, "utils.ts"), "export const ns = \""+sub+"\";\n")
	}

	cfg := agentContextTestConfig(dir, "ambiguous-threshold")
	cfg.Checks.ContextRules.AmbiguousSymbolThreshold = 3

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFindingRulePresent(t, report, "Agent Context", "context.ambiguous-symbol")
}

func TestAgentContextRuleTogglesDisableFindings(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "README.md"), "# demo\n\n```bash\n./scripts/gone.sh\n```\n")
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "# CLAUDE.md\n\nEdit `internal/gone/file.go`.\n")

	off := false
	cfg := agentContextTestConfig(dir, "toggles-off")
	cfg.Checks.ContextRules.DetectAgentDocsDrift = &off
	cfg.Checks.ContextRules.DetectReadmeDrift = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	assertSectionStatus(t, report, "Agent Context", "pass")
	assertFindingRuleAbsent(t, report, "Agent Context", "context.agent-docs-drift")
	assertFindingRuleAbsent(t, report, "Agent Context", "context.readme-drift")
}

func TestAgentContextSkippedInDiffScansByDefault(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "CodeGuard Test")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() { _ = 1 }\n")

	report, err := codeguard.RunWithOptions(context.Background(), agentContextTestConfig(dir, "context-diff-default"), codeguard.ScanOptions{
		Mode:    codeguard.ScanModeDiff,
		BaseRef: "main",
	})
	if err != nil {
		t.Fatalf("run diff: %v", err)
	}

	for _, section := range report.Sections {
		if section.ID == "context" {
			t.Fatal("context section should default off in diff scans; repo-level findings would repeat on every PR")
		}
	}
}

func TestAgentContextSectionCanBeDisabled(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")

	off := false
	cfg := agentContextTestConfig(dir, "context-disabled")
	cfg.Checks.Context = &off

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, section := range report.Sections {
		if section.ID == "context" {
			t.Fatal("context section should not run when checks.context is false")
		}
	}
	if len(report.Artifacts) != 0 {
		t.Fatalf("expected no artifacts when the section is disabled, got %d", len(report.Artifacts))
	}
}
