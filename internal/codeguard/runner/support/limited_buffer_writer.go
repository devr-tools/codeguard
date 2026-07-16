package support

import "bytes"

// LimitedBufferWriter writes into a buffer until the byte budget is exhausted,
// then discards the rest while recording truncation.
type LimitedBufferWriter struct {
	buffer    *bytes.Buffer
	remaining int
	truncated bool
}

func NewLimitedBufferWriter(buffer *bytes.Buffer, limit int) *LimitedBufferWriter {
	return &LimitedBufferWriter{buffer: buffer, remaining: limit}
}

func (writer *LimitedBufferWriter) Write(p []byte) (int, error) {
	if writer.remaining <= 0 {
		writer.truncated = true
		return len(p), nil
	}
	if len(p) > writer.remaining {
		writer.buffer.Write(p[:writer.remaining])
		writer.remaining = 0
		writer.truncated = true
		return len(p), nil
	}
	n, err := writer.buffer.Write(p)
	writer.remaining -= n
	return n, err
}

func (writer *LimitedBufferWriter) Truncated() bool {
	return writer.truncated
}
