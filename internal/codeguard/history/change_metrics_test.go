package history

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseChangeMetricsDiscardsOversizedCommit(t *testing.T) {
	var input strings.Builder
	input.WriteString(changeCommitMarker + "deadbeef\x00large change\n")
	for i := 0; i <= maxFilesPerChangeCommit; i++ {
		fmt.Fprintf(&input, "1\t0\tfile-%03d.go\n", i)
	}
	input.WriteString(changeCommitMarker + "cafebabe\x00small change\n1\t0\tkept.go\n")

	report := parseChangeMetrics(strings.NewReader(input.String()))

	if report.CommitsScanned != 1 {
		t.Fatalf("CommitsScanned = %d, want 1", report.CommitsScanned)
	}
	if len(report.Files) != 1 || report.Files["kept.go"].Commits != 1 {
		t.Fatalf("Files = %#v, want only kept.go", report.Files)
	}
}

func TestParseChangeMetricsBoundsStoredCoChangeRelations(t *testing.T) {
	var input strings.Builder
	for commit := 0; commit < 4; commit++ {
		fmt.Fprintf(&input, "%s%08d\x00change\n", changeCommitMarker, commit)
		for file := 0; file < maxFilesPerChangeCommit; file++ {
			fmt.Fprintf(&input, "1\t0\tcommit-%d/file-%03d.go\n", commit, file)
		}
	}

	report := parseChangeMetrics(strings.NewReader(input.String()))
	stored := 0
	for _, metric := range report.Files {
		stored += len(metric.CoChangePartners)
	}
	if stored != maxStoredCoChangeRelations {
		t.Fatalf("stored co-change relations = %d, want %d", stored, maxStoredCoChangeRelations)
	}
}
