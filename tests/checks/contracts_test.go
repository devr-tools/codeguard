package checks_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestContractsGoExportedBreaking(t *testing.T) {
	dir := initContractsRepo(t)
	writeFile(t, filepath.Join(dir, "api.go"), strings.Join([]string{
		"package api",
		"",
		"const Version = \"1\"",
		"",
		"type Client struct{}",
		"",
		"type Legacy struct{}",
		"",
		"func New(addr string) *Client { return &Client{} }",
		"",
		"func Helper() {}",
		"",
		"func (c *Client) Do(x int) error { return nil }",
		"",
	}, "\n"))
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "api.go"), strings.Join([]string{
		"package api",
		"",
		"type Client struct{}",
		"",
		"func New(addr string, timeout int) *Client { return &Client{} }",
		"",
		"func (c *Client) Do(x int) error { return nil }",
		"",
	}, "\n"))

	report := runContractsDiff(t, contractsTestConfig(dir))
	assertSectionStatus(t, report, "API Contracts", "fail")
	messages := contractsRuleMessages(report, "contracts.go-exported-breaking")
	assertMessageContaining(t, messages, "func Helper was removed")
	assertMessageContaining(t, messages, "const Version was removed")
	assertMessageContaining(t, messages, "type Legacy was removed")
	assertMessageContaining(t, messages, "func New changed signature from (string) (*Client) to (string, int) (*Client)")
}

func TestContractsGoExportedBreakingOnDeletedFile(t *testing.T) {
	dir := initContractsRepo(t)
	writeFile(t, filepath.Join(dir, "removed.go"), "package api\n\nfunc Gone() {}\n")
	writeFile(t, filepath.Join(dir, "kept.go"), "package api\n\nfunc Kept() {}\n")
	commitAll(t, dir, "base")
	if err := os.Remove(filepath.Join(dir, "removed.go")); err != nil {
		t.Fatalf("remove file: %v", err)
	}

	report := runContractsDiff(t, contractsTestConfig(dir))
	assertSectionStatus(t, report, "API Contracts", "fail")
	messages := contractsRuleMessages(report, "contracts.go-exported-breaking")
	assertMessageContaining(t, messages, "func Gone was removed")
}

func TestContractsOpenAPIBreaking(t *testing.T) {
	dir := initContractsRepo(t)
	writeFile(t, filepath.Join(dir, "openapi.yaml"), strings.Join([]string{
		"openapi: 3.0.0",
		"info: {title: pets, version: \"1.0\"}",
		"paths:",
		"  /pets:",
		"    get:",
		"      parameters:",
		"        - {name: limit, in: query, required: false, schema: {type: integer}}",
		"      responses:",
		"        \"200\": {description: ok}",
		"        \"404\": {description: missing}",
		"    post:",
		"      requestBody:",
		"        content:",
		"          application/json:",
		"            schema:",
		"              type: object",
		"              required: [name]",
		"              properties: {name: {type: string}, age: {type: integer}}",
		"      responses:",
		"        \"201\": {description: created}",
		"    delete:",
		"      responses:",
		"        \"204\": {description: deleted}",
		"  /owners:",
		"    get:",
		"      responses:",
		"        \"200\": {description: ok}",
		"",
	}, "\n"))
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "openapi.yaml"), strings.Join([]string{
		"openapi: 3.0.0",
		"info: {title: pets, version: \"2.0\"}",
		"paths:",
		"  /pets:",
		"    get:",
		"      parameters:",
		"        - {name: limit, in: query, required: true, schema: {type: integer}}",
		"      responses:",
		"        \"200\": {description: ok}",
		"    post:",
		"      requestBody:",
		"        content:",
		"          application/json:",
		"            schema:",
		"              type: object",
		"              required: [name, age]",
		"              properties: {name: {type: string}, age: {type: integer}}",
		"      responses:",
		"        \"201\": {description: created}",
		"",
	}, "\n"))

	report := runContractsDiff(t, contractsTestConfig(dir))
	assertSectionStatus(t, report, "API Contracts", "fail")
	messages := contractsRuleMessages(report, "contracts.openapi-breaking")
	assertMessageContaining(t, messages, "path /owners was removed")
	assertMessageContaining(t, messages, "operation DELETE /pets was removed")
	assertMessageContaining(t, messages, "response code 404 was removed from GET /pets")
	assertMessageContaining(t, messages, "parameter limit (query) is newly required on GET /pets")
	assertMessageContaining(t, messages, "request field \"age\" (application/json) is newly required on POST /pets")
}

func TestContractsProtoBreaking(t *testing.T) {
	dir := initContractsRepo(t)
	writeFile(t, filepath.Join(dir, "api.proto"), strings.Join([]string{
		"syntax = \"proto3\";",
		"",
		"package demo;",
		"",
		"message User {",
		"  string name = 1;",
		"  int32 age = 2;",
		"  string email = 3;",
		"}",
		"",
		"message Legacy {",
		"  string id = 1;",
		"}",
		"",
		"message UserRequest {",
		"  string id = 1;",
		"}",
		"",
		"service UserService {",
		"  rpc GetUser (UserRequest) returns (User);",
		"  rpc DeleteUser (UserRequest) returns (User);",
		"}",
		"",
	}, "\n"))
	commitAll(t, dir, "base")

	writeFile(t, filepath.Join(dir, "api.proto"), strings.Join([]string{
		"syntax = \"proto3\";",
		"",
		"package demo;",
		"",
		"message User {",
		"  string name = 4;",
		"  int64 age = 2;",
		"}",
		"",
		"message UserRequest {",
		"  string id = 1;",
		"}",
		"",
		"service UserService {",
		"  rpc GetUser (UserRequest) returns (User);",
		"}",
		"",
	}, "\n"))

	report := runContractsDiff(t, contractsTestConfig(dir))
	assertSectionStatus(t, report, "API Contracts", "fail")
	messages := contractsRuleMessages(report, "contracts.proto-breaking")
	assertMessageContaining(t, messages, "field User.name was renumbered from 1 to 4")
	assertMessageContaining(t, messages, "field User.age changed type from int32 to int64")
	assertMessageContaining(t, messages, "field User.email was removed")
	assertMessageContaining(t, messages, "message Legacy was removed")
	assertMessageContaining(t, messages, "rpc DeleteUser was removed from service UserService")
}

func TestContractsMigrationDestructiveFlagsNewMigrationsOnly(t *testing.T) {
	dir := initContractsRepo(t)
	writeFile(t, filepath.Join(dir, "migrations", "0001_init.sql"), "CREATE TABLE users (id INT);\n")
	commitAll(t, dir, "base")

	// Modified (not new) migration files must not be flagged in diff mode.
	writeFile(t, filepath.Join(dir, "migrations", "0001_init.sql"), "CREATE TABLE users (id INT);\nDROP TABLE archived;\n")
	writeFile(t, filepath.Join(dir, "migrations", "0002_cleanup.sql"), strings.Join([]string{
		"ALTER TABLE users DROP COLUMN email;",
		"TRUNCATE TABLE sessions;",
		"ALTER TABLE users ALTER COLUMN name SET NOT NULL;",
		"ALTER TABLE users ADD COLUMN bio TEXT NOT NULL DEFAULT '';",
		"DROP TABLE legacy;",
		"",
	}, "\n"))
	// Stage the new file so the worktree diff against main reports it as added.
	runGit(t, dir, "add", ".")

	report := runContractsDiff(t, contractsTestConfig(dir))
	assertSectionStatus(t, report, "API Contracts", "warn")
	findings := contractsRuleFindings(report, "contracts.migration-destructive")
	if len(findings) != 4 {
		t.Fatalf("migration findings = %d, want 4: %+v", len(findings), findings)
	}
	for _, finding := range findings {
		if finding.Path != "migrations/0002_cleanup.sql" {
			t.Fatalf("unexpected finding path %q (only the new migration should be flagged)", finding.Path)
		}
	}
	messages := contractsRuleMessages(report, "contracts.migration-destructive")
	assertMessageContaining(t, messages, "DROP COLUMN")
	assertMessageContaining(t, messages, "TRUNCATE")
	assertMessageContaining(t, messages, "ALTER ... NOT NULL without DEFAULT")
	assertMessageContaining(t, messages, "DROP TABLE")
}

func TestContractsFullScanRunsOnlyMigrationRule(t *testing.T) {
	dir := t.TempDir() // no git repo: base-comparison rules must no-op
	writeFile(t, filepath.Join(dir, "api.go"), "package api\n\nfunc Exported() {}\n")
	writeFile(t, filepath.Join(dir, "openapi.yaml"), "openapi: 3.0.0\npaths: {}\n")
	writeFile(t, filepath.Join(dir, "db", "migrate", "20240101_drop.sql"), "DROP TABLE old_users;\n")

	cfg := contractsTestConfig(dir)
	contractsOn := true
	cfg.Checks.Contracts = &contractsOn

	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("full scan: %v", err)
	}
	assertSectionStatus(t, report, "API Contracts", "warn")
	if findings := contractsRuleFindings(report, "contracts.migration-destructive"); len(findings) != 1 {
		t.Fatalf("migration findings = %d, want 1", len(findings))
	}
	for _, section := range report.Sections {
		for _, finding := range section.Findings {
			if finding.RuleID != "contracts.migration-destructive" {
				t.Fatalf("unexpected non-migration finding in full scan: %s", finding.RuleID)
			}
		}
	}
}

func TestContractsDisabledByDefaultInFullScan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "migrations", "0001.sql"), "DROP TABLE x;\n")

	report, err := codeguard.Run(context.Background(), contractsTestConfig(dir))
	if err != nil {
		t.Fatalf("full scan: %v", err)
	}
	for _, section := range report.Sections {
		if section.Name == "API Contracts" {
			t.Fatal("expected API Contracts section to be omitted in full scans by default")
		}
	}
}

func TestContractsEnabledByStrictProfile(t *testing.T) {
	cfg, err := codeguard.ExampleConfigForProfile("strict")
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if cfg.Checks.Contracts == nil || !*cfg.Checks.Contracts {
		t.Fatal("expected strict profile to enable contracts checks")
	}
}
