package cli

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/version"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

func Run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		writeUsage(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		writeUsage(stdout)
		return 0
	case "version":
		_, _ = fmt.Fprintln(stdout, version.Number)
		return 0
	case "init":
		return runInit(args[1:], stdin, stdout, stderr)
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "scan":
		return runScan(args[1:], stdin, stdout, stderr)
	case "baseline":
		return runBaseline(args[1:], stdout, stderr)
	case "rules":
		return runRules(args[1:], stdout, stderr)
	case "explain":
		return runExplain(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "profiles":
		return runProfiles(stdout)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		writeUsage(stderr)
		return 1
	}
}

func runInit(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	output := fs.String("output", service.DefaultConfigPath(), "output config path")
	interactive := fs.Bool("interactive", false, "prompt for config values in the terminal")
	profile := fs.String("profile", "", "optional policy profile: startup, strict, enterprise, ai-safe")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg, err := exampleConfigForProfile(strings.TrimSpace(*profile))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "init profile: %v\n", err)
		return 1
	}
	if *interactive {
		reader := bufio.NewReader(stdin)
		*output, err = promptString(reader, stdout, "config output path", *output)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "interactive init: %v\n", err)
			return 1
		}
		cfg.Name, err = promptString(reader, stdout, "config name", cfg.Name)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "interactive init: %v\n", err)
			return 1
		}
	}

	if err := service.WriteConfigFile(*output, cfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "write config: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "wrote %s\n", *output)
	return 0
}

func runValidate(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config file or directory path")
	profile := fs.String("profile", "", "optional policy profile override")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg, err := loadConfigWithProfile(*configPath, *profile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	if err := service.ValidateConfig(cfg); err != nil {
		_, _ = fmt.Fprintf(stderr, "invalid config: %v\n", err)
		return 1
	}

	_, _ = fmt.Fprintln(stdout, "config valid")
	return 0
}

func runScan(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	inputs := scanInputs{
		configPath: fs.String("config", service.DefaultConfigPath(), "config file or directory path"),
		mode:       fs.String("mode", string(service.ScanModeFull), "scan mode: full or diff"),
		baseRef:    fs.String("base-ref", "main", "base branch/ref for diff mode"),
	}
	format := fs.String("format", "", "optional output format override: text, json, sarif, github")
	interactive := fs.Bool("interactive", false, "prompt for scan inputs in the terminal")
	profile := fs.String("profile", "", "optional policy profile override")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if err := promptScanInputs(*interactive, stdin, stdout, &inputs); err != nil {
		_, _ = fmt.Fprintf(stderr, "interactive scan: %v\n", err)
		return 1
	}

	scanMode, err := parseScanMode(*inputs.mode)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	cfg, err := loadConfigWithProfile(*inputs.configPath, *profile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	if strings.TrimSpace(*format) != "" {
		cfg.Output.Format = strings.TrimSpace(*format)
	}

	if err := executeScan(stdout, cfg, scanMode, strings.TrimSpace(*inputs.baseRef)); err != nil {
		_, _ = fmt.Fprintf(stderr, "scan failed: %v\n", err)
		return 1
	}
	return 0
}

type scanInputs struct {
	configPath *string
	mode       *string
	baseRef    *string
}

func promptScanInputs(interactive bool, stdin io.Reader, stdout io.Writer, inputs *scanInputs) error {
	if !interactive {
		return nil
	}

	reader := bufio.NewReader(stdin)
	var err error
	*inputs.configPath, err = promptString(reader, stdout, "config path", *inputs.configPath)
	if err != nil {
		return err
	}
	*inputs.mode, err = promptString(reader, stdout, "scan mode (full|diff)", *inputs.mode)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*inputs.mode) != string(service.ScanModeDiff) {
		return nil
	}

	*inputs.baseRef, err = promptString(reader, stdout, "base ref", *inputs.baseRef)
	return err
}

func parseScanMode(mode string) (service.ScanMode, error) {
	scanMode := service.ScanMode(strings.TrimSpace(mode))
	if scanMode != service.ScanModeFull && scanMode != service.ScanModeDiff {
		return "", fmt.Errorf("invalid scan mode %q", mode)
	}
	return scanMode, nil
}

func executeScan(stdout io.Writer, cfg service.Config, scanMode service.ScanMode, baseRef string) error {
	report, err := service.RunWithOptions(context.Background(), cfg, service.ScanOptions{
		Mode:    scanMode,
		BaseRef: baseRef,
	})
	if err != nil {
		return err
	}
	if err := writeScanMetadata(stdout, cfg.Output.Format, scanMode, baseRef); err != nil {
		return err
	}
	if err := service.WriteReport(stdout, report, cfg.Output.Format); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	if report.Summary.FailedSections > 0 {
		return fmt.Errorf("one or more sections failed")
	}
	return nil
}

func writeScanMetadata(stdout io.Writer, format string, scanMode service.ScanMode, baseRef string) error {
	if strings.TrimSpace(format) != "" && strings.TrimSpace(format) != "text" {
		return nil
	}
	if scanMode != service.ScanModeDiff {
		return nil
	}
	_, err := fmt.Fprintf(stdout, "Base Ref: %s\n", baseRef)
	return err
}

func runBaseline(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("baseline", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config path")
	outputPath := fs.String("output", "codeguard-baseline.json", "baseline output path")
	mode := fs.String("mode", string(service.ScanModeFull), "scan mode: full or diff")
	baseRef := fs.String("base-ref", "main", "base branch/ref for diff mode")
	profile := fs.String("profile", "", "optional policy profile override")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg, err := loadConfigWithProfile(*configPath, *profile)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
		return 1
	}
	cfg.Baseline.Path = ""

	report, err := service.RunWithOptions(context.Background(), cfg, service.ScanOptions{
		Mode:    service.ScanMode(strings.TrimSpace(*mode)),
		BaseRef: strings.TrimSpace(*baseRef),
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "baseline scan failed: %v\n", err)
		return 1
	}
	if err := service.WriteBaselineFile(*outputPath, service.BaselineEntriesFromReport(report)); err != nil {
		_, _ = fmt.Fprintf(stderr, "write baseline: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "wrote %s\n", *outputPath)
	return 0
}

func runRules(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("rules", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "optional config path to include custom rule packs")
	profile := fs.String("profile", "", "optional policy profile override")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	rules := service.Rules()
	if strings.TrimSpace(*configPath) != "" {
		cfg, err := loadConfigWithProfile(*configPath, *profile)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
			return 1
		}
		rules = service.RulesForConfig(cfg)
	}
	for _, rule := range rules {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", rule.ID, rule.DefaultLevel, rule.Section, rule.Title)
	}
	return 0
}

func runExplain(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("explain", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "", "optional config path to include custom rule packs")
	profile := fs.String("profile", "", "optional policy profile override")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if fs.NArg() == 0 {
		_, _ = fmt.Fprintln(stderr, "explain requires a rule id")
		return 1
	}

	ruleID := fs.Arg(0)
	rule, ok := service.ExplainRule(ruleID)
	if strings.TrimSpace(*configPath) != "" {
		cfg, err := loadConfigWithProfile(*configPath, *profile)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "load config: %v\n", err)
			return 1
		}
		rule, ok = service.ExplainRuleForConfig(cfg, ruleID)
	}
	if !ok {
		_, _ = fmt.Fprintf(stderr, "unknown rule %q\n", ruleID)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "%s\ntitle: %s\nsection: %s\nlevel: %s\n%s\n", rule.ID, rule.Title, rule.Section, rule.DefaultLevel, rule.Description)
	if strings.TrimSpace(rule.HowToFix) != "" {
		_, _ = fmt.Fprintf(stdout, "how to fix: %s\n", rule.HowToFix)
	}
	return 0
}

func runDoctor(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", service.DefaultConfigPath(), "config path")
	profile := fs.String("profile", "", "optional policy profile override")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	checks := make([]doctorCheck, 0)
	if _, err := os.Stat(*configPath); err != nil {
		checks = append(checks, doctorCheck{Name: "config", Status: "fail", Message: fmt.Sprintf("config file %s is missing", *configPath)})
		writeDoctorReport(stdout, checks)
		return 1
	}

	cfg, err := loadConfigWithProfile(*configPath, *profile)
	if err != nil {
		checks = append(checks, doctorCheck{Name: "config", Status: "fail", Message: err.Error()})
		writeDoctorReport(stdout, checks)
		return 1
	}
	checks = append(checks, doctorCheck{Name: "config", Status: "pass", Message: "config loads and validates"})

	if _, err := exec.LookPath("git"); err != nil {
		checks = append(checks, doctorCheck{Name: "git", Status: "fail", Message: "git is not available on PATH"})
	} else {
		checks = append(checks, doctorCheck{Name: "git", Status: "pass", Message: "git is available"})
	}

	for _, target := range cfg.Targets {
		if info, err := os.Stat(target.Path); err != nil || !info.IsDir() {
			checks = append(checks, doctorCheck{Name: "target:" + target.Name, Status: "fail", Message: fmt.Sprintf("target path %s is missing", target.Path)})
			continue
		}
		checks = append(checks, doctorCheck{Name: "target:" + target.Name, Status: "pass", Message: fmt.Sprintf("target path %s exists", target.Path)})

		if err := exec.Command("git", "-C", target.Path, "rev-parse", "--show-toplevel").Run(); err != nil {
			checks = append(checks, doctorCheck{Name: "repo:" + target.Name, Status: "warn", Message: fmt.Sprintf("%s is not a git worktree; diff scans will not work", target.Path)})
		} else {
			checks = append(checks, doctorCheck{Name: "repo:" + target.Name, Status: "pass", Message: "git worktree detected"})
		}
	}

	mode := strings.ToLower(strings.TrimSpace(cfg.Checks.SecurityRules.GovulncheckMode))
	if cfg.Checks.Security && mode != "off" {
		command := strings.TrimSpace(cfg.Checks.SecurityRules.GovulncheckCommand)
		if command == "" {
			command = "govulncheck"
		}
		if _, err := exec.LookPath(command); err != nil {
			status := "warn"
			if mode == "required" {
				status = "fail"
			}
			checks = append(checks, doctorCheck{Name: "govulncheck", Status: status, Message: fmt.Sprintf("%s is not available on PATH", command)})
		} else {
			checks = append(checks, doctorCheck{Name: "govulncheck", Status: "pass", Message: fmt.Sprintf("%s is available", command)})
		}
	}

	if cfg.Baseline.Path != "" {
		if _, err := os.Stat(cfg.Baseline.Path); err != nil {
			checks = append(checks, doctorCheck{Name: "baseline", Status: "warn", Message: fmt.Sprintf("baseline file %s is missing", cfg.Baseline.Path)})
		} else {
			checks = append(checks, doctorCheck{Name: "baseline", Status: "pass", Message: "baseline file found"})
		}
	}

	if cfg.Cache.Enabled != nil && *cfg.Cache.Enabled {
		cacheDir := filepath.Dir(cfg.Cache.Path)
		if cacheDir == "" {
			cacheDir = "."
		}
		if _, err := os.Stat(cacheDir); err != nil {
			if os.IsNotExist(err) {
				checks = append(checks, doctorCheck{Name: "cache", Status: "pass", Message: fmt.Sprintf("cache directory %s will be created on first run", cacheDir)})
			} else {
				checks = append(checks, doctorCheck{Name: "cache", Status: "warn", Message: fmt.Sprintf("cache directory %s is not writable", cacheDir)})
			}
		} else {
			checks = append(checks, doctorCheck{Name: "cache", Status: "pass", Message: fmt.Sprintf("cache will be written to %s", cfg.Cache.Path)})
		}
	}

	writeDoctorReport(stdout, checks)
	if hasDoctorFailures(checks) {
		return 1
	}
	return 0
}

func runProfiles(stdout io.Writer) int {
	for _, profile := range service.Profiles() {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\n", profile.Name, profile.Description)
	}
	return 0
}

func promptString(reader *bufio.Reader, stdout io.Writer, label string, fallback string) (string, error) {
	if _, err := fmt.Fprintf(stdout, "%s [%s]: ", label, fallback); err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return fallback, nil
	}
	return line, nil
}

func writeUsage(w io.Writer) {
	_, _ = io.WriteString(w, `codeguard checks repositories for code quality, design, security, CI, prompt safety, and custom policy rules.

Usage:
  codeguard init [-output codeguard.yaml] [-interactive] [-profile startup|strict|enterprise|ai-safe]
  codeguard validate [-config codeguard.yaml] [-profile startup|strict|enterprise|ai-safe]
  codeguard scan [-config codeguard.yaml] [-mode full|diff] [-base-ref main] [-format text|json|sarif|github] [-interactive] [-profile startup|strict|enterprise|ai-safe]
  codeguard baseline [-config codeguard.yaml] [-output codeguard-baseline.json] [-mode full|diff] [-base-ref main] [-profile startup|strict|enterprise|ai-safe]
  codeguard rules [-config codeguard.yaml]
  codeguard explain [-config codeguard.yaml] <rule-id>
  codeguard doctor [-config codeguard.yaml] [-profile startup|strict|enterprise|ai-safe]
  codeguard profiles
  codeguard version
`)
}

func loadConfigWithProfile(path string, profile string) (service.Config, error) {
	cfg, err := service.LoadConfigFile(path)
	if err != nil {
		return service.Config{}, err
	}
	if strings.TrimSpace(profile) != "" {
		cfg.Profile = strings.TrimSpace(profile)
		service.ApplyDefaults(&cfg)
		if err := service.ValidateConfig(cfg); err != nil {
			return service.Config{}, err
		}
	}
	return cfg, nil
}

func exampleConfigForProfile(profile string) (service.Config, error) {
	if strings.TrimSpace(profile) == "" {
		return service.ExampleConfig(), nil
	}
	return service.ExampleConfigForProfile(profile)
}

type doctorCheck struct {
	Name    string
	Status  string
	Message string
}

func writeDoctorReport(w io.Writer, checks []doctorCheck) {
	for _, check := range checks {
		_, _ = fmt.Fprintf(w, "[%s] %s: %s\n", strings.ToUpper(check.Status), check.Name, check.Message)
	}
}

func hasDoctorFailures(checks []doctorCheck) bool {
	for _, check := range checks {
		if check.Status == "fail" {
			return true
		}
	}
	return false
}
