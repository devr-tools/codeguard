package cli

import (
	"fmt"
	"io"

	"github.com/devr-tools/codeguard/internal/version"
)

type commandRunner func(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int

var commandCatalog = map[string]commandRunner{
	"baseline":       withoutStdin(runBaseline),
	"doctor":         withoutStdin(runDoctor),
	"explain":        withoutStdin(runExplain),
	"fix":            withoutStdin(runFix),
	"init":           runInit,
	"owasp":          withoutStdin(runOWASP),
	"profiles":       noArgs(runProfiles),
	"report":         withoutStdin(runReport),
	"rules":          withoutStdin(runRules),
	"scan":           runScan,
	"scan-history":   withoutStdin(runScanHistory),
	"serve":          runServe,
	"validate":       withoutStdin(runValidate),
	"validate-patch": runValidatePatch,
}

func Run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		writeWhatsNew(stdout)
		writeMenu(stdout)
		return 0
	}

	command := args[0]
	if isHelpCommand(command) {
		writeWhatsNew(stdout)
		writeMenu(stdout)
		return 0
	}
	if command == "version" {
		_, _ = fmt.Fprintln(stdout, version.Number)
		return 0
	}
	if runner, ok := commandCatalog[command]; ok {
		return runner(args[1:], stdin, stdout, stderr)
	}

	_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n", command)
	writeMenu(stderr)
	return 1
}

func isHelpCommand(command string) bool {
	switch command {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func withoutStdin(fn func([]string, io.Writer, io.Writer) int) commandRunner {
	return func(args []string, _ io.Reader, stdout io.Writer, stderr io.Writer) int {
		return fn(args, stdout, stderr)
	}
}

func noArgs(fn func(io.Writer) int) commandRunner {
	return func(_ []string, _ io.Reader, stdout io.Writer, _ io.Writer) int {
		return fn(stdout)
	}
}
