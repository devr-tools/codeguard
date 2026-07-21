package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	service "github.com/devr-tools/codeguard/pkg/codeguard"
)

// batchFixInput is deliberately limited to caller-supplied findings and
// candidate diffs. Configuration and verification policy always come from the
// selected repository config, rather than an untrusted input file.
type batchFixInput struct {
	Items []service.FixBatchItem `json:"items"`
}

// runFixBatch verifies caller-supplied deterministic fixes in an isolated
// workspace. It prints an aggregate patch and never applies it to the current
// working tree.
func runFixBatch(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("fix-batch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	flags := registerScanRunFlags(fs)
	inputPath := fs.String("input", "", "path to JSON input containing explicit findings and candidate diffs")
	format := fs.String("format", "json", "output format: json or text")
	if err := fs.Parse(args); err != nil {
		return exitError
	}
	flags.applyTrustPolicy()
	if strings.TrimSpace(*inputPath) == "" {
		_, _ = fmt.Fprintln(stderr, "fix-batch requires -input with explicit finding and candidate-diff JSON")
		return exitError
	}
	if fs.NArg() != 0 {
		_, _ = fmt.Fprintln(stderr, "fix-batch does not accept positional arguments")
		return exitError
	}

	input, err := loadBatchFixInput(*inputPath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "load batch input: %v\n", err)
		return exitError
	}
	if len(input.Items) == 0 {
		_, _ = fmt.Fprintln(stderr, "fix-batch input must contain at least one item")
		return exitError
	}
	cfg, ok := loadConfigOrFail(*flags.configPath, *flags.profile, stderr)
	if !ok {
		return exitError
	}
	result, err := service.VerifyFixBatch(context.Background(), service.FixBatchRequest{
		Config: cfg,
		Items:  input.Items,
		Options: service.FixOptions{
			BaseRef:      strings.TrimSpace(*flags.baseRef),
			TestCommands: fixVerificationCommands(cfg),
		},
	})
	if err != nil {
		// VerifyFixBatch preserves structured failures in its result, but its
		// public API intentionally returns that result together with the error.
		return writeBatchFixResult(stdout, stderr, result, strings.TrimSpace(*format), err)
	}
	return writeBatchFixResult(stdout, stderr, result, strings.TrimSpace(*format), nil)
}

func loadBatchFixInput(path string) (batchFixInput, error) {
	// #nosec G304 -- the caller explicitly supplies the batch-fix input file.
	contents, err := os.ReadFile(path)
	if err != nil {
		return batchFixInput{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	var input batchFixInput
	if err := decoder.Decode(&input); err != nil {
		return batchFixInput{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return batchFixInput{}, fmt.Errorf("input must contain one JSON object")
		}
		return batchFixInput{}, err
	}
	return input, nil
}

func writeBatchFixResult(stdout io.Writer, stderr io.Writer, result service.FixBatchResult, format string, verifyErr error) int {
	switch format {
	case "", "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "marshal batch fix result: %v\n", err)
			return exitError
		}
		_, _ = stdout.Write(append(data, '\n'))
	case "text":
		if result.Verification.Diff != "" {
			_, _ = fmt.Fprintf(stdout, "Verified batch of %d deterministic fix(es):\n\n%s\n", len(result.Included), result.Verification.Diff)
		} else {
			_, _ = fmt.Fprintln(stdout, "No aggregate patch was verified.")
		}
		for _, issue := range append(result.Skipped, result.Failures...) {
			_, _ = fmt.Fprintf(stdout, "- item %d (%s): %s\n", issue.Index, issue.RuleID, issue.Reason)
		}
	default:
		_, _ = fmt.Fprintf(stderr, "unsupported batch fix output format %q\n", format)
		return exitError
	}
	if verifyErr != nil {
		_, _ = fmt.Fprintf(stderr, "verify batch fix: %v\n", verifyErr)
		return exitError
	}
	return exitOK
}
