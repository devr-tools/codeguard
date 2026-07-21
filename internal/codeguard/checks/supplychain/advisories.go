package supplychain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

const advisoryCacheSchemaVersion = 1

// advisoryCache is deliberately a small, stable offline interchange format.
// Updating it belongs to a separate approved process; scan execution only reads
// this file and never makes a network request.
type advisoryCache struct {
	SchemaVersion int              `json:"schema_version"`
	GeneratedAt   string           `json:"generated_at"`
	Source        string           `json:"source,omitempty"`
	Advisories    []cachedAdvisory `json:"advisories"`
}

type cachedAdvisory struct {
	ID               string   `json:"id"`
	Ecosystem        string   `json:"ecosystem"`
	Package          string   `json:"package"`
	AffectedVersions []string `json:"affected_versions"`
	FixedVersion     string   `json:"fixed_version,omitempty"`
	URL              string   `json:"url,omitempty"`
}

func vulnerableDependencyFindings(env support.Context, target core.TargetConfig, manifest core.SupplyChainManifest) []core.Finding {
	rules := env.Config.Checks.SupplyChainRules
	if rules.DetectVulnerabilities == nil || !*rules.DetectVulnerabilities {
		return nil
	}
	cache, generatedAt, source, ok := loadAdvisoryCache(target.Path, rules.AdvisoryCachePath)
	if !ok {
		return nil
	}
	findings := make([]core.Finding, 0)
	for _, dep := range manifest.Dependencies {
		version, ok := concreteDependencyVersion(dep)
		if !ok {
			continue
		}
		for _, advisory := range cache.Advisories {
			if !advisoryMatchesDependency(advisory, manifest.Ecosystem, dep.Name, version) {
				continue
			}
			metadata := map[string]string{
				"advisory_id":        advisory.ID,
				"advisory_ecosystem": advisory.Ecosystem,
				"advisory_package":   advisory.Package,
				"advisory_source":    source,
				"cache_generated_at": generatedAt.UTC().Format(time.RFC3339),
				"cache_age":          advisoryCacheAge(env.ScanTime, generatedAt),
			}
			if advisory.URL != "" {
				metadata["advisory_url"] = advisory.URL
			}
			if advisory.FixedVersion != "" {
				metadata["fixed_version"] = advisory.FixedVersion
			}
			message := "dependency " + dep.Name + "@" + version + " is affected by advisory " + advisory.ID + " in the local advisory cache"
			if advisory.FixedVersion != "" {
				message += "; upgrade to " + advisory.FixedVersion + " or later"
			}
			findings = append(findings, env.NewFinding(support.FindingInput{
				RuleID: "supply_chain.vulnerable-dependency", Level: "fail", Path: manifest.Path,
				Line: dep.Line, Column: 1, Message: message, Confidence: "high", Metadata: metadata,
			}))
		}
	}
	return findings
}

func loadAdvisoryCache(targetRoot, configuredPath string) (advisoryCache, time.Time, string, bool) {
	cachePath := strings.TrimSpace(configuredPath)
	if !filepath.IsAbs(cachePath) {
		cachePath = filepath.Join(targetRoot, cachePath)
	}
	// #nosec G304 -- config validation constrains this to the configured advisory cache.
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return advisoryCache{}, time.Time{}, "", false
	}
	var cache advisoryCache
	if json.Unmarshal(data, &cache) != nil || cache.SchemaVersion != advisoryCacheSchemaVersion {
		return advisoryCache{}, time.Time{}, "", false
	}
	generatedAt, err := time.Parse(time.RFC3339, cache.GeneratedAt)
	if err != nil {
		return advisoryCache{}, time.Time{}, "", false
	}
	source := strings.TrimSpace(cache.Source)
	if source == "" {
		source = "local advisory cache"
	}
	return cache, generatedAt, source, true
}

func concreteDependencyVersion(dep core.SupplyChainDependency) (string, bool) {
	version := strings.TrimSpace(dep.Version)
	if version == "" || !dep.Pinned {
		return "", false
	}
	version = strings.TrimPrefix(version, "v")
	if parseAdvisoryVersion(version) == nil {
		return "", false
	}
	return version, true
}

func advisoryMatchesDependency(advisory cachedAdvisory, ecosystem, packageName, version string) bool {
	if !strings.EqualFold(strings.TrimSpace(advisory.Ecosystem), strings.TrimSpace(ecosystem)) || !strings.EqualFold(strings.TrimSpace(advisory.Package), strings.TrimSpace(packageName)) {
		return false
	}
	for _, affected := range advisory.AffectedVersions {
		if advisoryVersionInRange(version, affected) {
			return true
		}
	}
	return false
}

func advisoryCacheAge(scanTime, generatedAt time.Time) string {
	if scanTime.IsZero() {
		scanTime = time.Now().UTC()
	}
	age := scanTime.Sub(generatedAt)
	if age < 0 {
		return "0s"
	}
	return age.Round(time.Hour).String()
}

// advisoryVersionInRange supports the compact comparator form used in the
// cache (for example, ">=1.0.0, <1.2.0" or "=1.2.3").
func advisoryVersionInRange(version, expression string) bool {
	comparators := strings.Split(strings.TrimSpace(expression), ",")
	if len(comparators) == 0 || strings.TrimSpace(expression) == "" {
		return false
	}
	actual := parseAdvisoryVersion(version)
	if actual == nil {
		return false
	}
	for _, comparator := range comparators {
		part := strings.TrimSpace(comparator)
		op := "="
		for _, candidate := range []string{"<=", ">=", "<", ">", "="} {
			if strings.HasPrefix(part, candidate) {
				op, part = candidate, strings.TrimSpace(strings.TrimPrefix(part, candidate))
				break
			}
		}
		expected := parseAdvisoryVersion(strings.TrimPrefix(part, "v"))
		if expected == nil || !advisoryVersionComparisonMatches(compareAdvisoryVersions(actual, expected), op) {
			return false
		}
	}
	return true
}

func advisoryVersionComparisonMatches(comparison int, operator string) bool {
	switch operator {
	case "<":
		return comparison < 0
	case "<=":
		return comparison <= 0
	case ">":
		return comparison > 0
	case ">=":
		return comparison >= 0
	default:
		return comparison == 0
	}
}

func parseAdvisoryVersion(raw string) []int {
	raw = strings.TrimSpace(strings.SplitN(raw, "-", 2)[0])
	parts := strings.Split(raw, ".")
	if len(parts) == 0 || len(parts) > 4 {
		return nil
	}
	version := make([]int, 3)
	for index, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return nil
		}
		if index < len(version) {
			version[index] = value
		}
	}
	return version
}

func compareAdvisoryVersions(left, right []int) int {
	for index := 0; index < len(left) && index < len(right); index++ {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	return 0
}
