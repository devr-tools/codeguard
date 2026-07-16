package govulncheck

import (
	"context"
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
	"github.com/devr-tools/codeguard/internal/codeguard/trust"
)

// defaultCommand is the built-in govulncheck binary name. It is resolved from
// PATH (never the working directory) and is a static analyzer that does not
// execute the code it scans, so it is safe to run unguarded against untrusted
// repositories. Any other command name comes from repository configuration and
// must pass the command-trust gate.
const defaultCommand = "govulncheck"

// maxOutputBytes caps how much govulncheck output is buffered so a runaway or
// malicious tool cannot exhaust memory.
const maxOutputBytes = 64 << 20 // 64 MiB

func Run(ctx context.Context, dir string, cmdName string, sc runnersupport.Context) ([]core.Finding, error) {
	cmdName = strings.TrimSpace(cmdName)
	if cmdName == "" {
		cmdName = defaultCommand
	}
	// A config-supplied override of the govulncheck binary is untrusted (the repo
	// under scan may be an untrusted pull request) and must pass the command-trust
	// gate. The built-in default is exempt so the default "auto" mode keeps working.
	if cmdName != defaultCommand {
		if err := trust.GuardConfigCommand("govulncheck_command", cmdName); err != nil {
			return nil, err
		}
	}
	text, err := runnersupport.RunLimitedCommand(ctx, dir, maxOutputBytes, cmdName, "./...")
	parsed := parseOutput(text, sc)
	if len(parsed) > 0 {
		return parsed, nil
	}
	if err != nil {
		return nil, fmt.Errorf("govulncheck integration failed: %w", err)
	}
	return nil, nil
}

func parseOutput(output string, sc runnersupport.Context) []core.Finding {
	lines := strings.Split(output, "\n")
	findings := make([]core.Finding, 0)
	current := ""
	foundIn := ""
	fixedIn := ""
	flush := func() {
		if current == "" {
			return
		}
		message := current
		if foundIn != "" {
			message += " found in " + foundIn
		}
		if fixedIn != "" {
			message += " fixed in " + fixedIn
		}
		findings = append(findings, runnersupport.NewFinding(sc, runnersupport.FindingInput{
			RuleID:  "security.govulncheck",
			Level:   "fail",
			Message: message,
		}))
		current, foundIn, fixedIn = "", "", ""
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Vulnerability #"):
			flush()
			current = line
		case strings.HasPrefix(line, "Found in:"):
			foundIn = strings.TrimSpace(strings.TrimPrefix(line, "Found in:"))
		case strings.HasPrefix(line, "Fixed in:"):
			fixedIn = strings.TrimSpace(strings.TrimPrefix(line, "Fixed in:"))
		case line == "":
			flush()
		}
	}
	flush()
	return findings
}
