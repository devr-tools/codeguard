// codeguard-benchmark is a deliberately separate developer tool for frozen PR
// corpus measurements. It never fetches repositories; provision them first.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/devr-tools/codeguard/internal/benchmark"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "export":
		export(os.Args[2:])
	case "run":
		run(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func export(args []string) {
	flags := flag.NewFlagSet("export", flag.ExitOnError)
	manifestPath := flags.String("manifest", "", "benchmark manifest JSON")
	out := flags.String("out", "", "corpus export JSON")
	_ = flags.Parse(args)
	manifest, err := benchmark.Load(*manifestPath)
	fail(err)
	fail(benchmark.WriteJSON(*out, manifest.Export()))
}

func run(args []string) {
	flags := flag.NewFlagSet("run", flag.ExitOnError)
	manifestPath := flags.String("manifest", "", "benchmark manifest JSON")
	out := flags.String("out", "", "result JSON")
	binary := flags.String("binary", "codeguard", "CodeGuard binary")
	workRoot := flags.String("work-root", "", "directory containing provisioned worktrees")
	warm := flags.Int("warm-repeats", 3, "warm scan repeats per entry")
	_ = flags.Parse(args)
	manifest, err := benchmark.Load(*manifestPath)
	fail(err)
	result, err := benchmark.Run(context.Background(), manifest, benchmark.RunOptions{Binary: *binary, WorkRoot: *workRoot, WarmRepeats: *warm})
	fail(err)
	fail(benchmark.WriteJSON(*out, result))
}

func usage() { fmt.Fprintln(os.Stderr, "usage: codeguard-benchmark <export|run> [flags]") }
func fail(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
