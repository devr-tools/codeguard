package checks_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/contracts"
	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const oversizedProtoBase = "syntax = \"proto3\";\n\nmessage User {\n  string name = 1;\n}\n\nmessage Legacy {\n  string id = 1;\n}\n"

// writeOversizedFile writes content and then extends the file (sparsely) to
// one byte past the scan cap, so the valid content is a prefix of a file that
// must not be read whole.
func writeOversizedFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Truncate(path, support.MaxTargetFileBytes+1); err != nil {
		t.Fatalf("truncate %s: %v", path, err)
	}
}

func TestReadCappedFileRejectsFileJustOverTheCap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	writeOversizedFile(t, path, "x")
	if data, err := support.ReadCappedFile(path); !errors.Is(err, support.ErrFileTooLarge) || data != nil {
		t.Fatalf("ReadCappedFile = (%d bytes, %v), want ErrFileTooLarge and no data", len(data), err)
	}
	if err := os.Truncate(path, support.MaxTargetFileBytes); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	if data, err := support.ReadCappedFile(path); err != nil || len(data) != support.MaxTargetFileBytes {
		t.Fatalf("ReadCappedFile(cap-sized) = (%d bytes, %v), want the whole file", len(data), err)
	}
}

func protoContractsEnv(t *testing.T, dir string) support.Context {
	t.Helper()
	on := true
	cfg := core.Config{Targets: []core.TargetConfig{{Name: "repo", Path: dir}}}
	cfg.Checks.ContractRules.ProtoBreaking = &on
	return support.Context{
		Config: cfg,
		Mode:   core.ScanModeDiff,
		ListChangedFiles: func(core.TargetConfig) ([]core.ChangedFile, error) {
			return []core.ChangedFile{{Path: "api.proto", Status: core.ChangedFileModified}}, nil
		},
		ReadBaseFile: func(core.TargetConfig, string) ([]byte, error) {
			return []byte(oversizedProtoBase), nil
		},
		NewFinding: func(input support.FindingInput) core.Finding {
			return core.Finding{RuleID: input.RuleID, Path: input.Path, Line: input.Line, Message: input.Message, Level: input.Level}
		},
		FinalizeSection: func(id string, name string, findings []core.Finding) core.SectionResult {
			return core.SectionResult{ID: id, Name: name, Findings: findings}
		},
	}
}

// The contracts head read takes its path from the PR diff. An oversized head
// file must be skipped, not read whole and not compared as an empty head
// (which would report every base message as removed).
func TestContractsSkipsOversizedHeadFile(t *testing.T) {
	dir := t.TempDir()
	head := filepath.Join(dir, "api.proto")

	// Control: a small head that drops Legacy is compared and flagged.
	if err := os.WriteFile(head, []byte("syntax = \"proto3\";\n\nmessage User {\n  string name = 1;\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	section := contracts.Run(context.Background(), protoContractsEnv(t, dir))
	if !containsMessage(section.Findings, "message Legacy was removed") {
		t.Fatalf("control findings = %+v, want Legacy removal", section.Findings)
	}

	// The same head content padded past the cap is skipped entirely.
	writeOversizedFile(t, head, "syntax = \"proto3\";\n\nmessage User {\n  string name = 1;\n}\n")
	section = contracts.Run(context.Background(), protoContractsEnv(t, dir))
	if len(section.Findings) != 0 {
		t.Fatalf("findings = %+v, want none for an oversized head file", section.Findings)
	}
}

func containsMessage(findings []core.Finding, want string) bool {
	for _, finding := range findings {
		if strings.Contains(finding.Message, want) {
			return true
		}
	}
	return false
}
