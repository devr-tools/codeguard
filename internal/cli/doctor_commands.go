package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
	"github.com/devr-tools/codeguard/internal/codeguard/trust"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func govulncheckDoctorCheck(cfg service.Config) (doctorCheck, bool) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Checks.SecurityRules.GovulncheckMode))
	goTarget, hasGo := firstGoTarget(cfg.Targets)
	if !cfg.Checks.Security || mode == "off" || !hasGo {
		return doctorCheck{}, false
	}
	problem := warnDoctorCheck
	if mode == "required" {
		problem = failDoctorCheck
	}

	command := strings.TrimSpace(cfg.Checks.SecurityRules.GovulncheckCommand)
	if command == "" {
		command = defaultGovulncheckCommand
	}
	if _, err := exec.LookPath(command); err != nil {
		return problem("govulncheck", fmt.Sprintf("%s is not available on PATH", command)), true
	}
	// A config-supplied command is untrusted until the operator opts in; doctor
	// must not run what a scan would refuse to run.
	if command != defaultGovulncheckCommand {
		if err := trust.GuardConfigCommand("govulncheck_command", command); err != nil {
			return warnDoctorCheck("govulncheck", fmt.Sprintf("%s is on PATH but was not run: %v", command, err)), true
		}
	}
	return probeGovulncheck(command, goTarget.Path, problem), true
}

// defaultGovulncheckCommand mirrors the scanner's built-in binary name.
const defaultGovulncheckCommand = "govulncheck"

// govulncheckProbeTimeout bounds the `-version` probe; it normally returns in
// well under a second.
const govulncheckProbeTimeout = 30 * time.Second

// probeGovulncheck runs `<command> -version` in dir. Being on PATH is not
// enough: a broken Go toolchain makes every real scan fail with a bare exit
// status, so the probe reports the tool's stderr tail on failure, and warns
// when govulncheck cannot report the Go toolchain it would scan with.
func probeGovulncheck(command string, dir string, problem func(string, string) doctorCheck) doctorCheck {
	ctx, cancel := context.WithTimeout(context.Background(), govulncheckProbeTimeout)
	defer cancel()
	output, err := runnersupport.RunLimitedCommandCapture(ctx, dir, 1<<20, command, "-version")
	if err != nil {
		message := fmt.Sprintf("%s -version failed: %v", command, err)
		if tail := strings.TrimSpace(output.StderrTail); tail != "" {
			message += "\nstderr:\n" + tail
		}
		return problem("govulncheck", message)
	}
	version := govulncheckVersionLine(output.Combined, "Scanner:")
	if govulncheckVersionLine(output.Combined, "Go:") == "" {
		return warnDoctorCheck("govulncheck", fmt.Sprintf("%s runs (%s) but reports no Go toolchain; check GOROOT/PATH, or scans will fail to load packages", command, firstNonBlank(version, "version unknown")))
	}
	return passDoctorCheck("govulncheck", fmt.Sprintf("%s runs (%s)", command, firstNonBlank(version, "version unknown")))
}

func govulncheckVersionLine(output string, prefix string) string {
	for _, line := range strings.Split(output, "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), prefix); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func languageCommandDoctorChecks(cfg service.Config) []doctorCheck {
	checks := make([]doctorCheck, 0)
	if cfg.Checks.Design {
		checks = append(checks, commandDoctorChecks("design", cfg.Checks.DesignRules.LanguageCommands, cfg.Targets)...)
	}
	if cfg.Checks.Quality {
		checks = append(checks, commandDoctorChecks("quality", cfg.Checks.QualityRules.LanguageCommands, cfg.Targets)...)
	}
	if cfg.Checks.Security {
		checks = append(checks, commandDoctorChecks("security", cfg.Checks.SecurityRules.LanguageCommands, cfg.Targets)...)
	}
	return checks
}

func commandDoctorChecks(section string, languageCommands map[string][]service.CommandCheckConfig, targets []service.TargetConfig) []doctorCheck {
	checks := make([]doctorCheck, 0)
	for _, target := range targets {
		for _, check := range languageCommands[normalizedLanguage(target.Language)] {
			name := fmt.Sprintf("%s:%s:%s", section, target.Name, check.Name)
			if _, err := resolveCommandPath(strings.TrimSpace(check.Command), target.Path); err != nil {
				checks = append(checks, failDoctorCheck(name, fmt.Sprintf("%s is not available or executable", check.Command)))
				continue
			}
			checks = append(checks, passDoctorCheck(name, fmt.Sprintf("%s is available", check.Command)))
		}
	}
	return checks
}

func firstGoTarget(targets []service.TargetConfig) (service.TargetConfig, bool) {
	for _, target := range targets {
		if normalizedLanguage(target.Language) == "go" || strings.TrimSpace(target.Language) == "" {
			return target, true
		}
	}
	return service.TargetConfig{}, false
}

func normalizedLanguage(language string) string {
	return strings.ToLower(strings.TrimSpace(language))
}

func resolveCommandPath(command string, dir string) (string, error) {
	if strings.Contains(command, string(filepath.Separator)) {
		path := command
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, command)
		}
		info, err := os.Stat(path)
		if err != nil {
			return "", err
		}
		if info.IsDir() {
			return "", fmt.Errorf("%s is a directory", path)
		}
		if info.Mode()&0o111 == 0 {
			return "", fmt.Errorf("%s is not executable", path)
		}
		return path, nil
	}
	return exec.LookPath(command)
}
