package quality

import (
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/codeguard/checks/support"
	"github.com/devr-tools/codeguard/codeguard/core"
)

func Evaluate(cfg core.Config, scope core.ScanScope) core.SectionResult {
	if !cfg.Checks.Quality {
		return core.SectionResult{
			Name:   "Code Quality",
			Status: core.StatusSkip,
			Note:   "Disabled in config.",
		}
	}

	result := core.SectionResult{
		Name:   "Code Quality",
		Status: core.StatusPass,
		Note:   "Go source formatting, parse, and maintainability checks passed.",
	}
	rules := resolveQualityRules(cfg.Checks.QualityRules)
	for _, target := range cfg.Targets {
		if !support.IsGoTarget(target) {
			continue
		}

		files, err := support.ScopedGoFiles(target.Path, scope)
		if err != nil {
			result.Status = core.StatusFail
			result.Note = "Unable to enumerate Go files."
			result.Findings = append(result.Findings, core.Finding{
				Path:     filepath.ToSlash(target.Path),
				Message:  err.Error(),
				Severity: core.SeverityError,
			})
			continue
		}
		if len(files) == 0 {
			result.Findings = append(result.Findings, core.Finding{
				Path:     filepath.ToSlash(target.Path),
				Message:  "No Go files found for quality checks",
				Severity: core.SeverityInfo,
			})
			continue
		}

		for _, file := range files {
			findings, severity := checkFile(file, rules)
			if len(findings) == 0 {
				continue
			}
			recordQualityFindings(&result, findings, severity)
		}
	}

	if result.Status == core.StatusPass {
		result.Findings = append(result.Findings, core.Finding{
			Path:     filepath.ToSlash("codeguard"),
			Message:  "All scanned Go files are parseable and gofmt-clean",
			Severity: core.SeverityInfo,
		})
	}
	return result
}

func checkFile(path string, rules qualityRules) ([]core.Finding, core.Severity) {
	source, file, fset, findings, severity := runFormatChecks(path)
	if severity == core.SeverityError {
		return findings, severity
	}

	findings = append(findings, maintainabilityFindings(path, source, file, fset, rules)...)
	findings = append(findings, dependencyDirectionFindings(path, file)...)

	if len(findings) == 0 {
		return nil, core.SeverityInfo
	}
	return findings, core.SeverityWarn
}

func recordQualityFindings(result *core.SectionResult, findings []core.Finding, severity core.Severity) {
	result.Findings = append(result.Findings, findings...)
	switch severity {
	case core.SeverityError:
		result.Status = core.StatusFail
		result.Note = "Go source quality issues detected."
	case core.SeverityWarn:
		if result.Status == core.StatusPass {
			result.Status = core.StatusWarn
		}
		if !strings.Contains(result.Note, "maintainability") && !strings.Contains(result.Note, "dependency") {
			result.Note = "Maintainability thresholds exceeded."
		}
	}
}
