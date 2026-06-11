package cli

import (
	"fmt"
	"io"

	"github.com/devr-tools/codeguard/internal/version"
)

type commandRunner func(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int

var commandCatalog = map[string]commandRunner{
	"baseline": withoutStdin(runBaseline),
	"doctor":   withoutStdin(runDoctor),
	"explain":  withoutStdin(runExplain),
	"init":     runInit,
	"profiles": noArgs(runProfiles),
	"rules":    withoutStdin(runRules),
	"scan":     runScan,
	"validate": withoutStdin(runValidate),
}

func Run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		writeUsage(stdout)
		return 0
	}

	command := args[0]
	if isHelpCommand(command) {
		writeUsage(stdout)
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
	writeUsage(stderr)
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
