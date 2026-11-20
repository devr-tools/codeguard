package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/whatsnew"
	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// scanOutputFlags are the scan flags that shape CLI output and the exit
// policy. Empty string values mean "keep what the config says".
type scanOutputFlags struct {
	quiet         *bool
	failOn        *string
	minConfidence *string
	sarifOutput   *string
}

// failOnFlagUsage is shared by scan and validate-patch so both describe the
// same exit policy.
const failOnFlagUsage = "exit policy: fail (default; exit 1 when a section fails), warn (also when a section warns), none (only operational failures exit non-zero); overrides output.fail_on"

func registerFailOnFlag(fs *flag.FlagSet) *string {
	return fs.String("fail-on", "", failOnFlagUsage)
}

func registerScanOutputFlags(fs *flag.FlagSet) scanOutputFlags {
	return scanOutputFlags{
		quiet:         fs.Bool("quiet", false, "suppress the banner and progress lines; findings and the summary are still printed"),
		failOn:        registerFailOnFlag(fs),
		minConfidence: fs.String("min-confidence", "", "drop findings below this confidence: low, medium, high; overrides checks.min_confidence.default (per-section overrides still apply)"),
		sarifOutput:   fs.String("sarif-output", "", "also write the report as SARIF 2.1.0 to this file, independent of -format; written even when the scan finds failures"),
	}
}

// configOverrides validates the flag values and translates them into the
// dotted config overrides they stand for, so the CLI and the config file share
// one representation and one validator.
func (flags scanOutputFlags) configOverrides() ([]string, error) {
	overrides, err := failOnOverrides(*flags.failOn)
	if err != nil {
		return nil, err
	}
	if raw := strings.TrimSpace(*flags.minConfidence); raw != "" {
		level := core.NormalizedConfidence(raw)
		if level == "" {
			return nil, fmt.Errorf("invalid -min-confidence %q: must be one of %s, %s, %s", raw,
				core.ConfidenceLow, core.ConfidenceMedium, core.ConfidenceHigh)
		}
		overrides = append(overrides, "checks.min_confidence.default="+level)
	}
	return overrides, nil
}

// failOnOverrides validates a -fail-on value and returns the matching
// output.fail_on override, or none when the flag is empty.
func failOnOverrides(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return make([]string, 0, 2), nil
	}
	policy, ok := core.NormalizedFailOn(raw)
	if !ok {
		return nil, fmt.Errorf("invalid -fail-on %q: must be one of %s", raw, strings.Join(core.FailOnValues(), ", "))
	}
	return []string{"output.fail_on=" + policy}, nil
}

// bannerEnabled reports whether the text banner belongs on stdout: only for an
// interactive terminal outside CI, with color allowed and -quiet unset.
func bannerEnabled(stdout io.Writer, quiet bool) bool {
	return BannerPolicy(quiet, whatsnew.IsTerminal(stdout), os.Getenv)
}

// BannerPolicy is the banner decision with its inputs made explicit: the
// banner is shown only when quiet is false, stdout is a terminal, CI is unset
// or falsy, NO_COLOR is unset, and TERM is not "dumb". It is exported so the
// terminal case, which tests cannot fake with a pipe, stays covered.
func BannerPolicy(quiet bool, stdoutIsTerminal bool, getenv func(string) string) bool {
	if quiet || !stdoutIsTerminal {
		return false
	}
	if strings.TrimSpace(getenv("NO_COLOR")) != "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(getenv("TERM")), "dumb") {
		return false
	}
	return !envTruthy(getenv("CI"))
}

func envTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

// writeSARIFFile writes the report as SARIF to path, creating its directory.
// It lets one scan feed both the human-facing -format output and a code
// scanning upload, instead of scanning twice.
func writeSARIFFile(path string, report service.Report) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create SARIF output directory: %w", err)
		}
	}
	file, err := os.Create(path) // #nosec G304 -- the path is the operator's own -sarif-output flag
	if err != nil {
		return fmt.Errorf("create SARIF output: %w", err)
	}
	writeErr := service.WriteReport(file, report, "sarif")
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write SARIF output: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("write SARIF output: %w", closeErr)
	}
	return nil
}
