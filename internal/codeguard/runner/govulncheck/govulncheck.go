package govulncheck

import (
	"context"
	"fmt"
	"regexp"
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

var advisoryPattern = regexp.MustCompile(`GO-[0-9]{4}-[0-9]+`)

func RunWorkspace(ctx context.Context, dir string, cmdName string, sc runnersupport.Context) ([]core.Finding, []core.Diagnostic) {
	workspace, err := DiscoverWorkspace(dir)
	if err != nil {
		return nil, []core.Diagnostic{{ID: "scan.govulncheck.workspace", Level: "fail", Kind: "infrastructure", Message: err.Error(), Operational: true}}
	}
	if len(workspace.Modules) == 0 {
		return nil, []core.Diagnostic{{ID: "scan.govulncheck.module-unavailable", Level: "fail", Kind: "infrastructure", Message: "no Go module is available for govulncheck", Path: dir, Operational: true}}
	}
	result := ScanWorkspace(ctx, workspace, ScanOptions{Execute: func(moduleCtx context.Context, module Module) ([]Vulnerability, error) {
		findings, runErr := Run(moduleCtx, module.Dir, cmdName, sc)
		vulnerabilities := make([]Vulnerability, 0, len(findings))
		for _, finding := range findings {
			id := advisoryPattern.FindString(finding.Message)
			if id == "" {
				id = finding.Message
			}
			callStack := []string(nil)
			if raw := finding.Metadata["call_stack"]; raw != "" {
				callStack = strings.Split(raw, " -> ")
			}
			vulnerabilities = append(vulnerabilities, Vulnerability{AdvisoryID: id, Package: finding.Metadata["package"], CallStack: callStack})
		}
		return vulnerabilities, runErr
	}})
	findings := make([]core.Finding, 0, len(result.Vulnerabilities))
	for _, vulnerability := range result.Vulnerabilities {
		findings = append(findings, runnersupport.NewFinding(sc, runnersupport.FindingInput{RuleID: "security.govulncheck", Level: "fail", Message: "govulncheck reported " + vulnerability.AdvisoryID, Metadata: map[string]string{"advisory_id": vulnerability.AdvisoryID, "affected_modules": occurrenceField(vulnerability.Occurrences, func(o Occurrence) string { return o.Module }), "affected_packages": occurrenceField(vulnerability.Occurrences, func(o Occurrence) string { return o.Package }), "call_stacks": occurrenceField(vulnerability.Occurrences, func(o Occurrence) string { return strings.Join(o.CallStack, " -> ") })}}))
	}
	diagnostics := make([]core.Diagnostic, 0)
	for _, module := range result.Modules {
		if module.Status == ModuleSucceeded {
			diagnostics = append(diagnostics, core.Diagnostic{ID: "scan.govulncheck.module-status", Level: "info", Kind: "scan_status", Message: "govulncheck succeeded for module " + module.Module.ModulePath, Path: module.Module.Dir, Metadata: map[string]string{"module": module.Module.ModulePath, "status": string(module.Status)}})
			continue
		}
		message := "govulncheck failed for module " + module.Module.ModulePath
		if module.Status == ModuleTimedOut {
			message = "govulncheck timed out for module " + module.Module.ModulePath
		}
		if module.Err != nil {
			message += ": " + module.Err.Error()
		}
		diagnosticID, failureKind := classifyModuleFailure(module)
		diagnostics = append(diagnostics, core.Diagnostic{ID: diagnosticID, Level: "fail", Kind: "infrastructure", Message: message, Path: module.Module.Dir, Operational: true, Evidence: []string{"failure_kind:" + failureKind}, Metadata: map[string]string{"module": module.Module.ModulePath, "status": string(module.Status), "failure_kind": failureKind}})
	}
	return findings, diagnostics
}

func classifyModuleFailure(module ModuleResult) (string, string) {
	if module.Status == ModuleTimedOut {
		return "scan.govulncheck.timeout", "timeout"
	}
	message := ""
	if module.Err != nil {
		message = strings.ToLower(module.Err.Error())
	}
	if strings.Contains(message, "no packages") || strings.Contains(message, "package") && strings.Contains(message, "unavailable") || strings.Contains(message, "module") && strings.Contains(message, "unavailable") {
		return "scan.govulncheck.module-unavailable", "module_or_package_unavailable"
	}
	return "scan.govulncheck.execution-failure", "tool_execution_failure"
}

func occurrenceField(occurrences []Occurrence, value func(Occurrence) string) string {
	values := make([]string, 0, len(occurrences))
	seen := map[string]struct{}{}
	for _, occurrence := range occurrences {
		field := value(occurrence)
		if field == "" {
			continue
		}
		if _, ok := seen[field]; ok {
			continue
		}
		seen[field] = struct{}{}
		values = append(values, field)
	}
	return strings.Join(values, ",")
}

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
	if err != nil {
		return parsed, fmt.Errorf("govulncheck integration failed: %w", err)
	}
	return parsed, nil
}

func parseOutput(output string, sc runnersupport.Context) []core.Finding {
	lines := strings.Split(output, "\n")
	findings := make([]core.Finding, 0)
	current := ""
	foundIn := ""
	fixedIn := ""
	trace := make([]string, 0)
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
		metadata := map[string]string{"advisory_id": advisoryPattern.FindString(current)}
		if foundIn != "" {
			metadata["package"] = foundIn
		}
		if len(trace) > 0 {
			metadata["call_stack"] = strings.Join(trace, " -> ")
		}
		findings = append(findings, runnersupport.NewFinding(sc, runnersupport.FindingInput{
			RuleID:   "security.govulncheck",
			Level:    "fail",
			Message:  message,
			Metadata: metadata,
		}))
		current, foundIn, fixedIn, trace = "", "", "", trace[:0]
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
		case current != "":
			trace = append(trace, line)
		}
	}
	flush()
	return findings
}
