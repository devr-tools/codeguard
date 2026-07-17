package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func designFinding(t *testing.T, report codeguard.Report, ruleID string) codeguard.Finding {
	t.Helper()
	for _, section := range report.Sections {
		if section.Name != "Design Patterns" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == ruleID {
				return finding
			}
		}
	}
	t.Fatalf("finding %q not found", ruleID)
	return codeguard.Finding{}
}

func TestDesignGoLayerBoundaryUsesImportingFileAndLine(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/layers\n\ngo 1.23.0\n")
	writeFile(t, filepath.Join(dir, "internal", "domain", "model.go"), "package domain\n")
	writeFile(t, filepath.Join(dir, "internal", "adapters", "store.go"), "package adapters\n")
	writeFile(t, filepath.Join(dir, "internal", "domain", "service.go"), "package domain\n\nimport _ \"example.com/layers/internal/adapters\"\n")
	writeFile(t, filepath.Join(dir, "misc", "orphan.go"), "package misc\n")

	required := true
	cfg := graphTestConfig("design-go-layer-boundary", dir, "go")
	cfg.Checks.DesignRules.RequireBoundaryAssignment = &required
	cfg.Checks.DesignRules.Layers = []codeguard.DesignLayerConfig{
		{Name: "domain", Paths: []string{"internal/domain/**"}, MayDependOn: []string{"domain"}, DeniedExternal: []string{"database/sql"}},
		{Name: "adapters", Paths: []string{"internal/adapters/**"}, MayDependOn: []string{"domain"}},
	}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	boundary := designFinding(t, report, "design.layer-boundary")
	if boundary.Path != "internal/domain/service.go" || boundary.Line != 3 {
		t.Fatalf("layer finding location = %s:%d, want internal/domain/service.go:3", boundary.Path, boundary.Line)
	}
	if !strings.Contains(boundary.Message, `layer "domain"`) || !strings.Contains(boundary.Message, `layer "adapters"`) {
		t.Fatalf("layer finding lacks exact boundary names: %q", boundary.Message)
	}
	unassigned := designFinding(t, report, "design.unassigned-module")
	if unassigned.Path != "misc/orphan.go" {
		t.Fatalf("unassigned finding path = %q, want misc/orphan.go", unassigned.Path)
	}
}

func TestDesignTypeScriptDomainAndDataOwnershipBoundaries(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "orders", "contracts", "order.ts"), "export interface Order { id: string }\n")
	writeFile(t, filepath.Join(dir, "src", "orders", "data", "store.ts"), "export const table = 'orders';\n")
	writeFile(t, filepath.Join(dir, "src", "billing", "allowed.ts"), "import type { Order } from '../orders/contracts/order';\nexport const id = (o: Order) => o.id;\n")
	writeFile(t, filepath.Join(dir, "src", "billing", "forbidden.ts"), "// billing service\nimport { table } from '../orders/data/store';\nexport const source = table;\n")

	cfg := graphTestConfig("design-ts-domains", dir, "typescript")
	cfg.Checks.DesignRules.Domains = []codeguard.DesignDomainConfig{
		{Name: "billing", Paths: []string{"src/billing/**"}, MayDependOn: []string{"orders"}},
		{Name: "orders", Paths: []string{"src/orders/**"}, PublicPaths: []string{"src/orders/contracts/**"}, DataPaths: []string{"src/orders/data/**"}},
	}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	for _, ruleID := range []string{"design.domain-boundary", "design.data-ownership"} {
		finding := designFinding(t, report, ruleID)
		if finding.Path != "src/billing/forbidden.ts" || finding.Line != 2 {
			t.Fatalf("%s location = %s:%d, want src/billing/forbidden.ts:2", ruleID, finding.Path, finding.Line)
		}
	}
}

func TestDesignLayerDeniedExternalImport(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/external-layer\n\ngo 1.23.0\n")
	writeFile(t, filepath.Join(dir, "internal", "domain", "query.go"), "package domain\n\nimport _ \"database/sql\"\n")
	cfg := graphTestConfig("design-layer-external", dir, "go")
	cfg.Checks.DesignRules.Layers = []codeguard.DesignLayerConfig{{
		Name: "domain", Paths: []string{"internal/domain/**"}, DeniedExternal: []string{"database/sql"},
	}}

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	finding := designFinding(t, report, "design.layer-boundary")
	if finding.Path != "internal/domain/query.go" || finding.Line != 3 || !strings.Contains(finding.Message, "database/sql") {
		t.Fatalf("external layer finding = %s:%d %q", finding.Path, finding.Line, finding.Message)
	}
}

func TestDesignCapabilityBoundaryCoversSupportedLanguageImports(t *testing.T) {
	tests := []struct {
		name       string
		language   string
		file       string
		source     string
		pattern    string
		line       int
		extraFiles map[string]string
	}{
		{name: "go", language: "go", file: "internal/domain/service.go", source: "package domain\n\nimport _ \"database/sql\"\n", pattern: "database/sql", line: 3, extraFiles: map[string]string{"go.mod": "module example.com/capability\n\ngo 1.23.0\n"}},
		{name: "typescript", language: "typescript", file: "src/domain/service.ts", source: "import { S3Client } from '@aws-sdk/client-s3';\nexport const client = S3Client;\n", pattern: "@aws-sdk/**", line: 1},
		{name: "python", language: "python", file: "src/domain/service.py", source: "import boto3\n", pattern: "boto3", line: 1},
		{name: "rust", language: "rust", file: "src/domain/service.rs", source: "use aws_sdk_s3::Client;\n", pattern: "aws_sdk_s3/**", line: 1},
		{name: "java", language: "java", file: "src/domain/Service.java", source: "package domain;\nimport software.amazon.awssdk.services.s3.S3Client;\npublic class Service {}\n", pattern: "software.amazon.awssdk.**", line: 2},
		{name: "cpp", language: "cpp", file: "src/domain/service.cpp", source: "#include <aws/s3/S3Client.h>\n", pattern: "aws/**", line: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, filepath.Join(dir, filepath.FromSlash(tt.file)), tt.source)
			for file, content := range tt.extraFiles {
				writeFile(t, filepath.Join(dir, file), content)
			}
			cfg := graphTestConfig("design-capability-"+tt.name, dir, tt.language)
			cfg.Checks.DesignRules.Capabilities = []codeguard.DesignCapabilityConfig{{
				Name: "cloud-or-database", Imports: []string{tt.pattern}, AllowedPaths: []string{"src/adapters/**", "internal/adapters/**"},
			}}

			report, err := codeguard.Run(context.Background(), cfg)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			finding := designFinding(t, report, "design.capability-boundary")
			if finding.Path != filepath.ToSlash(tt.file) || finding.Line != tt.line {
				t.Fatalf("capability finding location = %s:%d, want %s:%d", finding.Path, finding.Line, filepath.ToSlash(tt.file), tt.line)
			}
		})
	}
}

func TestDesignRustUsesLanguageNeutralLayerPolicy(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "lib.rs"), "mod domain;\nmod adapters;\n")
	writeFile(t, filepath.Join(dir, "src", "domain.rs"), "use crate::adapters::Store;\n")
	writeFile(t, filepath.Join(dir, "src", "adapters.rs"), "pub struct Store;\n")

	cfg := graphTestConfig("design-rust-layer", dir, "rust")
	cfg.Checks.DesignRules.Layers = []codeguard.DesignLayerConfig{
		{Name: "domain", Paths: []string{"src/domain.rs"}, DenyDependOn: []string{"adapters"}},
		{Name: "adapters", Paths: []string{"src/adapters.rs"}},
	}
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	finding := designFinding(t, report, "design.layer-boundary")
	if finding.Path != "src/domain.rs" || finding.Line != 1 {
		t.Fatalf("Rust boundary location = %s:%d, want src/domain.rs:1", finding.Path, finding.Line)
	}
}

func TestDesignArchitectureRulesAreOptIn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "service.ts"), "import '@aws-sdk/client-s3';\n")
	report, err := codeguard.Run(context.Background(), graphTestConfig("design-boundaries-opt-in", dir, "typescript"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	for _, ruleID := range []string{"design.layer-boundary", "design.unassigned-module", "design.domain-boundary", "design.data-ownership", "design.capability-boundary"} {
		assertFindingRuleAbsent(t, report, "Design Patterns", ruleID)
	}
}
