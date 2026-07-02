package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func runDoctor(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config path")
	profile := fs.String("profile", "", "optional policy profile override")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg, checks, ok := loadDoctorConfig(*configPath, *profile)
	writeDoctorChecks(&checks, cfg)
	writeDoctorReport(stdout, checks)
	if !ok || hasDoctorFailures(checks) {
		return 1
	}
	return 0
}

func loadDoctorConfig(configPath string, profile string) (service.Config, []doctorCheck, bool) {
	if _, err := os.Stat(configPath); err != nil {
		return service.Config{}, []doctorCheck{failDoctorCheck("config", fmt.Sprintf("config file %s is missing", configPath))}, false
	}

	cfg, err := loadConfigWithProfile(configPath, profile)
	if err != nil {
		return service.Config{}, []doctorCheck{failDoctorCheck("config", err.Error())}, false
	}

	return cfg, []doctorCheck{passDoctorCheck("config", "config loads and validates")}, true
}

func writeDoctorChecks(checks *[]doctorCheck, cfg service.Config) {
	*checks = append(*checks, gitDoctorCheck())
	*checks = append(*checks, targetDoctorChecks(cfg.Targets)...)
	*checks = append(*checks, languageCommandDoctorChecks(cfg)...)
	if govulncheck, ok := govulncheckDoctorCheck(cfg); ok {
		*checks = append(*checks, govulncheck)
	}
	if baseline, ok := baselineDoctorCheck(cfg); ok {
		*checks = append(*checks, baseline)
	}
	if cache, ok := cacheDoctorCheck(cfg); ok {
		*checks = append(*checks, cache)
	}
	*checks = append(*checks, ruleHealthDoctorChecks(cfg, time.Now())...)
}
