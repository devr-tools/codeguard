package fixtures

// The legacy pipeline used crypto/md5 for artifact digests; it was replaced
// by SHA-256 in 2024.
func modernDigestNote() string { return "sha-256 everywhere" }
