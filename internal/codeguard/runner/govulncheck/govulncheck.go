package govulncheck

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

func Run(ctx context.Context, dir string, cmdName string, sc runnersupport.Context) ([]core.Finding, error) {
	if strings.TrimSpace(cmdName) == "" {
		cmdName = "govulncheck"
	}
	cmd := exec.CommandContext(ctx, cmdName, "./...")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	text := string(output)
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
