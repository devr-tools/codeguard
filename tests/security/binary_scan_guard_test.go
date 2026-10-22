package security_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

// TestScanSkipsBinaryContentForEveryLineDetector locks in the hoisted binary
// guard.
//
// The NUL-byte check used to live inside the secrets pass and had exactly one
// caller, so every other repository-wide line detector - weak hash, weak
// cipher, and the per-language shell-execution, dynamic-code and
// unsafe-HTML-sink patterns - happily regex-matched compiled binaries and
// reported findings at "line numbers" inside object code (a real report pointed
// at line 7321 of a linked executable). The guard now runs in the shared
// per-file scan path, so no detector can opt out of it.
//
// The identical content in a text file must still be reported; otherwise this
// test would pass just as well with detection switched off.
func TestScanSkipsBinaryContentForEveryLineDetector(t *testing.T) {
	// Content that trips several independent line detectors at once: a weak
	// hash construction, a weak cipher, and an AWS-shaped credential.
	detectable := "package app\n" +
		"\n" +
		"import (\n" +
		"\t\"crypto/des\"\n" +
		"\t\"crypto/md5\"\n" +
		")\n" +
		"\n" +
		"const accessKey = \"AKIA" + "1234567890ABCDEF" + "\"\n" +
		"\n" +
		"func Digest(data []byte) []byte {\n" +
		"\th := md5.New()\n" +
		"\th.Write(data)\n" +
		"\tblock, _ := des.NewCipher(data)\n" +
		"\t_ = block\n" +
		"\treturn h.Sum(nil)\n" +
		"}\n"

	root := t.TempDir()
	write := func(rel string, content []byte) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	// The text control.
	write("app/hash.go", []byte(detectable))
	// The same bytes with a NUL in the leading window, which is what an object
	// file or a linked executable in a build-cache tree looks like.
	write("app/compiled.o", append([]byte("\x7fELF\x02\x01\x01\x00"), detectable...))

	cfg := codeguard.ExampleConfig()
	cfg.Name = "binary-scan-guard"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: root, Language: "go"}}
	cfg.Checks.Security = true
	cfg.Checks.SecurityRules.GovulncheckMode = "off"
	cacheDisabled := false
	cfg.Cache.Enabled = &cacheDisabled

	report, err := codeguard.RunWithOptions(context.Background(), cfg, codeguard.ScanOptions{Mode: codeguard.ScanModeFull})
	if err != nil {
		t.Fatalf("full scan: %v", err)
	}

	if !reportHasFindingPath(report, "app/hash.go") {
		t.Fatal("no findings on the text control file; the detectors under test did not fire at all")
	}
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.Path == "app/compiled.o" {
				t.Fatalf("binary file produced a line finding: %s at line %d (%s)", finding.RuleID, finding.Line, finding.Message)
			}
		}
	}
}
