package history

import (
	"bufio"
	"strings"
	"testing"
)

func TestReadBoundedLineDrainsOversizedLine(t *testing.T) {
	input := "+" + strings.Repeat("x", maxHistoryLineBytes*2) + "\nnext\n"
	buf := strings.NewReader(input)
	reader := bufio.NewReaderSize(buf, maxHistoryLineBytes)

	line, err := readBoundedLine(reader)
	if err != nil {
		t.Fatalf("read oversized line: %v", err)
	}
	if len(line) != maxHistoryLineBytes {
		t.Fatalf("line length = %d, want %d", len(line), maxHistoryLineBytes)
	}
	next, err := readBoundedLine(reader)
	if err != nil {
		t.Fatalf("read following line: %v", err)
	}
	if got := string(next); got != "next\n" {
		t.Fatalf("following line = %q, want %q", got, "next\\n")
	}
}
