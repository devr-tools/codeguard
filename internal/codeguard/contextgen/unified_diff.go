package contextgen

import (
	"fmt"
	"strings"
)

// diffContext is the number of unchanged lines shown around each change.
const diffContext = 3

type diffOp struct {
	kind byte // ' ', '-', '+'
	text string
}

// UnifiedDiff renders a unified diff from oldText to newText, labelled with
// path. It returns "" when the texts are equal. Agent docs are capped at a
// few hundred lines, so the quadratic LCS table stays small.
func UnifiedDiff(path string, oldText string, newText string) string {
	if oldText == newText {
		return ""
	}
	ops := diffLines(splitLines(oldText), splitLines(newText))
	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n", path, path)
	for _, hunk := range diffHunks(ops) {
		writeHunk(&b, ops, hunk)
	}
	return b.String()
}

func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// diffLines computes a line-level edit script with a longest-common-
// subsequence table.
func diffLines(a []string, b []string) []diffOp {
	lcs := make([][]int, len(a)+1)
	for i := range lcs {
		lcs[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	ops := make([]diffOp, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, diffOp{'-', a[i]})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j]})
			j++
		}
	}
	for ; i < len(a); i++ {
		ops = append(ops, diffOp{'-', a[i]})
	}
	for ; j < len(b); j++ {
		ops = append(ops, diffOp{'+', b[j]})
	}
	return ops
}

type hunkRange struct{ start, end int }

// diffHunks groups changed ops with diffContext lines of context, merging
// hunks whose context would overlap.
func diffHunks(ops []diffOp) []hunkRange {
	var hunks []hunkRange
	for idx, op := range ops {
		if op.kind == ' ' {
			continue
		}
		start := max(0, idx-diffContext)
		end := min(len(ops), idx+diffContext+1)
		if n := len(hunks); n > 0 && start <= hunks[n-1].end {
			hunks[n-1].end = max(hunks[n-1].end, end)
			continue
		}
		hunks = append(hunks, hunkRange{start, end})
	}
	return hunks
}

func writeHunk(b *strings.Builder, ops []diffOp, hunk hunkRange) {
	oldStart, newStart := 1, 1
	for _, op := range ops[:hunk.start] {
		if op.kind != '+' {
			oldStart++
		}
		if op.kind != '-' {
			newStart++
		}
	}
	oldCount, newCount := 0, 0
	for _, op := range ops[hunk.start:hunk.end] {
		if op.kind != '+' {
			oldCount++
		}
		if op.kind != '-' {
			newCount++
		}
	}
	fmt.Fprintf(b, "@@ -%s +%s @@\n", hunkSpan(oldStart, oldCount), hunkSpan(newStart, newCount))
	for _, op := range ops[hunk.start:hunk.end] {
		b.WriteByte(op.kind)
		b.WriteString(op.text)
		b.WriteByte('\n')
	}
}

// hunkSpan formats a hunk side; an empty side names the line before it, as
// diff(1) does.
func hunkSpan(start int, count int) string {
	if count == 0 {
		return fmt.Sprintf("%d,0", start-1)
	}
	return fmt.Sprintf("%d,%d", start, count)
}
