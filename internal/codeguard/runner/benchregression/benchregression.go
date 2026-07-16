// Package benchregression runs `go test -bench` for the performance section's
// benchmark-regression gate and provides the pure parser, comparator, and
// baseline persistence around it. It deliberately lives beside the govulncheck
// runner rather than inside checks/performance: the subprocess plumbing
// (bounded output, contained timeout) mirrors the shell-out template in
// runner/govulncheck, and keeping ParseOutput/Compare as pure functions in a
// leaf package lets tests exercise them without ever executing a benchmark.
//
// The `go` binary here is codeguard's own fixed tool invocation (like git):
// the command name is never config-supplied, so it does not pass through
// trust.GuardConfigCommand. Only the package patterns come from configuration,
// and those are charset-validated (config/validate_performance.go) plus
// re-checked here so a pattern can never smuggle a flag or a path outside the
// target.
package benchregression

import (
	"context"
	"fmt"
	"strings"
	"time"

	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
)

// maxOutputBytes caps how much benchmark output is buffered so a runaway or
// malicious test binary cannot exhaust memory (mirrors runner/govulncheck).
const maxOutputBytes = 16 << 20 // 16 MiB

// runTimeout bounds a single benchmark run. Benchmarks default to ~1s of
// measurement per benchmark plus compilation, so a stuck run is capped well
// before it stalls a CI job indefinitely.
const runTimeout = 10 * time.Minute

// RunBenchmarks executes `go test -run=^$ -bench=. -benchmem <packages>` in
// dir with a contained timeout and bounded output, returning the combined
// stdout+stderr text. A non-zero exit is not fatal by itself when benchmark
// lines were produced (a failing unrelated package still yields usable
// results); callers get both the output and the error and decide.
func RunBenchmarks(ctx context.Context, dir string, packages []string) (string, error) {
	if len(packages) == 0 {
		return "", fmt.Errorf("no benchmark packages configured")
	}
	args := []string{"test", "-run=^$", "-bench=.", "-benchmem"}
	for _, pkg := range packages {
		// Defense in depth on top of config validation: a package argument must
		// never be able to act as a flag or leave the target directory.
		if err := validatePackagePattern(pkg); err != nil {
			return "", err
		}
		args = append(args, pkg)
	}
	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	text, err := runnersupport.RunLimitedCommand(ctx, dir, maxOutputBytes, "go", args...)
	if err != nil {
		return text, fmt.Errorf("go test -bench failed: %w", err)
	}
	return text, nil
}

// validatePackagePattern rejects package arguments that could be interpreted
// as flags or escape the working directory. It mirrors the config-time
// validation so programmatically-built configs get the same guarantee.
func validatePackagePattern(pkg string) error {
	trimmed := strings.TrimSpace(pkg)
	if trimmed == "" || trimmed != pkg {
		return fmt.Errorf("invalid benchmark package pattern %q", pkg)
	}
	if strings.HasPrefix(pkg, "-") {
		return fmt.Errorf("benchmark package pattern %q must not begin with '-'", pkg)
	}
	if pkg != "." && !strings.HasPrefix(pkg, "./") {
		return fmt.Errorf("benchmark package pattern %q must be relative (start with \"./\")", pkg)
	}
	for _, segment := range strings.Split(pkg, "/") {
		if segment == ".." {
			return fmt.Errorf("benchmark package pattern %q must not contain \"..\" segments", pkg)
		}
	}
	return nil
}
