// codeguard-benchmark is a deliberately separate developer tool for frozen PR
// corpus measurements. It never fetches repositories; provision them first.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/benchmark"
)

func main() {
	os.Exit(runMain(os.Args[1:], os.Stdout, os.Stderr))
}

func runMain(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "-h", "--help", "help":
		usage(stdout)
		return 0
	case "export":
		return export(args[1:], stderr)
	case "external":
		return external(args[1:], stderr)
	case "run":
		return run(args[1:], stderr)
	case "compare":
		return compare(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
		usage(stderr)
		return 2
	}
}

func export(args []string, stderr io.Writer) int {
	flags := newFlagSet("export", stderr)
	manifestPath := flags.String("manifest", "", "benchmark manifest JSON")
	out := flags.String("out", "", "corpus export JSON")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	manifest, err := benchmark.Load(*manifestPath)
	if err != nil {
		fail(stderr, err)
		return 1
	}
	if err := benchmark.WriteJSON(*out, manifest.Export()); err != nil {
		fail(stderr, err)
		return 1
	}
	return 0
}

func run(args []string, stderr io.Writer) int {
	flags := newFlagSet("run", stderr)
	manifestPath := flags.String("manifest", "", "benchmark manifest JSON")
	out := flags.String("out", "", "result JSON")
	binary := flags.String("binary", "codeguard", "CodeGuard binary")
	workRoot := flags.String("work-root", "", "directory containing provisioned worktrees")
	warm := flags.Int("warm-repeats", 3, "warm scan repeats per entry")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	manifest, err := benchmark.Load(*manifestPath)
	if err != nil {
		fail(stderr, err)
		return 1
	}
	result, err := benchmark.Run(context.Background(), manifest, benchmark.RunOptions{Binary: *binary, WorkRoot: *workRoot, WarmRepeats: *warm})
	if err != nil {
		fail(stderr, err)
		return 1
	}
	if err := benchmark.WriteJSON(*out, result); err != nil {
		fail(stderr, err)
		return 1
	}
	return 0
}

func external(args []string, stderr io.Writer) int {
	flags := newFlagSet("external", stderr)
	var reports externalReportFlags
	flags.Var(&reports, "report", "external report in tool=path form; tool is one of gitleaks, trivy, trufflehog, semgrep")
	jsonOut := flags.String("json-out", "", "normalized external benchmark JSON")
	markdownOut := flags.String("markdown-out", "", "Markdown external benchmark summary")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if len(reports) == 0 {
		fail(stderr, fmt.Errorf("at least one -report tool=path flag is required"))
		return 2
	}
	if *jsonOut == "" && *markdownOut == "" {
		fail(stderr, fmt.Errorf("at least one of -json-out or -markdown-out is required"))
		return 2
	}
	report, err := benchmark.ImportExternalReports(reports)
	if err != nil {
		fail(stderr, err)
		return 1
	}
	if *jsonOut != "" {
		if err := benchmark.WriteJSON(*jsonOut, report); err != nil {
			fail(stderr, err)
			return 1
		}
	}
	if *markdownOut != "" {
		if err := benchmark.WriteMarkdown(*markdownOut, benchmark.RenderExternalMarkdown(report)); err != nil {
			fail(stderr, err)
			return 1
		}
	}
	return 0
}

func compare(args []string, stdout, stderr io.Writer) int {
	flags := newFlagSet("compare", stderr)
	out := flags.String("out", "", "Markdown report path; stdout when empty")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() == 0 {
		fail(stderr, fmt.Errorf("compare requires at least one benchmark result JSON"))
		return 2
	}
	summaries := make([]compareSummary, 0, flags.NArg())
	for _, path := range flags.Args() {
		result, err := readResult(path)
		if err != nil {
			fail(stderr, err)
			return 1
		}
		summaries = append(summaries, summarizeResult(result))
	}
	report := formatCompareMarkdown(summaries)
	if *out == "" {
		_, err := io.WriteString(stdout, report)
		if err != nil {
			fail(stderr, err)
			return 1
		}
		return 0
	}
	if err := os.WriteFile(*out, []byte(report), 0o600); err != nil {
		fail(stderr, fmt.Errorf("write compare report: %w", err))
		return 1
	}
	return 0
}

type externalReportFlags []benchmark.ExternalReportInput

func (flags *externalReportFlags) String() string {
	parts := make([]string, 0, len(*flags))
	for _, report := range *flags {
		parts = append(parts, report.Tool+"="+report.Path)
	}
	return strings.Join(parts, ",")
}

func (flags *externalReportFlags) Set(value string) error {
	tool, path, ok := strings.Cut(value, "=")
	if !ok {
		tool, path, ok = strings.Cut(value, ":")
	}
	if !ok || strings.TrimSpace(tool) == "" || strings.TrimSpace(path) == "" {
		return fmt.Errorf("report must be in tool=path form")
	}
	*flags = append(*flags, benchmark.ExternalReportInput{Tool: tool, Path: path})
	return nil
}

func readResult(path string) (benchmark.Result, error) {
	// #nosec G304 -- benchmark operators explicitly select local result files.
	data, err := os.ReadFile(path)
	if err != nil {
		return benchmark.Result{}, fmt.Errorf("read benchmark result %q: %w", path, err)
	}
	var result benchmark.Result
	if err := json.Unmarshal(data, &result); err != nil {
		return benchmark.Result{}, fmt.Errorf("parse benchmark result %q: %w", path, err)
	}
	if result.Version != benchmark.SchemaVersion {
		return benchmark.Result{}, fmt.Errorf("benchmark result %q version must be %d", path, benchmark.SchemaVersion)
	}
	if strings.TrimSpace(result.Tool) == "" {
		return benchmark.Result{}, fmt.Errorf("benchmark result %q tool is required", path)
	}
	return result, nil
}

type compareSummary struct {
	Tool       string
	Corpus     string
	Runs       int
	Entries    int
	Errors     int
	NonZero    int
	ColdMedian time.Duration
	WarmMedian time.Duration
	AllMedian  time.Duration
}

func summarizeResult(result benchmark.Result) compareSummary {
	entryIDs := map[string]bool{}
	var cold, warm, all []time.Duration
	summary := compareSummary{Tool: result.Tool, Corpus: result.Corpus, Runs: len(result.Runs)}
	for _, run := range result.Runs {
		entryIDs[run.ID] = true
		if run.Error != "" {
			summary.Errors++
		}
		if run.ExitCode != 0 {
			summary.NonZero++
		}
		all = append(all, run.Duration)
		switch run.Mode {
		case "cold":
			cold = append(cold, run.Duration)
		case "warm":
			warm = append(warm, run.Duration)
		}
	}
	summary.Entries = len(entryIDs)
	summary.ColdMedian = medianDuration(cold)
	summary.WarmMedian = medianDuration(warm)
	summary.AllMedian = medianDuration(all)
	return summary
}

func medianDuration(values []time.Duration) time.Duration {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func formatCompareMarkdown(summaries []compareSummary) string {
	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].Corpus == summaries[j].Corpus {
			return summaries[i].Tool < summaries[j].Tool
		}
		return summaries[i].Corpus < summaries[j].Corpus
	})
	var builder strings.Builder
	builder.WriteString("# CodeGuard Benchmark Comparison\n\n")
	builder.WriteString("| Tool | Corpus | Entries | Runs | Cold median | Warm median | Overall median | Non-zero exits | Errors |\n")
	builder.WriteString("| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, summary := range summaries {
		fmt.Fprintf(&builder, "| %s | %s | %d | %d | %s | %s | %s | %d | %d |\n",
			escapeMarkdownCell(summary.Tool),
			escapeMarkdownCell(summary.Corpus),
			summary.Entries,
			summary.Runs,
			formatDuration(summary.ColdMedian),
			formatDuration(summary.WarmMedian),
			formatDuration(summary.AllMedian),
			summary.NonZero,
			summary.Errors,
		)
	}
	builder.WriteString("\n")
	builder.WriteString("Non-zero exits are retained as benchmark data because scanners commonly exit non-zero when findings are present.\n")
	return builder.String()
}

func escapeMarkdownCell(value string) string {
	return strings.ReplaceAll(value, "|", `\|`)
}

func formatDuration(duration time.Duration) string {
	if duration == 0 {
		return "-"
	}
	ms := float64(duration) / float64(time.Millisecond)
	if ms < 1000 {
		return fmt.Sprintf("%.1fms", round(ms, 1))
	}
	seconds := float64(duration) / float64(time.Second)
	return fmt.Sprintf("%.2fs", round(seconds, 2))
}

func round(value float64, places int) float64 {
	scale := math.Pow10(places)
	return math.Round(value*scale) / scale
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	return flags
}

func usage(out io.Writer) {
	fmt.Fprintln(out, `usage: codeguard-benchmark <command> [flags]

Developer tool for reproducible benchmark corpora. It never fetches
repositories; provision immutable worktrees before running scans.

Commands:
  export    Write a publishable manifest inventory without local paths
  run       Run CodeGuard against provisioned manifest worktrees
  external  Normalize saved external scanner reports and summarize raw counts
  compare   Render a Markdown comparison from one or more result JSON files

Examples:
  codeguard-benchmark export -manifest benchmarks/manifest.example.json -out /tmp/corpus.json
  codeguard-benchmark run -manifest benchmarks/manifest.example.json -work-root /tmp/codeguard-bench -out /tmp/codeguard.json
  codeguard-benchmark external -report gitleaks=/tmp/gitleaks.sarif -report trivy=/tmp/trivy.json -json-out /tmp/external.json -markdown-out /tmp/external.md
  codeguard-benchmark compare /tmp/codeguard.json /tmp/semgrep.json -out /tmp/benchmark-report.md`)
}

func fail(stderr io.Writer, err error) {
	fmt.Fprintln(stderr, err)
}
