package reliability

import (
	"strings"
	"testing"
)

func TestIsZeroIntegerLiteral(t *testing.T) {
	tests := map[string]bool{
		"0":     true,
		"0_0":   true,
		"0b0_0": true,
		"0o0_0": true,
		"0x0_0": true,
		"1":     false,
		"0b0_1": false,
		"0o0_7": false,
		"0x0_f": false,
		"0X0_A": false,
	}

	for literal, want := range tests {
		t.Run(literal, func(t *testing.T) {
			if got := isZeroIntegerLiteral(literal); got != want {
				t.Fatalf("isZeroIntegerLiteral(%q) = %t, want %t", literal, got, want)
			}
		})
	}
}

func TestIsZeroIntegerLiteralHandlesHugeUntrustedLiteral(t *testing.T) {
	literal := strings.Repeat("9", 4_000_000)
	if isZeroIntegerLiteral(literal) {
		t.Fatal("large nonzero literal classified as zero")
	}
}
