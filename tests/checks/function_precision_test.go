package checks_test

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFunctionExcessiveParametersWarnsWithSpecificRule(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "params.go"), strings.Join([]string{
		"package sample",
		"",
		"func CreateUser(name string, email string, plan string, source string) string {",
		"\treturn name + email + plan + source",
		"}",
		"",
	}, "\n"))
	cfg := qualityPrecisionConfig(dir, "go")
	cfg.Checks.QualityRules.MaxParameters = 2

	report := runQualityPrecisionScan(t, cfg)

	assertFindingRulePresent(t, report, "Code Quality", "function.excessive-parameters")
	assertFindingLevel(t, report, "Code Quality", "function.excessive-parameters", "warn")
}

func TestFunctionMixedAbstractionLevelWarnsForInfrastructureInsideOrchestration(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "checkout.go"), strings.Join([]string{
		"package sample",
		"",
		"func Checkout(order Order) error {",
		"\tvalidateOrder(order)",
		"\trows, err := db.Query(\"select 1\")",
		"\tif err != nil {",
		"\t\treturn err",
		"\t}",
		"\tdefer rows.Close()",
		"\treturn persistOrder(order)",
		"}",
		"",
		"type Order struct{}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir, "go"))

	assertFindingRulePresent(t, report, "Code Quality", "function.mixed-abstraction-level")
	assertFindingLevel(t, report, "Code Quality", "function.mixed-abstraction-level", "warn")
}

func TestFunctionCommandQueryMixWarnsWhenQueryMutatesState(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "query.go"), strings.Join([]string{
		"package sample",
		"",
		"type Repository interface {",
		"\tFind(string) (User, error)",
		"\tSaveAudit(string) error",
		"}",
		"",
		"type User struct{}",
		"",
		"func GetUser(repo Repository, id string) (User, error) {",
		"\tif err := repo.SaveAudit(id); err != nil {",
		"\t\treturn User{}, err",
		"\t}",
		"\treturn repo.Find(id)",
		"}",
		"",
	}, "\n"))

	report := runQualityPrecisionScan(t, qualityPrecisionConfig(dir, "go"))

	assertFindingRulePresent(t, report, "Code Quality", "function.command-query-mix")
	assertFindingLevel(t, report, "Code Quality", "function.command-query-mix", "warn")
}
