package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/whatsnew"
)

// scanOutputFlags are the scan flags that shape CLI output and the exit
// policy. Empty string values mean "keep what the config says".
type scanOutputFlags struct {
	quiet         *bool
	failOn        *string
	minConfidence *string
}

func registerScanOutputFlags(fs *flag.FlagSet) scanOutputFlags {
	return scanOutputFlags{
		quiet:         fs.Bool("quiet", false, "suppress the banner and progress lines; findings and the summary are still printed"),
		failOn:        fs.String("fail-on", "", "exit policy: fail (default; exit 1 when a section fails), warn (also when a section warns), none (only operational failures exit non-zero); overrides output.fail_on"),
		minConfidence: fs.String("min-confidence", "", "drop findings below this confidence: low, medium, high; overrides checks.min_confidence.default (per-section overrides still apply)"),
	}
}

// configOverrides validates the flag values and translates them into the
// dotted config overrides they stand for, so the CLI and the config file share
// one representation and one validator.
func (flags scanOutputFlags) configOverrides() ([]string, error) {
	overrides := make([]string, 0, 2)
	if raw := strings.TrimSpace(*flags.failOn); raw != "" {
		policy, ok := core.NormalizedFailOn(raw)
		if !ok {
			return nil, fmt.Errorf("invalid -fail-on %q: must be one of %s", raw, strings.Join(core.FailOnValues(), ", "))
		}
		overrides = append(overrides, "output.fail_on="+policy)
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
