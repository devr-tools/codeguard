package whatsnew_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/whatsnew"
)

const sampleChangelog = `# Changelog

## [0.6.1](https://github.com/devr-tools/codeguard/compare/v0.6.0...v0.6.1) (2026-07-01)


### Bug Fixes

* **security:** harden untrusted-input handling ([34c7f87](https://github.com/devr-tools/codeguard/commit/34c7f87))

## [0.6.0](https://github.com/devr-tools/codeguard/compare/v0.5.0...v0.6.0) (2026-06-30)

### Features

* earlier release bullet ([f2f6c61](https://github.com/devr-tools/codeguard/commit/f2f6c61))
`

func TestLatestFromChangelogParsesTopSection(t *testing.T) {
	rel, ok := whatsnew.LatestFromChangelog(sampleChangelog)
	if !ok {
		t.Fatal("expected a release to be found")
	}
	if rel.Version != "0.6.1" {
		t.Fatalf("version = %q, want 0.6.1", rel.Version)
	}
	if rel.Date != "2026-07-01" {
		t.Fatalf("date = %q, want 2026-07-01", rel.Date)
	}
	if len(rel.Highlights) != 1 {
		t.Fatalf("highlights = %v, want exactly the 0.6.1 bullet", rel.Highlights)
	}
	got := rel.Highlights[0]
	if strings.Contains(got, "**") || strings.Contains(got, "([") || strings.Contains(got, "earlier release") {
		t.Fatalf("highlight not cleaned/scoped: %q", got)
	}
	if got != "security: harden untrusted-input handling" {
		t.Fatalf("highlight = %q", got)
	}
}

func TestLatestFromChangelogDedupesAndCaps(t *testing.T) {
	var b strings.Builder
	b.WriteString("## [1.0.0](https://x/compare/v0.9.0...v1.0.0) (2026-01-01)\n\n### Features\n\n")
	b.WriteString("* dup bullet ([aaaaaaa](https://x/c/a))\n")
	b.WriteString("* dup bullet ([bbbbbbb](https://x/c/b))\n") // same cleaned text -> deduped
	for i := 0; i < 10; i++ {
		b.WriteString("* unique bullet number " + string(rune('a'+i)) + "\n")
	}
	rel, ok := whatsnew.LatestFromChangelog(b.String())
	if !ok {
		t.Fatal("expected release")
	}
	if len(rel.Highlights) > 5 {
		t.Fatalf("expected highlights capped at 5, got %d", len(rel.Highlights))
	}
	if rel.Highlights[0] != "dup bullet" {
		t.Fatalf("first highlight = %q", rel.Highlights[0])
	}
}

func TestLatestFromChangelogNoRelease(t *testing.T) {
	if _, ok := whatsnew.LatestFromChangelog("# Changelog\n\nnothing here\n"); ok {
		t.Fatal("expected ok=false when no release heading present")
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.6.1", "0.6.1", 0},
		{"v0.6.1", "0.6.1", 0},
		{"0.6.0", "0.6.1", -1},
		{"0.7.0", "0.6.1", 1},
		{"1.2", "1.2.1", -1},
		{"1.2.0", "1.2", 0},
		{"1.2.3-rc1", "1.2.3", 0},
	}
	for _, c := range cases {
		if got := whatsnew.CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestUpdateAvailable(t *testing.T) {
	if !whatsnew.UpdateAvailable("0.6.1", "0.7.0") {
		t.Error("expected update available for newer latest")
	}
	if whatsnew.UpdateAvailable("0.7.0", "0.6.1") {
		t.Error("did not expect update when current is newer")
	}
	if whatsnew.UpdateAvailable("0.6.1", "") {
		t.Error("empty latest must not report an update")
	}
	if whatsnew.UpdateAvailable("", "0.6.1") {
		t.Error("empty current must not report an update")
	}
}

func TestRenderShowsUpdateNotice(t *testing.T) {
	var buf bytes.Buffer
	rel := whatsnew.Release{Version: "0.7.0", Date: "2026-08-01", Highlights: []string{"shiny new thing"}}
	whatsnew.Render(&buf, "v0.6.1", "0.7.0", rel, false)
	out := buf.String()
	if !strings.Contains(out, "codeguard v0.6.1") {
		t.Errorf("missing current version: %s", out)
	}
	if !strings.Contains(out, "update available: v0.7.0") {
		t.Errorf("missing update notice: %s", out)
	}
	if !strings.Contains(out, "go install github.com/devr-tools/codeguard/cmd/codeguard@latest") {
		t.Errorf("missing upgrade hint: %s", out)
	}
	if !strings.Contains(out, "• shiny new thing") {
		t.Errorf("missing highlight: %s", out)
	}
}

func TestRenderNoUpdateShowsLatest(t *testing.T) {
	var buf bytes.Buffer
	whatsnew.Render(&buf, "0.6.1", "0.6.1", whatsnew.Release{Version: "0.6.1"}, false)
	out := buf.String()
	if !strings.Contains(out, "codeguard v0.6.1 (latest)") {
		t.Errorf("expected (latest) marker: %s", out)
	}
	if strings.Contains(out, "update available") {
		t.Errorf("did not expect update notice: %s", out)
	}
}

func TestRenderEmptyWritesNothing(t *testing.T) {
	var buf bytes.Buffer
	whatsnew.Render(&buf, "", "", whatsnew.Release{}, false)
	if buf.Len() != 0 {
		t.Errorf("expected no output, got: %q", buf.String())
	}
}

func TestRenderColorUsesDevrBlue(t *testing.T) {
	const devrBlue = "38;2;37;169;255"

	var colored bytes.Buffer
	whatsnew.Render(&colored, "0.6.1", "0.6.1", whatsnew.Release{Version: "0.6.1", Highlights: []string{"a thing"}}, true)
	if !strings.Contains(colored.String(), devrBlue) {
		t.Fatalf("expected devr blue ANSI code in colored output:\n%q", colored.String())
	}

	var plain bytes.Buffer
	whatsnew.Render(&plain, "0.6.1", "0.6.1", whatsnew.Release{Version: "0.6.1", Highlights: []string{"a thing"}}, false)
	if strings.Contains(plain.String(), "\x1b[") {
		t.Fatalf("expected no ANSI codes when color disabled:\n%q", plain.String())
	}
}

func TestColorForWriterPlainForNonTerminal(t *testing.T) {
	// A bytes.Buffer is not a terminal, so color must be disabled.
	if whatsnew.ColorForWriter(&bytes.Buffer{}) {
		t.Fatal("ColorForWriter must be false for a non-terminal writer")
	}
	// NO_COLOR forces plain even for os.Stdout.
	t.Setenv("NO_COLOR", "1")
	if whatsnew.ColorForWriter(os.Stdout) {
		t.Fatal("ColorForWriter must honor NO_COLOR")
	}
}
