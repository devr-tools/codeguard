package mcp_test

import (
	"strings"
	"testing"
)

func TestMCPHostSmokeProfiles(t *testing.T) {
	type profile struct {
		name       string
		transcript string
		setup      func(*testing.T, string) map[string]string
		assertion  func(*testing.T, []string)
	}

	profiles := []profile{
		{
			name:       "editor-current",
			transcript: "testdata/transcripts/current_discovery.jsonl",
			setup:      setupPromptConfig,
			assertion:  assertCurrentDiscovery,
		},
		{
			name:       "editor-compat",
			transcript: "testdata/transcripts/compat_discovery.jsonl",
			setup:      setupPromptConfig,
			assertion:  assertCompatDiscovery,
		},
		{
			name:       "review-agent",
			transcript: "testdata/transcripts/validate_patch_prompt_secret.jsonl",
			setup:      setupPromptConfig,
			assertion:  assertValidatePatchProfile,
		},
		{
			name:       "scan-agent",
			transcript: "testdata/transcripts/scan_progress_cancel.jsonl",
			setup:      setupCancelableScanConfig,
			assertion:  assertScanProgressCancel,
		},
		{
			name:       "resource-agent",
			transcript: "testdata/transcripts/resources_discovery.jsonl",
			setup:      setupPromptConfig,
			assertion:  assertResourcesDiscovery,
		},
		{
			name:       "prompt-agent",
			transcript: "testdata/transcripts/prompts_discovery.jsonl",
			setup:      setupPromptConfig,
			assertion:  assertPromptsDiscovery,
		},
		{
			name:       "streaming-agent",
			transcript: "testdata/transcripts/scan_streaming.jsonl",
			setup:      setupStreamingConfig,
			assertion:  assertScanStreaming,
		},
		{
			name:       "verify-fix-agent",
			transcript: "testdata/transcripts/verify_fix_failclosed.jsonl",
			setup:      setupPromptConfig,
			assertion:  assertVerifyFixFailsClosed,
		},
		{
			name:       "apply-fix-agent",
			transcript: "testdata/transcripts/apply_fix_failclosed.jsonl",
			setup:      setupPromptConfig,
			assertion:  assertApplyFixFailsClosed,
		},
	}

	for _, profile := range profiles {
		t.Run(profile.name, func(t *testing.T) {
			dir := t.TempDir()
			replacements := profile.setup(t, dir)
			configPath := replacements["__CONFIG_PATH__"]
			transcript := loadTranscript(t, profile.transcript, replacements)
			lines, stderr := runTranscriptThroughSubprocess(t, configPath, transcript)
			if strings.TrimSpace(stderr) != "" {
				t.Fatalf("expected empty stderr, got: %s", stderr)
			}
			profile.assertion(t, lines)
		})
	}
}
