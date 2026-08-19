package quality

import (
	"errors"
	"testing"
)

func TestLimitedBufferRejectsOutputPastLimit(t *testing.T) {
	t.Parallel()

	var output limitedBuffer
	output.limit = 4

	n, err := output.Write([]byte("abcdef"))
	if n != output.limit {
		t.Fatalf("Write() wrote %d bytes, want %d", n, output.limit)
	}
	if !errors.Is(err, errGitHeadMessageTooLarge) {
		t.Fatalf("Write() error = %v, want %v", err, errGitHeadMessageTooLarge)
	}
	if got := output.String(); got != "abcd" {
		t.Fatalf("buffer contents = %q, want %q", got, "abcd")
	}

	n, err = output.Write([]byte("more"))
	if n != 0 || !errors.Is(err, errGitHeadMessageTooLarge) {
		t.Fatalf("second Write() = (%d, %v), want (0, %v)", n, err, errGitHeadMessageTooLarge)
	}
}

func TestLimitedBufferAcceptsOutputWithinLimit(t *testing.T) {
	t.Parallel()

	output := limitedBuffer{limit: 4}
	n, err := output.Write([]byte("abcd"))
	if err != nil || n != 4 {
		t.Fatalf("Write() = (%d, %v), want (4, nil)", n, err)
	}
	if got := output.String(); got != "abcd" {
		t.Fatalf("buffer contents = %q, want %q", got, "abcd")
	}
}
