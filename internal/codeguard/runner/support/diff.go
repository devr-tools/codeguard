package support

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type LineRanges struct {
	allChanged bool
	ranges     [][2]int
}

// Export converts the internal representation into the core type shared with
// checks that need to intersect findings with changed lines.
func (r LineRanges) Export() core.ChangedLineRanges {
	return core.ChangedLineRanges{
		AllChanged: r.allChanged,
		Ranges:     append([][2]int(nil), r.ranges...),
	}
}

func LoadDiffScope(targets []core.TargetConfig, baseRef string) (map[string]LineRanges, error) {
	out := map[string]LineRanges{}
	for _, target := range targets {
		scope, err := gitChangedLines(target.Path, baseRef)
		if err != nil {
			return nil, err
		}
		for path, ranges := range scope {
			out[path] = ranges
		}
	}
	return out, nil
}

func gitChangedLines(dir string, baseRef string) (map[string]LineRanges, error) {
	argsVariants := [][]string{
		{"-C", dir, "diff", "--unified=0", "--no-color", baseRef, "--"},
		{"-C", dir, "diff", "--unified=0", "--no-color", baseRef + "...HEAD", "--"},
	}
	var output []byte
	var err error
	for _, args := range argsVariants {
		cmd := exec.Command("git", args...)
		output, err = cmd.CombinedOutput()
		if err == nil {
			return parseUnifiedDiff(string(output)), nil
		}
	}
	return nil, fmt.Errorf("diff mode requires git diff against %q: %v", baseRef, err)
}

func parseUnifiedDiff(diff string) map[string]LineRanges {
	out := map[string]LineRanges{}
	currentFile := ""
	deletedFrom := ""
	lines := strings.Split(diff, "\n")
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "--- a/"):
			deletedFrom = strings.TrimPrefix(line, "--- a/")
		case strings.HasPrefix(line, "+++ /dev/null"):
			// Deleted file: keep the old path in scope so findings that
			// reference removed files survive diff filtering.
			currentFile = ""
			if deletedFrom != "" {
				out[deletedFrom] = LineRanges{allChanged: true}
				deletedFrom = ""
			}
		case strings.HasPrefix(line, "+++ b/"):
			deletedFrom = ""
			currentFile = strings.TrimPrefix(line, "+++ b/")
			if currentFile != "" {
				if _, ok := out[currentFile]; !ok {
					out[currentFile] = LineRanges{allChanged: true}
				}
			}
		case strings.HasPrefix(line, "@@") && currentFile != "":
			start, end, ok := parseHunkHeader(line)
			if !ok {
				continue
			}
			scope := out[currentFile]
			scope.allChanged = false
			scope.ranges = append(scope.ranges, [2]int{start, end})
			out[currentFile] = scope
		}
	}
	return out
}

func parseHunkHeader(header string) (int, int, bool) {
	parts := strings.Split(header, " ")
	for _, part := range parts {
		if !strings.HasPrefix(part, "+") {
			continue
		}
		part = strings.TrimPrefix(part, "+")
		part = strings.TrimSuffix(part, "@@")
		pieces := strings.Split(part, ",")
		start, err := strconv.Atoi(strings.TrimSpace(pieces[0]))
		if err != nil {
			return 0, 0, false
		}
		count := 1
		if len(pieces) > 1 {
			count, _ = strconv.Atoi(strings.TrimSpace(pieces[1]))
		}
		if count == 0 {
			return start, start, true
		}
		return start, start + count - 1, true
	}
	return 0, 0, false
}
