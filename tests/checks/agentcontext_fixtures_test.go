package checks_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// writeLegibleRepoFixture builds a repo whose docs are accurate and salted
// with the reference shapes the drift rules must NOT flag.
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

func goFileWithLines(lines int) string {
	var b strings.Builder
	b.WriteString("package fixture\n")
	for i := 0; i < lines; i++ {
		b.WriteString("// filler line to inflate the fixture beyond the context budget\n")
	}
	return b.String()
}
