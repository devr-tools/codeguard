package cli_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/report"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// ciFixture builds a one-section repository whose scan result is chosen by
// kind: "warn" yields a single warn-level security finding
// (security.shell-execution) and nothing else, "fail" adds an unformatted Go
// file so the quality section fails on quality.gofmt.
func ciFixture(t *testing.T, kind string) string {
	t.Helper()
	dir := t.TempDir()
	writeScanTestFile(t, filepath.Join(dir, "main.go"),
		"package main\n\nimport \"os/exec\"\n\nfunc main() {\n\t_ = exec.Command(\"sh\", \"-c\", \"echo hi\").Run()\n}\n")
	quality := "false"
	if kind == "fail" {
		quality = "true"
		writeScanTestFile(t, filepath.Join(dir, "ugly.go"), "package main\nfunc  ugly( ) {   }\n")
	}
	config := `{
  "name": "ci-output",
  "targets": [{"name": "repo", "path": ".", "language": "go"}],
  "cache": {"enabled": false},
  "checks": {"quality": ` + quality + `, "design": false, "security": true, "prompts": false, "ci": false,
             "performance": false, "context": false,
             "security_rules": {"govulncheck_mode": "off"}},
  "output": {"format": "text"}
}`
	configPath := filepath.Join(dir, "codeguard.json")
	writeScanTestFile(t, configPath, config)
	return configPath
}

func runCLI(t *testing.T, args ...string) (code int, stdout string, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = cli.Run(args, strings.NewReader(""), &out, &errOut)
	return code, out.String(), errOut.String()
}

// containsBanner detects either form of the brand banner: the braille logo
// from img/codeguard.txt, or the ASCII fallback used when the asset is absent.
func containsBanner(text string) bool {
	for _, r := range text {
		if r >= 0x2800 && r <= 0x28FF {
			return true
		}
	}
	return strings.Contains(text, `\____\___/`)
}

func TestBannerDetectorRecognisesLibraryBanner(t *testing.T) {
	// report.Write keeps the banner by default for library callers; this is the
	// positive control that keeps the "no banner" assertions below meaningful.
	var buf bytes.Buffer
	if err := report.Write(&buf, core.Report{Name: "control"}, "text"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !containsBanner(buf.String()) {
		t.Fatalf("library text report has no banner:\n%s", buf.String())
	}
	buf.Reset()
	if err := report.WriteWithOptions(&buf, core.Report{Name: "control"}, "text", report.WriteOptions{OmitBanner: true}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if containsBanner(buf.String()) || !strings.Contains(buf.String(), "control") {
		t.Fatalf("OmitBanner output = %q, want the header without the banner", buf.String())
	}
}

func TestScanOmitsBannerWhenStdoutIsPiped(t *testing.T) {
	configPath := ciFixture(t, "warn")

	code, stdout, stderr := runCLI(t, "scan", "-config", configPath)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if containsBanner(stdout) {
		t.Fatalf("banner printed to a non-terminal writer:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Summary:") {
		t.Fatalf("summary missing:\n%s", stdout)
	}

	// A real OS pipe, not just an in-memory buffer.
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	var errOut bytes.Buffer
	done := make(chan []byte)
	go func() {
		data, _ := io.ReadAll(reader)
		done <- data
	}()
	code = cli.Run([]string{"scan", "-config", configPath}, strings.NewReader(""), writer, &errOut)
	_ = writer.Close()
	piped := string(<-done)
	_ = reader.Close()
	if code != 0 {
		t.Fatalf("piped exit = %d, stderr = %s", code, errOut.String())
	}
	if containsBanner(piped) || !strings.Contains(piped, "Summary:") {
		t.Fatalf("piped stdout = %q, want summary without banner", piped)
	}
}

func TestScanOmitsBannerUnderCIAndNoColor(t *testing.T) {
	configPath := ciFixture(t, "warn")
	for _, env := range []struct{ key, value string }{{"CI", "true"}, {"NO_COLOR", "1"}} {
		t.Run(env.key, func(t *testing.T) {
			t.Setenv(env.key, env.value)
			code, stdout, stderr := runCLI(t, "scan", "-config", configPath)
			if code != 0 {
				t.Fatalf("exit = %d, stderr = %s", code, stderr)
			}
			if containsBanner(stdout) {
				t.Fatalf("banner printed with %s=%s:\n%s", env.key, env.value, stdout)
			}
		})
	}
}

// The terminal case cannot be produced with a pipe in a test, so the policy is
// exercised directly with stdout reported as a terminal.
func TestBannerPolicy(t *testing.T) {
	env := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}
	tests := []struct {
		name     string
		quiet    bool
		terminal bool
		env      map[string]string
		want     bool
	}{
		{name: "interactive terminal", terminal: true, want: true},
		{name: "not a terminal", terminal: false, want: false},
		{name: "quiet", quiet: true, terminal: true, want: false},
		{name: "CI=true", terminal: true, env: map[string]string{"CI": "true"}, want: false},
		{name: "CI=1", terminal: true, env: map[string]string{"CI": "1"}, want: false},
		{name: "CI=false is not CI", terminal: true, env: map[string]string{"CI": "false"}, want: true},
		{name: "CI=0 is not CI", terminal: true, env: map[string]string{"CI": "0"}, want: true},
		{name: "NO_COLOR", terminal: true, env: map[string]string{"NO_COLOR": "1"}, want: false},
		{name: "TERM=dumb", terminal: true, env: map[string]string{"TERM": "dumb"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cli.BannerPolicy(tt.quiet, tt.terminal, env(tt.env)); got != tt.want {
				t.Fatalf("BannerPolicy = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScanQuietSuppressesProgressButKeepsFindingsAndSummary(t *testing.T) {
	configPath := ciFixture(t, "warn")

	code, _, stderr := runCLI(t, "scan", "-config", configPath)
	if code != 0 || !strings.Contains(stderr, "scan started") {
		t.Fatalf("baseline run: exit = %d, stderr = %q; want progress lines without -quiet", code, stderr)
	}

	code, stdout, stderr := runCLI(t, "scan", "-quiet", "-config", configPath)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	for _, progress := range []string{"scan started", "completed ", "scan in progress"} {
		if strings.Contains(stderr, progress) {
			t.Fatalf("-quiet stderr contains progress line %q: %q", progress, stderr)
		}
	}
	plain := stripANSI(stdout)
	if containsBanner(plain) {
		t.Fatalf("-quiet printed the banner:\n%s", plain)
	}
	if !strings.Contains(plain, "security.shell-execution") {
		t.Fatalf("-quiet dropped findings:\n%s", plain)
	}
	if !strings.Contains(plain, "Summary: 0 pass, 1 warn, 0 fail") {
		t.Fatalf("-quiet dropped the summary:\n%s", plain)
	}
}

func TestScanFailOnExitCodes(t *testing.T) {
	warnConfig := ciFixture(t, "warn")
	failConfig := ciFixture(t, "fail")
	tests := []struct {
		name   string
		config string
		args   []string
		want   int
	}{
		{name: "default/warn result", config: warnConfig, want: 0},
		{name: "default/fail result", config: failConfig, want: 1},
		{name: "fail/warn result", config: warnConfig, args: []string{"-fail-on", "fail"}, want: 0},
		{name: "fail/fail result", config: failConfig, args: []string{"-fail-on", "fail"}, want: 1},
		{name: "warn/warn result", config: warnConfig, args: []string{"-fail-on", "warn"}, want: 1},
		{name: "warn/fail result", config: failConfig, args: []string{"-fail-on", "warn"}, want: 1},
		{name: "none/warn result", config: warnConfig, args: []string{"-fail-on", "none"}, want: 0},
		{name: "none/fail result", config: failConfig, args: []string{"-fail-on", "none"}, want: 0},
		{name: "case-insensitive", config: warnConfig, args: []string{"-fail-on", "WARN"}, want: 1},
		{name: "config output.fail_on via -set", config: warnConfig, args: []string{"-set", "output.fail_on=warn"}, want: 1},
		{name: "flag wins over -set", config: failConfig, args: []string{"-set", "output.fail_on=warn", "-fail-on", "none"}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"scan", "-quiet", "-config", tt.config}, tt.args...)
			code, stdout, stderr := runCLI(t, args...)
			if code != tt.want {
				t.Fatalf("exit = %d, want %d\nstderr = %s\nstdout = %s", code, tt.want, stderr, stdout)
			}
			if !strings.Contains(stdout, "Summary:") {
				t.Fatalf("report not written:\n%s", stdout)
			}
		})
	}
}

func TestScanFailOnFromConfigFile(t *testing.T) {
	configPath := ciFixture(t, "warn")
	data, err := os.ReadFile(configPath) //nolint:gosec // test-owned path under t.TempDir
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	strict := strings.Replace(string(data), `"format": "text"`, `"format": "text", "fail_on": "warn"`, 1)
	writeScanTestFile(t, configPath, strict)

	if code, _, stderr := runCLI(t, "scan", "-quiet", "-config", configPath); code != 1 || !strings.Contains(stderr, "fail-on: warn") {
		t.Fatalf("exit = %d, stderr = %q; want output.fail_on=warn to fail a warning scan", code, stderr)
	}
	if code, _, stderr := runCLI(t, "validate", "-config", configPath); code != 0 {
		t.Fatalf("validate rejected output.fail_on=warn: %s", stderr)
	}

	writeScanTestFile(t, configPath, strings.Replace(strict, `"fail_on": "warn"`, `"fail_on": "sometimes"`, 1))
	if code, _, stderr := runCLI(t, "validate", "-config", configPath); code != 1 || !strings.Contains(stderr, "output.fail_on") {
		t.Fatalf("validate exit = %d, stderr = %q; want an output.fail_on error", code, stderr)
	}
}

func TestScanRejectsInvalidFailOnAndMinConfidence(t *testing.T) {
	configPath := ciFixture(t, "warn")
	for _, args := range [][]string{{"-fail-on", "sometimes"}, {"-min-confidence", "certain"}} {
		code, stdout, stderr := runCLI(t, append([]string{"scan", "-config", configPath}, args...)...)
		if code != 1 || !strings.Contains(stderr, "invalid "+args[0]) {
			t.Fatalf("%v: exit = %d, stderr = %q", args, code, stderr)
		}
		if strings.Contains(stdout, "Summary:") {
			t.Fatalf("%v: scan ran despite an invalid flag", args)
		}
	}
}

func TestScanFailOnNoneStillFailsOnTargetWalkError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	configPath := ciFixture(t, "warn")
	locked := filepath.Join(filepath.Dir(configPath), "locked")
	writeScanTestFile(t, filepath.Join(locked, "hidden.go"), "package locked\n")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) }) //nolint:gosec // restore test dir for cleanup
	if _, err := os.ReadDir(locked); err == nil {
		t.Skip("filesystem does not enforce directory permissions")
	}

	code, stdout, stderr := runCLI(t, "scan", "-quiet", "-fail-on", "none", "-config", configPath)
	if code == 0 {
		t.Fatalf("-fail-on none masked a target walk failure\nstdout = %s", stdout)
	}
	if !strings.Contains(stderr, "scan.target-walk") {
		t.Fatalf("stderr = %q, want it to name scan.target-walk", stderr)
	}
}

func TestScanFailOnNoneStillFailsOnMissingTarget(t *testing.T) {
	configPath := writeMissingTargetConfig(t, t.TempDir(), "./does-not-exist")
	if code, _, stderr := runCLI(t, "scan", "-fail-on", "none", "-config", configPath); code == 0 {
		t.Fatalf("-fail-on none masked a missing target; stderr = %s", stderr)
	}
}

func TestScanFailOnNoneStillFailsOnBadConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "codeguard.json")
	writeScanTestFile(t, configPath, `{"name": "broken", "targets": []}`)
	if code, _, stderr := runCLI(t, "scan", "-fail-on", "none", "-config", configPath); code == 0 {
		t.Fatalf("-fail-on none masked an invalid config; stderr = %s", stderr)
	}
}

// ReportGateError is the exit decision the scan command applies. A section
// panic cannot be provoked through a real scan, so it is exercised on a
// synthetic report: the operational diagnostic must fail the gate under every
// policy, while finding-driven status follows the policy.
func TestReportGateErrorKeepsOperationalFailuresUnmaskable(t *testing.T) {
	panicked := core.Report{
		Sections: []core.SectionResult{{
			ID: "security", Name: "Security", Status: core.StatusFail,
			Diagnostics: []core.Diagnostic{{
				ID: "scan.section.panic", Level: "fail", Kind: "infrastructure", Operational: true,
				Message: "Security check crashed",
			}},
		}},
		Summary: core.ReportSummary{FailedSections: 1},
	}
	walkFailed := core.Report{
		Sections: []core.SectionResult{{
			ID: "quality", Name: "Code Quality", Status: core.StatusFail,
			Diagnostics: []core.Diagnostic{{
				ID: "scan.target-walk", Level: "fail", Kind: "infrastructure", Operational: true,
				Message: "could not walk target",
			}},
		}},
		Summary: core.ReportSummary{FailedSections: 1},
	}
	for _, policy := range []string{"", service.FailOnFail, service.FailOnWarn, service.FailOnNone} {
		for name, rep := range map[string]core.Report{"panic": panicked, "walk": walkFailed} {
			err := service.ReportGateError(rep, policy)
			if err == nil {
				t.Fatalf("policy %q masked a %s diagnostic", policy, name)
			}
			if !strings.Contains(err.Error(), rep.Sections[0].Diagnostics[0].ID) {
				t.Fatalf("policy %q error = %q, want it to name the diagnostic", policy, err)
			}
		}
	}

	warnOnly := core.Report{
		Sections: []core.SectionResult{{
			ID: "security", Name: "Security", Status: core.StatusWarn,
			// A warn-level operational diagnostic is degraded, not broken.
			Diagnostics: []core.Diagnostic{{ID: "scan.file-read", Level: "warn", Operational: true}},
		}},
		Summary: core.ReportSummary{WarnedSections: 1},
	}
	findingFail := core.Report{
		Sections: []core.SectionResult{{ID: "quality", Name: "Code Quality", Status: core.StatusFail}},
		Summary:  core.ReportSummary{FailedSections: 1},
	}
	cases := []struct {
		rep    core.Report
		policy string
		fails  bool
	}{
		{warnOnly, "", false},
		{warnOnly, service.FailOnWarn, true},
		{warnOnly, service.FailOnNone, false},
		{findingFail, service.FailOnFail, true},
		{findingFail, service.FailOnNone, false},
	}
	for _, c := range cases {
		if err := service.ReportGateError(c.rep, c.policy); (err != nil) != c.fails {
			t.Fatalf("ReportGateError(status=%s, policy=%q) = %v, want fails=%v", c.rep.Sections[0].Status, c.policy, err, c.fails)
		}
	}
	if err := service.ReportGateError(findingFail, ""); err == nil || !strings.Contains(err.Error(), "one or more sections failed") {
		t.Fatalf("default policy error = %v, want the historical message", err)
	}
}

func TestScanMinConfidenceFiltersFindings(t *testing.T) {
	configPath := ciFixture(t, "warn")

	code, stdout, stderr := runCLI(t, "scan", "-quiet", "-format", "json", "-config", configPath)
	if code != 0 || !strings.Contains(stdout, "security.shell-execution") {
		t.Fatalf("baseline: exit = %d, stderr = %s; want the shell-execution finding", code, stderr)
	}

	code, stdout, stderr = runCLI(t, "scan", "-quiet", "-format", "json", "-min-confidence", "high", "-config", configPath)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if !strings.Contains(stdout, `"total_findings": 0`) {
		t.Fatalf("-min-confidence high kept a finding below high confidence:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"confidence_filtered_count": 1`) {
		t.Fatalf("filtered finding not accounted for:\n%s", stdout)
	}

	// Same result through the config key the flag maps onto.
	code, viaSet, stderr := runCLI(t, "scan", "-quiet", "-format", "json", "-set", "checks.min_confidence.default=high", "-config", configPath)
	if code != 0 || !strings.Contains(viaSet, `"confidence_filtered_count": 1`) {
		t.Fatalf("-set checks.min_confidence.default=high: exit = %d, stderr = %s\n%s", code, stderr, viaSet)
	}

	// Filtering the only warning turns a strict scan green.
	if code, _, stderr := runCLI(t, "scan", "-quiet", "-fail-on", "warn", "-min-confidence", "high", "-config", configPath); code != 0 {
		t.Fatalf("-fail-on warn -min-confidence high: exit = %d, stderr = %s", code, stderr)
	}
}
