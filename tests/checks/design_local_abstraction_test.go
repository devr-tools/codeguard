package checks_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func designLocalConfig(dir string, language string) codeguard.Config {
	cfg := codeguard.ExampleConfig()
	cfg.Name = "design-local-abstraction"
	cfg.Targets = []codeguard.TargetConfig{{Name: "repo", Path: dir, Language: language}}
	cfg.Checks.Design = true
	cfg.Checks.Quality = false
	cfg.Checks.Security = false
	cfg.Checks.Prompts = false
	cfg.Checks.CI = false
	off := false
	cfg.Checks.Context = &off
	cfg.Cache.Enabled = &off
	cfg.Checks.DesignRules.MaxDeclsPerFile = 2
	cfg.Checks.DesignRules.MaxInterfaceMethods = 2
	return cfg
}

func runDesignLocalScan(t *testing.T, cfg codeguard.Config) codeguard.Report {
	t.Helper()
	report, err := codeguard.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return report
}

func TestDesignLocalAbstractionRules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pkg", "domain", "service.go"), strings.Join([]string{
		"package domain",
		"",
		"import \"database/sql\"",
		"",
		"type Config struct{}",
		"type Order struct{}",
		"type Repository struct { DB *sql.DB }",
		"",
		"func PublicA() error {",
		"\treturn service.Save()",
		"}",
		"",
		"func PublicB() error {",
		"\treturn repo.Save()",
		"}",
		"",
		"func Configure(cfg Config) string {",
		"\treturn os.Getenv(\"REGION\")",
		"}",
		"",
		"func SendOrder(client Client) {",
		"\tclient.Init()",
		"\tclient.Send()",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "pkg", "api", "user.go"), strings.Join([]string{
		"package api",
		"",
		"type UserRecord struct{}",
		"func GetUser() UserRecord { return UserRecord{} }",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "pkg", "api", "order_handler.go"), strings.Join([]string{
		"package api",
		"",
		"func HandleOrder(order Order) {",
		"\tif order.Status == \"paid\" {",
		"\t\trepo.Save(order)",
		"\t}",
		"\tif order.Amount > 1000 {",
		"\t\trepo.Update(order)",
		"\t}",
		"}",
	}, "\n"))

	report := runDesignLocalScan(t, designLocalConfig(dir, "go"))

	for _, ruleID := range []string{
		"design.shallow-module",
		"design.excessive-public-surface",
		"design.pass-through-abstraction",
		"design.configuration-leak",
		"design.temporal-coupling",
		"design.infrastructure-type-leak",
		"design.persistence-model-leak",
		"design.domain-logic-in-handler",
	} {
		assertFindingRulePresent(t, report, "Design Patterns", ruleID)
	}
	assertFindingConfidence(t, report, "Design Patterns", "design.infrastructure-type-leak", "high")
}

func TestDesignLocalAbstractionAdditionalLanguages(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "domain", "payment.ts"), strings.Join([]string{
		"export function savePayment(req: express.Request) {",
		"  return service.save(req);",
		"}",
		"export function publishPayment(client: Client) {",
		"  client.init();",
		"  client.send();",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "app", "controllers", "order_controller.py"), strings.Join([]string{
		"def handle_order(order):",
		"    if order.status == 'paid':",
		"        repo.save(order)",
		"    if order.amount > 1000:",
		"        repo.update(order)",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "include", "api", "user.hpp"), strings.Join([]string{
		"class UserRecord {};",
		"UserRecord GetUser();",
	}, "\n"))

	tsReport := runDesignLocalScan(t, designLocalConfig(dir, "typescript"))
	assertFindingRulePresent(t, tsReport, "Design Patterns", "design.infrastructure-type-leak")
	assertFindingRulePresent(t, tsReport, "Design Patterns", "design.temporal-coupling")

	pythonReport := runDesignLocalScan(t, designLocalConfig(dir, "python"))
	assertFindingRulePresent(t, pythonReport, "Design Patterns", "design.domain-logic-in-handler")

	cppReport := runDesignLocalScan(t, designLocalConfig(dir, "cpp"))
	assertFindingRulePresent(t, cppReport, "Design Patterns", "design.persistence-model-leak")
}

func TestDesignPersistenceModelLeakAllowsGeneratedPrismaEnums(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps", "web", "app", "contracts", "_components", "contracts-filters.ts"), strings.Join([]string{
		"import { ContractStatus, ContractType } from '@prisma/client';",
		"export const statusLabels: Record<ContractStatus, string> = {",
		"  DRAFT: 'Draft',",
		"} as Record<ContractStatus, string>;",
		"export type ContractFilter = { status?: ContractStatus; type?: ContractType };",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "apps", "web", "app", "api", "contracts", "route.ts"), strings.Join([]string{
		"export type ContractModel = { id: string };",
		"export async function GET() {",
		"  return Response.json({ ok: true });",
		"}",
	}, "\n"))

	report := runDesignLocalScan(t, designLocalConfig(dir, "typescript"))

	assertFindingRulePresent(t, report, "Design Patterns", "design.persistence-model-leak")
	for _, section := range report.Sections {
		if section.Name != "Design Patterns" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == "design.persistence-model-leak" && strings.Contains(finding.Path, "contracts-filters.ts") {
				t.Fatalf("generated Prisma enum import should not leak persistence model: %+v", finding)
			}
		}
	}
}

func TestDesignPersistenceModelLeakSkipsFrontendUIProps(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps", "web", "app", "contracts", "_components", "contract-card.tsx"), strings.Join([]string{
		"export type ContractCardProps = {",
		"  contract: ContractModel;",
		"};",
		"type ContractModel = { id: string };",
		"export function ContractCard(props: ContractCardProps) {",
		"  return <span>{props.contract.id}</span>;",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages", "api", "contracts", "contract.ts"), strings.Join([]string{
		"export type ContractModel = { id: string };",
	}, "\n"))

	report := runDesignLocalScan(t, designLocalConfig(dir, "typescript"))

	assertFindingRulePresent(t, report, "Design Patterns", "design.persistence-model-leak")
	for _, section := range report.Sections {
		if section.Name != "Design Patterns" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == "design.persistence-model-leak" && strings.Contains(finding.Path, "contract-card.tsx") {
				t.Fatalf("frontend UI props should not leak persistence model: %+v", finding)
			}
		}
	}
}

func TestDesignPersistenceModelLeakSkipsReactAndReactNativePresentationPaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps", "web", "app", "(legal)", "contracts", "[id]", "_components", "contract-detail.tsx"), strings.Join([]string{
		"import type { ContractStatus } from '@prisma/client';",
		"type ContractModel = { id: string; status: ContractStatus };",
		"export type ContractDetailProps = {",
		"  contract: ContractModel;",
		"};",
		"export const contractStatusLabels: Record<ContractStatus, string> = {",
		"  DRAFT: 'Draft',",
		"};",
		"export function ContractDetail(props: ContractDetailProps) {",
		"  return <span>{props.contract.id}</span>;",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "apps", "mobile", "src", "screens", "contracts", "ContractDetailsScreen.tsx"), strings.Join([]string{
		"import type { ContractStatus } from '@prisma/client';",
		"type ContractModel = { id: string; status: ContractStatus };",
		"export type ContractDetailsScreenProps = {",
		"  contract: ContractModel;",
		"};",
		"export const contractStatusOptions: Array<{ value: ContractStatus; label: string }> = [];",
		"export function ContractDetailsScreen(props: ContractDetailsScreenProps) {",
		"  return <Text>{props.contract.id}</Text>;",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages", "ui", "src", "components", "ContractBadge.tsx"), strings.Join([]string{
		"type ContractModel = { id: string };",
		"export type ContractBadgeProps = { contract: ContractModel };",
		"export function ContractBadge(props: ContractBadgeProps) {",
		"  return <span>{props.contract.id}</span>;",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages", "api", "contracts", "public.ts"), strings.Join([]string{
		"export type ContractModel = { id: string };",
	}, "\n"))

	report := runDesignLocalScan(t, designLocalConfig(dir, "typescript"))

	assertFindingRulePresent(t, report, "Design Patterns", "design.persistence-model-leak")
	for _, section := range report.Sections {
		if section.Name != "Design Patterns" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID != "design.persistence-model-leak" {
				continue
			}
			if strings.Contains(finding.Path, "contract-detail.tsx") ||
				strings.Contains(finding.Path, "ContractDetailsScreen.tsx") ||
				strings.Contains(finding.Path, "ContractBadge.tsx") {
				t.Fatalf("React/React Native presentation path should not leak persistence model: %+v", finding)
			}
		}
	}
}

func TestDesignPersistenceModelLeakSkipsNestJSDTOControllerBoundaries(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps", "api", "src", "users", "users.controller.ts"), strings.Join([]string{
		"import { Body, Controller, Post } from '@nestjs/common';",
		"@Controller('users')",
		"export class UsersController {",
		"  @Post()",
		"  async create(@Body() data: CreateUserDto): Promise<UserResponseDto> {",
		"    return this.usersService.create(data);",
		"  }",
		"}",
		"export interface CreateUserDto { id: string }",
		"export interface UserResponseDto { id: string }",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "apps", "api", "src", "users", "users.route.ts"), strings.Join([]string{
		"export type UserEntity = { id: string };",
		"export async function GET(): Promise<UserEntity> {",
		"  return db.user.findFirst();",
		"}",
	}, "\n"))

	report := runDesignLocalScan(t, designLocalConfig(dir, "typescript"))

	assertFindingRulePresent(t, report, "Design Patterns", "design.persistence-model-leak")
	for _, section := range report.Sections {
		if section.Name != "Design Patterns" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == "design.persistence-model-leak" && strings.Contains(finding.Path, "users.controller.ts") {
				t.Fatalf("NestJS DTO controller boundary should not leak persistence model: %+v", finding)
			}
		}
	}
}

func TestDesignPersistenceModelLeakAllowsPackageAPIImplementationBoundary(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "packages", "api", "src", "claims", "repository.ts"), strings.Join([]string{
		"import { PrismaClient } from '@prisma/client';",
		"export async function loadClaimRows(db: PrismaClient) {",
		"  // Convert database row shapes before returning API DTOs.",
		"  const rows = await db.claim.findMany();",
		"  return rows.map((row) => ({ id: row.id }));",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages", "domain", "claims", "model.ts"), strings.Join([]string{
		"export type ClaimModel = { id: string };",
	}, "\n"))

	report := runDesignLocalScan(t, designLocalConfig(dir, "typescript"))

	assertFindingRulePresent(t, report, "Design Patterns", "design.persistence-model-leak")
	for _, section := range report.Sections {
		if section.Name != "Design Patterns" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID == "design.persistence-model-leak" && strings.Contains(finding.Path, "packages/api/src") {
				t.Fatalf("packages/api/src is an implementation boundary and should allow persistence vocabulary: %+v", finding)
			}
		}
	}
}

func TestDesignPersistenceModelLeakIgnoresCommentsTestsAndStubs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps", "web", "app", "api", "claims", "route.test.ts"), strings.Join([]string{
		"// Prisma row comment in test should not be a boundary leak.",
		"export type ClaimRow = { id: string };",
		"export const prismaStub = { claim: { findMany: async () => [] } };",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "apps", "web", "app", "api", "claims", "__fixtures__", "row-stub.ts"), strings.Join([]string{
		"// test fixture mentions PrismaClient and row shapes only for stubs.",
		"export type ClaimRecord = { id: string };",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "apps", "web", "app", "api", "claims", "route.ts"), strings.Join([]string{
		"export type ClaimModel = { id: string };",
		"export async function GET() {",
		"  return Response.json({ ok: true });",
		"}",
	}, "\n"))

	report := runDesignLocalScan(t, designLocalConfig(dir, "typescript"))

	assertFindingRulePresent(t, report, "Design Patterns", "design.persistence-model-leak")
	for _, section := range report.Sections {
		if section.Name != "Design Patterns" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID != "design.persistence-model-leak" {
				continue
			}
			if strings.Contains(finding.Path, "route.test.ts") || strings.Contains(finding.Path, "__fixtures__") {
				t.Fatalf("comments/tests/stubs should not produce persistence leak findings: %+v", finding)
			}
		}
	}
}

func TestDesignPersistenceModelLeakKeepsAPIAndDomainBoundaries(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "apps", "web", "app", "api", "contracts", "route.ts"), strings.Join([]string{
		"export type ContractModel = { id: string };",
		"export async function GET() {",
		"  return Response.json({ ok: true });",
		"}",
	}, "\n"))
	writeFile(t, filepath.Join(dir, "packages", "domain", "contracts", "model.ts"), strings.Join([]string{
		"export type ContractRecord = { id: string };",
	}, "\n"))

	report := runDesignLocalScan(t, designLocalConfig(dir, "typescript"))

	assertFindingRulePresent(t, report, "Design Patterns", "design.persistence-model-leak")
	foundAPI := false
	foundDomain := false
	for _, section := range report.Sections {
		if section.Name != "Design Patterns" {
			continue
		}
		for _, finding := range section.Findings {
			if finding.RuleID != "design.persistence-model-leak" {
				continue
			}
			if !strings.Contains(finding.Message, "Contract") {
				t.Fatalf("persistence leak finding should include source context: %+v", finding)
			}
			foundAPI = foundAPI || strings.Contains(finding.Path, "route.ts")
			foundDomain = foundDomain || strings.Contains(finding.Path, "model.ts")
		}
	}
	if !foundAPI {
		t.Fatal("expected API route persistence model leak")
	}
	if !foundDomain {
		t.Fatal("expected domain persistence record leak")
	}
}
