package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func govulncheckDoctorCheck(cfg service.Config) (doctorCheck, bool) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Checks.SecurityRules.GovulncheckMode))
	if !cfg.Checks.Security || mode == "off" || !hasGoTarget(cfg.Targets) {
		return doctorCheck{}, false
	}

	command := strings.TrimSpace(cfg.Checks.SecurityRules.GovulncheckCommand)
	if command == "" {
		command = "govulncheck"
	}
	if _, err := exec.LookPath(command); err != nil {
		if mode == "required" {
			return failDoctorCheck("govulncheck", fmt.Sprintf("%s is not available on PATH", command)), true
		}
		return warnDoctorCheck("govulncheck", fmt.Sprintf("%s is not available on PATH", command)), true
	}
	return passDoctorCheck("govulncheck", fmt.Sprintf("%s is available", command)), true
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

func hasGoTarget(targets []service.TargetConfig) bool {
	for _, target := range targets {
		if normalizedLanguage(target.Language) == "go" || strings.TrimSpace(target.Language) == "" {
			return true
		}
	}
	return false
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
