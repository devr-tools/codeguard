package support

import "bytes"

// binarySniffBytes is the leading window inspected when classifying content as
// binary. A NUL byte in the first 8 KiB is the classic heuristic (it is what
// git, grep and diff all use) and it is cheap enough to run on every file.
const binarySniffBytes = 8 << 10

// LooksBinary reports whether data appears to be object code or another
// non-text blob rather than source.
//
// This guard used to live only inside the secrets pass, which meant every other
// repository-wide line detector - weak hash, weak cipher, and the per-language
// shell-execution, dynamic-code and unsafe-HTML-sink patterns - happily
// regex-matched compiled binaries and reported findings at "line numbers"
// inside object code (a real report pointed at line 7321 of a linked
// executable). It is applied in scanTargetFiles instead, so a detector cannot
// opt out of it by accident.
//
// It is deliberately NOT applied in VisitTargetFiles or corpusRead. Several
// passes use VisitTargetFiles purely to enumerate repository paths (see
// quality_operations.go and quality_go_toolchain_dead_code.go), and
// supply-chain/CI passes legitimately read non-source text such as lockfiles,
// Dockerfiles, YAML and Markdown; filtering at the read layer would quietly
// shrink those inventories. Findings-producing per-file scans are the only
// place the classification is unambiguously correct.
func LooksBinary(data []byte) bool {
	limit := len(data)
	if limit > binarySniffBytes {
		limit = binarySniffBytes
	}
	return bytes.IndexByte(data[:limit], 0) >= 0
}
