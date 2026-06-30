package contracts

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

type destructivePattern struct {
	re      *regexp.Regexp
	summary string
}

var destructivePatterns = []destructivePattern{
	{regexp.MustCompile(`(?i)\bDROP\s+TABLE\b`), "DROP TABLE"},
	{regexp.MustCompile(`(?i)\bDROP\s+COLUMN\b`), "DROP COLUMN"},
	{regexp.MustCompile(`(?i)\bTRUNCATE\b`), "TRUNCATE"},
	{regexp.MustCompile(`(?i)\bdrop_table\s*\(`), "drop_table()"},
	{regexp.MustCompile(`(?i)\bdrop_column\s*\(`), "drop_column()"},
}

var (
	alterNotNullRe  = regexp.MustCompile(`(?is)\bALTER\b.*\bNOT\s+NULL\b`)
	defaultClauseRe = regexp.MustCompile(`(?i)\bDEFAULT\b`)
)

// migrationFindings warns on destructive operations in migration files. In
// diff mode only newly added migration files are checked; in full-scan mode
// every migration file is checked.
func migrationFindings(env support.Context, target core.TargetConfig, changed []core.ChangedFile) []core.Finding {
	if !enabled(env.Config.Checks.ContractRules.MigrationDestructive) {
		return nil
	}
	include := func(rel string) bool { return isMigrationFile(env, rel) }
	if env.Mode == core.ScanModeDiff {
		added := map[string]bool{}
		for _, file := range changed {
			if file.Status == core.ChangedFileAdded {
				added[file.Path] = true
			}
		}
		include = func(rel string) bool {
			return added[filepath.ToSlash(rel)] && isMigrationFile(env, rel)
		}
	}
	return env.ScanTargetFiles(target, "contracts", include, func(file string, data []byte) []core.Finding {
		return destructiveStatementFindings(env, file, string(data))
	})
}

func isMigrationFile(env support.Context, rel string) bool {
	normalized := strings.ToLower(filepath.ToSlash(rel))
	if strings.HasSuffix(normalized, ".sql") {
		return true
	}
	prefixed := "/" + normalized
	for _, fragment := range env.Config.Checks.ContractRules.MigrationPaths {
		fragment = strings.ToLower(strings.Trim(filepath.ToSlash(strings.TrimSpace(fragment)), "/"))
		if fragment == "" {
			continue
		}
		if strings.Contains(prefixed, "/"+fragment+"/") {
			return true
		}
	}
	return false
}

// destructiveStatementFindings scans ";"-separated statements so multi-line
// SQL statements are evaluated as a unit (for the NOT NULL/DEFAULT pairing).
func destructiveStatementFindings(env support.Context, file string, content string) []core.Finding {
	findings := make([]core.Finding, 0) //nolint:prealloc // count not known up front; each statement appends a variable number
	line := 1
	for _, statement := range strings.Split(content, ";") {
		findings = append(findings, statementFindings(env, file, statement, line)...)
		line += strings.Count(statement, "\n")
	}
	return findings
}

func statementFindings(env support.Context, file string, statement string, startLine int) []core.Finding {
	findings := make([]core.Finding, 0)
	for _, pattern := range destructivePatterns {
		for _, loc := range pattern.re.FindAllStringIndex(statement, -1) {
			findings = append(findings, newMigrationFinding(env, file, lineAt(statement, startLine, loc[0]),
				fmt.Sprintf("destructive migration operation: %s", pattern.summary)))
		}
	}
	if loc := alterNotNullRe.FindStringIndex(statement); loc != nil && !defaultClauseRe.MatchString(statement) {
		findings = append(findings, newMigrationFinding(env, file, lineAt(statement, startLine, loc[0]),
			"destructive migration operation: ALTER ... NOT NULL without DEFAULT"))
	}
	return findings
}

func lineAt(statement string, startLine int, offset int) int {
	return startLine + strings.Count(statement[:offset], "\n")
}

func newMigrationFinding(env support.Context, file string, line int, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  "contracts.migration-destructive",
		Level:   "warn",
		Path:    file,
		Line:    line,
		Column:  1,
		Message: message,
	})
}
