package benchregression

import (
	"strconv"
	"strings"
)

// Result is one parsed benchmark line from standard `go test -bench` output,
// e.g. "BenchmarkX-8  1000  1234 ns/op  456 B/op  7 allocs/op". Name has the
// -GOMAXPROCS suffix stripped so baselines survive a core-count change.
type Result struct {
	Name        string  `json:"name"`
	Iterations  int64   `json:"iterations"`
	NsPerOp     float64 `json:"ns_per_op"`
	BytesPerOp  float64 `json:"bytes_per_op"`
	AllocsPerOp float64 `json:"allocs_per_op"`
}

// ParseOutput extracts benchmark results from go test -bench output text,
// ignoring every non-benchmark line (goos/goarch headers, PASS/ok trailers,
// compiler noise). Malformed benchmark lines are skipped rather than failing
// the run. When the same benchmark name appears more than once (e.g. two
// packages defining the same benchmark), the last occurrence wins.
func ParseOutput(text string) []Result {
	results := make([]Result, 0)
	index := map[string]int{}
	for _, line := range strings.Split(text, "\n") {
		result, ok := parseBenchmarkLine(line)
		if !ok {
			continue
		}
		if at, seen := index[result.Name]; seen {
			results[at] = result
			continue
		}
		index[result.Name] = len(results)
		results = append(results, result)
	}
	return results
}

// parseBenchmarkLine parses a single "Benchmark<Name>[-N] <iters> <value>
// <unit> ..." line. The value/unit pairs after the iteration count are
// interpreted by unit so extra custom metrics (b.ReportMetric) do not break
// parsing.
func parseBenchmarkLine(line string) (Result, bool) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 4 || !strings.HasPrefix(fields[0], "Benchmark") {
		return Result{}, false
	}
	// "Benchmark" alone is not a benchmark name; require Benchmark<X>.
	if fields[0] == "Benchmark" {
		return Result{}, false
	}
	iterations, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return Result{}, false
	}
	result := Result{Name: normalizeBenchmarkName(fields[0]), Iterations: iterations}
	sawNsPerOp := false
	for i := 2; i+1 < len(fields); i += 2 {
		value, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return Result{}, false
		}
		switch fields[i+1] {
		case "ns/op":
			result.NsPerOp = value
			sawNsPerOp = true
		case "B/op":
			result.BytesPerOp = value
		case "allocs/op":
			result.AllocsPerOp = value
		}
	}
	if !sawNsPerOp {
		return Result{}, false
	}
	return result, true
}

// normalizeBenchmarkName strips the trailing -GOMAXPROCS suffix go test
// appends ("BenchmarkX-8" -> "BenchmarkX") so results stay comparable across
// machines with different core counts. Sub-benchmark separators ("/") are
// preserved.
func normalizeBenchmarkName(name string) string {
	dash := strings.LastIndex(name, "-")
	if dash <= 0 {
		return name
	}
	suffix := name[dash+1:]
	if suffix == "" {
		return name
	}
	if _, err := strconv.Atoi(suffix); err != nil {
		return name
	}
	return name[:dash]
}
