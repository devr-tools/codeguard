package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/cli"
)

func TestUsageMenuShowsWhatsNewBanner(t *testing.T) {
	// Keep the test offline and deterministic: no upstream update check.
	t.Setenv("CODEGUARD_NO_UPDATE_CHECK", "1")

	var stdout, stderr bytes.Buffer
	code := cli.Run(nil, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "codeguard v") {
		t.Fatalf("expected version banner in menu, got:\n%s", out)
	}
	if !strings.Contains(out, "What's new") {
		t.Fatalf("expected What's new section in menu, got:\n%s", out)
	}
	// The usage text must still follow the banner.
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("expected usage text after banner, got:\n%s", out)
	}
}

func TestMenuGroupsAndCommands(t *testing.T) {
	t.Setenv("CODEGUARD_NO_UPDATE_CHECK", "1")

	var stdout, stderr bytes.Buffer
	if code := cli.Run(nil, strings.NewReader(""), &stdout, &stderr); code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()

	for _, group := range []string{"GET STARTED", "SCAN & BASELINE", "RULES & POLICY", "SERVE & DIAGNOSE"} {
		if !strings.Contains(out, group) {
			t.Errorf("menu missing group %q:\n%s", group, out)
		}
	}
	for _, cmd := range []string{"scan", "scan-history", "explain", "serve", "doctor"} {
		if !strings.Contains(out, cmd) {
			t.Errorf("menu missing command %q", cmd)
		}
	}
	if !strings.Contains(out, "Common flags") {
		t.Errorf("menu missing Common flags section:\n%s", out)
	}
	if !strings.Contains(out, "codeguard <command> -h") {
		t.Errorf("menu missing per-command help hint:\n%s", out)
	}
	// Descriptions from the old flag-dump format should be gone.
	if strings.Contains(out, "-base-ref main]") {
		t.Errorf("menu still contains the old dense flag signatures:\n%s", out)
	}
}

func TestHelpCommandShowsBanner(t *testing.T) {
	t.Setenv("CODEGUARD_NO_UPDATE_CHECK", "1")

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"help"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "What's new") {
		t.Fatalf("expected banner on help, got:\n%s", stdout.String())
	}
}
