package govulncheck

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// limitedWriter writes to w until remaining bytes are exhausted, then silently
// drops the rest while recording that truncation occurred. It lets a single
// writer back both cmd.Stdout and cmd.Stderr under a shared byte budget.
type limitedWriter struct {
	w         *bytes.Buffer
	remaining int
	truncated bool
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.remaining <= 0 {
		l.truncated = true
		return len(p), nil
	}
	if len(p) > l.remaining {
		l.w.Write(p[:l.remaining])
		l.remaining = 0
		l.truncated = true
		return len(p), nil
	}
	n, err := l.w.Write(p)
	l.remaining -= n
	return n, err
}

// maxOutputBytes caps how much govulncheck output is buffered so a runaway or
// malicious tool cannot exhaust memory.
const maxOutputBytes = 64 << 20 // 64 MiB

func Run(ctx context.Context, dir string, cmdName string, sc runnersupport.Context) ([]core.Finding, error) {
	if strings.TrimSpace(cmdName) == "" {
		cmdName = "govulncheck"
	}
	cmd := exec.CommandContext(ctx, cmdName, "./...")
	cmd.Dir = dir
	var buf bytes.Buffer
	limited := &limitedWriter{w: &buf, remaining: maxOutputBytes}
	cmd.Stdout = limited
	cmd.Stderr = limited
	err := cmd.Run()
	if limited.truncated {
		return nil, fmt.Errorf("govulncheck output exceeded %d bytes", maxOutputBytes)
	}
	text := buf.String()
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
