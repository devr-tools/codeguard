package quality

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/checks/support"
	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/trust"
)

const (
	rustToolchainDeadCodeRuleID      = "quality.dead-code.toolchain"
	rustToolchainCommandOutputLimit  = 1 << 20
	rustToolchainArtifactCode        = "artifact_unreachable"
	rustToolchainArtifactReportLimit = 8 << 20
)

type rustToolchainIssue struct {
	path    string
	line    int
	column  int
	code    string
	message string
}

type rustToolchainArtifactCandidate struct {
	path                string
	line                int
	column              int
	name                string
	moduleSymbol        string
	symbol              string
	legacyMangled       string
	legacyModuleMangled string
}

type rustToolchainArtifactEvidence struct {
	reports []string
	lines   []string
}

type rustToolchainCrateRoot struct {
	rel       string
	crateName string
}

type cargoCompilerMessage struct {
	Reason  string `json:"reason"`
	Message struct {
		Level   string `json:"level"`
		Message string `json:"message"`
		Code    *struct {
			Code string `json:"code"`
		} `json:"code"`
		Spans []struct {
			FileName    string `json:"file_name"`
			LineStart   int    `json:"line_start"`
			ColumnStart int    `json:"column_start"`
			IsPrimary   bool   `json:"is_primary"`
		} `json:"spans"`
	} `json:"message"`
}

func rustToolchainDeadCodeFindings(ctx context.Context, env support.Context, target core.TargetConfig) []core.Finding {
	cfg := env.Config.Checks.QualityRules.DeadCode
	if !toolchainDeadCodeEnabled(cfg) || !isRustQualityTarget(target) {
		return nil
	}
	level := toolchainDeadCodeLevel(cfg)
	if err := trust.GuardConfigCommand("quality_rules.dead_code", "cargo check"); err != nil {
		return []core.Finding{rustToolchainDeadCodeDiagnostic(env, level, err.Error())}
	}
	issues, err := rustToolchainDeadCodeIssues(ctx, env, target, cfg)
	findings := make([]core.Finding, 0, len(issues)+1)
	for _, issue := range issues {
		if rustToolchainIssueIgnored(env, target, cfg, issue) {
			continue
		}
		message := fmt.Sprintf("Rust compiler reported %s: %s", issue.code, issue.message)
		metadata := map[string]string{
			"language": "rust",
			"lint":     issue.code,
		}
		if issue.code == rustToolchainArtifactCode {
			message = issue.message
			metadata["evidence"] = "artifact-symbol-report"
		}
		findings = append(findings, env.NewFinding(support.FindingInput{
			RuleID:     rustToolchainDeadCodeRuleID,
			Level:      level,
			Path:       issue.path,
			Line:       issue.line,
			Column:     issue.column,
			Confidence: core.ConfidenceHigh,
			Message:    message,
			Metadata:   metadata,
		}))
	}
	if err != nil {
		findings = append(findings, rustToolchainDeadCodeDiagnostic(env, level, err.Error()))
	}
	return findings
}

func toolchainDeadCodeEnabled(cfg core.QualityDeadCodeConfig) bool {
	if cfg.Enabled == nil || !*cfg.Enabled {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Mode)) {
	case "", "toolchain":
		return true
	default:
		return false
	}
}

func toolchainDeadCodeLevel(cfg core.QualityDeadCodeConfig) string {
	switch strings.ToLower(strings.TrimSpace(cfg.Level)) {
	case "fail":
		return "fail"
	default:
		return "warn"
	}
}

func rustToolchainDeadCodeDiagnostic(env support.Context, level string, message string) core.Finding {
	return env.NewFinding(support.FindingInput{
		RuleID:  rustToolchainDeadCodeRuleID,
		Level:   level,
		Message: "Rust toolchain dead-code analysis failed: " + message,
	})
}

func isRustQualityTarget(target core.TargetConfig) bool {
	switch support.NormalizedLanguage(target.Language) {
	case "rust":
		return true
	default:
		return false
	}
}

func rustToolchainDeadCodeIssues(ctx context.Context, env support.Context, target core.TargetConfig, cfg core.QualityDeadCodeConfig) ([]rustToolchainIssue, error) {
	targetDir := target.Path
	tmpDir, err := os.MkdirTemp("", "codeguard-rust-dead-code-*")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	crates := rustToolchainCrates(cfg.Rust.Crates)
	allIssues := make([]rustToolchainIssue, 0)
	for _, crate := range crates {
		issues, crateErr := rustToolchainDeadCodeIssuesForCrate(ctx, targetDir, tmpDir, crate, cfg)
		allIssues = append(allIssues, issues...)
		if crateErr != nil {
			return allIssues, crateErr
		}
	}
	artifactIssues, err := rustToolchainArtifactIssues(env, target, cfg)
	if err != nil {
		return allIssues, err
	}
	allIssues = appendRustToolchainUniqueIssues(allIssues, artifactIssues...)
	sort.Slice(allIssues, func(i, j int) bool {
		if allIssues[i].path != allIssues[j].path {
			return allIssues[i].path < allIssues[j].path
		}
		if allIssues[i].line != allIssues[j].line {
			return allIssues[i].line < allIssues[j].line
		}
		return allIssues[i].column < allIssues[j].column
	})
	return allIssues, nil
}

func appendRustToolchainUniqueIssues(base []rustToolchainIssue, more ...rustToolchainIssue) []rustToolchainIssue {
	seen := map[string]struct{}{}
	for _, issue := range base {
		seen[fmt.Sprintf("%s:%d:%d", issue.path, issue.line, issue.column)] = struct{}{}
	}
	for _, issue := range more {
		key := fmt.Sprintf("%s:%d:%d", issue.path, issue.line, issue.column)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		base = append(base, issue)
	}
	return base
}

func rustToolchainCrates(configured []string) []string {
	if len(configured) == 0 {
		return []string{""}
	}
	crates := make([]string, 0, len(configured))
	for _, crate := range configured {
		trimmed := filepath.ToSlash(strings.TrimSpace(crate))
		if trimmed != "" {
			crates = append(crates, trimmed)
		}
	}
	return crates
}

func rustToolchainDeadCodeIssuesForCrate(ctx context.Context, targetDir string, tmpDir string, crate string, cfg core.QualityDeadCodeConfig) ([]rustToolchainIssue, error) {
	args := []string{"check", "--message-format=json"}
	if manifest := rustToolchainManifestPath(crate); manifest != "" {
		args = append(args, "--manifest-path", manifest)
	}
	if cfg.IncludeTests != nil && *cfg.IncludeTests {
		args = append(args, "--all-targets")
	}
	for _, pkg := range cfg.Rust.Packages {
		args = append(args, "--package", strings.TrimSpace(pkg))
	}

	output, err := runRustToolchainCommand(ctx, targetDir, rustToolchainCommandEnv(tmpDir), "cargo", args...)
	issues := parseCargoDeadCodeDiagnostics(output, targetDir)
	if err != nil && len(issues) == 0 {
		if strings.TrimSpace(output) != "" {
			return nil, fmt.Errorf("%w: %s", err, firstNonEmptyLine(output))
		}
		return nil, err
	}
	return issues, nil
}

func rustToolchainManifestPath(crate string) string {
	crate = filepath.ToSlash(strings.TrimSpace(crate))
	if crate == "" || crate == "." {
		return ""
	}
	if strings.HasSuffix(crate, ".toml") {
		return crate
	}
	return filepath.ToSlash(filepath.Join(crate, "Cargo.toml"))
}

func rustToolchainArtifactIssues(env support.Context, target core.TargetConfig, cfg core.QualityDeadCodeConfig) ([]rustToolchainIssue, error) {
	if len(cfg.Rust.Reports) == 0 {
		return nil, nil
	}
	evidence, err := rustToolchainArtifactEvidenceFromReports(target.Path, cfg.Rust.Reports)
	if err != nil {
		return nil, err
	}
	candidates := rustToolchainArtifactCandidates(env, target, cfg)
	issues := make([]rustToolchainIssue, 0)
	for _, candidate := range candidates {
		if !evidence.HasModuleEvidence(candidate) || evidence.HasSymbol(candidate) {
			continue
		}
		issues = append(issues, rustToolchainIssue{
			path:   candidate.path,
			line:   candidate.line,
			column: candidate.column,
			code:   rustToolchainArtifactCode,
			message: fmt.Sprintf(
				"private Rust function %q is absent from configured artifact symbol reports with same-module evidence (%s)",
				candidate.name,
				strings.Join(evidence.reports, ", "),
			),
		})
	}
	return issues, nil
}

func rustToolchainArtifactEvidenceFromReports(targetDir string, reports []string) (rustToolchainArtifactEvidence, error) {
	evidence := rustToolchainArtifactEvidence{
		reports: make([]string, 0, len(reports)),
		lines:   make([]string, 0),
	}
	for _, report := range reports {
		abs, rel, err := rustToolchainReportPath(targetDir, report)
		if err != nil {
			return evidence, err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return evidence, fmt.Errorf("read Rust artifact report %q: %w", rel, err)
		}
		if info.Size() > rustToolchainArtifactReportLimit {
			return evidence, fmt.Errorf("rust artifact report %q exceeds %d bytes", rel, rustToolchainArtifactReportLimit)
		}
		data, err := os.ReadFile(abs) //nolint:gosec // report path is validated target-relative before reading.
		if err != nil {
			return evidence, fmt.Errorf("read Rust artifact report %q: %w", rel, err)
		}
		evidence.reports = append(evidence.reports, rel)
		for _, line := range strings.Split(string(data), "\n") {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				evidence.lines = append(evidence.lines, trimmed)
			}
		}
	}
	return evidence, nil
}

func rustToolchainReportPath(targetDir string, report string) (string, string, error) {
	trimmed := filepath.ToSlash(strings.TrimSpace(report))
	if trimmed == "" {
		return "", "", fmt.Errorf("quality_rules.dead_code.rust.reports contains a blank path")
	}
	clean := filepath.Clean(filepath.FromSlash(trimmed))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("quality_rules.dead_code.rust.reports path %q must be relative to the target", report)
	}
	rel := filepath.ToSlash(clean)
	return filepath.Join(targetDir, clean), rel, nil
}

func (e rustToolchainArtifactEvidence) HasModuleEvidence(candidate rustToolchainArtifactCandidate) bool {
	if candidate.moduleSymbol == "" {
		return false
	}
	modulePrefix := candidate.moduleSymbol + "::"
	for _, line := range e.lines {
		if strings.Contains(line, modulePrefix) || (candidate.legacyModuleMangled != "" && strings.Contains(line, candidate.legacyModuleMangled)) {
			return true
		}
	}
	return false
}

func (e rustToolchainArtifactEvidence) HasSymbol(candidate rustToolchainArtifactCandidate) bool {
	if candidate.symbol == "" {
		return false
	}
	for _, line := range e.lines {
		if rustToolchainLineContainsSymbol(line, candidate.symbol) || (candidate.legacyMangled != "" && strings.Contains(line, candidate.legacyMangled)) {
			return true
		}
	}
	return false
}

func rustToolchainArtifactCandidates(env support.Context, target core.TargetConfig, cfg core.QualityDeadCodeConfig) []rustToolchainArtifactCandidate {
	if env.VisitTargetFiles == nil {
		return nil
	}
	roots := rustToolchainCrateRoots(env, target, cfg)
	candidates := make([]rustToolchainArtifactCandidate, 0)
	env.VisitTargetFiles(target, func(rel string) bool {
		rel = filepath.ToSlash(rel)
		if !strings.HasSuffix(rel, ".rs") || rustToolchainArtifactPathIgnored(rel, cfg) {
			return false
		}
		return true
	}, func(rel string, data []byte) {
		if rustToolchainGeneratedFile(data) || rustToolchainProcMacroFile(data) || rustToolchainPluginRegistryFile(data) {
			return
		}
		candidates = append(candidates, rustToolchainArtifactCandidatesForFile(filepath.ToSlash(rel), data, roots)...)
	})
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].path != candidates[j].path {
			return candidates[i].path < candidates[j].path
		}
		if candidates[i].line != candidates[j].line {
			return candidates[i].line < candidates[j].line
		}
		return candidates[i].name < candidates[j].name
	})
	return candidates
}

func rustToolchainArtifactPathIgnored(rel string, cfg core.QualityDeadCodeConfig) bool {
	for _, pattern := range cfg.Rust.IgnorePaths {
		if support.PathMatchesPattern(pattern, rel) {
			return true
		}
	}
	if cfg.IncludeTests == nil || !*cfg.IncludeTests {
		switch {
		case strings.HasPrefix(rel, "tests/"), strings.HasPrefix(rel, "benches/"), strings.HasPrefix(rel, "examples/"):
			return true
		case strings.Contains(rel, "/tests/"), strings.Contains(rel, "/benches/"), strings.Contains(rel, "/examples/"):
			return true
		case strings.HasSuffix(rel, "_test.rs"):
			return true
		}
	}
	return strings.Contains(rel, "/generated/") ||
		strings.HasPrefix(rel, "generated/") ||
		strings.Contains(rel, "/vendor/") ||
		strings.HasPrefix(rel, "vendor/") ||
		strings.Contains(rel, "/target/") ||
		strings.HasPrefix(rel, "target/")
}

func rustToolchainCrateRoots(env support.Context, target core.TargetConfig, cfg core.QualityDeadCodeConfig) []rustToolchainCrateRoot {
	crates := rustToolchainCrates(cfg.Rust.Crates)
	roots := make([]rustToolchainCrateRoot, 0, len(crates))
	for _, crate := range crates {
		rel := rustToolchainCrateRootRel(crate)
		crateName := rustToolchainCrateName(env, target, rel)
		roots = append(roots, rustToolchainCrateRoot{rel: rel, crateName: crateName})
	}
	sort.Slice(roots, func(i, j int) bool { return len(roots[i].rel) > len(roots[j].rel) })
	return roots
}

func rustToolchainCrateRootRel(crate string) string {
	crate = filepath.ToSlash(strings.TrimSpace(crate))
	switch {
	case crate == "", crate == ".":
		return ""
	case strings.HasSuffix(crate, ".toml"):
		return filepath.ToSlash(filepath.Dir(crate))
	default:
		return strings.Trim(filepath.ToSlash(crate), "/")
	}
}

func rustToolchainCrateName(env support.Context, target core.TargetConfig, rootRel string) string {
	manifest := filepath.ToSlash(filepath.Join(rootRel, "Cargo.toml"))
	if rootRel == "" {
		manifest = "Cargo.toml"
	}
	if data, ok := rustToolchainReadTargetFile(env, target, manifest); ok {
		if name := rustToolchainPackageNameFromManifest(data); name != "" {
			return name
		}
	}
	if rootRel != "" {
		return rustToolchainNormalizeCrateName(filepath.Base(rootRel))
	}
	return rustToolchainNormalizeCrateName(filepath.Base(target.Path))
}

func rustToolchainPackageNameFromManifest(data []byte) string {
	inPackage := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "[package]":
			inPackage = true
			continue
		case strings.HasPrefix(trimmed, "[") && trimmed != "[package]":
			inPackage = false
		}
		if !inPackage || !strings.HasPrefix(trimmed, "name") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		return rustToolchainNormalizeCrateName(strings.Trim(strings.TrimSpace(parts[1]), `"`))
	}
	return ""
}

func rustToolchainNormalizeCrateName(name string) string {
	return strings.ReplaceAll(strings.TrimSpace(name), "-", "_")
}

func rustToolchainArtifactCandidatesForFile(rel string, data []byte, roots []rustToolchainCrateRoot) []rustToolchainArtifactCandidate {
	_, module, ok := rustToolchainModuleSymbolForFile(rel, roots)
	if !ok {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	candidates := make([]rustToolchainArtifactCandidate, 0)
	depth := 0
	attrs := make([]string, 0)
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if depth == 0 && strings.HasPrefix(trimmed, "#[") {
			attrs = append(attrs, trimmed)
			continue
		}
		if depth == 0 {
			if name, column, ok := rustToolchainPrivateFunctionOnLine(line); ok && rustToolchainArtifactFunctionEligible(data, idx+1, strings.Join(attrs, "\n")) {
				symbol := module + "::" + name
				candidates = append(candidates, rustToolchainArtifactCandidate{
					path:                rel,
					line:                idx + 1,
					column:              column,
					name:                name,
					moduleSymbol:        module,
					symbol:              symbol,
					legacyMangled:       rustToolchainLegacyMangledPrefix(append(strings.Split(module, "::"), name)),
					legacyModuleMangled: rustToolchainLegacyMangledPrefix(strings.Split(module, "::")),
				})
			}
			if trimmed != "" && !strings.HasPrefix(trimmed, "//") {
				attrs = attrs[:0]
			}
		}
		depth += strings.Count(line, "{")
		depth -= strings.Count(line, "}")
		if depth < 0 {
			depth = 0
		}
	}
	return candidates
}

func rustToolchainModuleSymbolForFile(rel string, roots []rustToolchainCrateRoot) (rustToolchainCrateRoot, string, bool) {
	for _, root := range roots {
		fileRel := rel
		if root.rel != "" {
			if rel != root.rel && !strings.HasPrefix(rel, root.rel+"/") {
				continue
			}
			fileRel = strings.TrimPrefix(rel, root.rel+"/")
		}
		if !strings.HasPrefix(fileRel, "src/") {
			continue
		}
		srcRel := strings.TrimPrefix(fileRel, "src/")
		if srcRel == "lib.rs" || srcRel == "main.rs" || strings.HasPrefix(srcRel, "bin/") {
			if strings.HasPrefix(srcRel, "bin/") {
				return root, "", false
			}
			return root, root.crateName, true
		}
		var moduleRel string
		switch {
		case strings.HasSuffix(srcRel, "/mod.rs"):
			moduleRel = strings.TrimSuffix(srcRel, "/mod.rs")
		case strings.HasSuffix(srcRel, ".rs"):
			moduleRel = strings.TrimSuffix(srcRel, ".rs")
		default:
			return root, "", false
		}
		if moduleRel == "" {
			return root, root.crateName, true
		}
		parts := []string{root.crateName}
		for _, part := range strings.Split(moduleRel, "/") {
			if part != "" {
				parts = append(parts, part)
			}
		}
		return root, strings.Join(parts, "::"), true
	}
	return rustToolchainCrateRoot{}, "", false
}

func rustToolchainPrivateFunctionOnLine(line string) (string, int, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "pub ") || strings.HasPrefix(trimmed, "pub(") {
		return "", 0, false
	}
	rest := trimmed
	for {
		changed := false
		for _, prefix := range []string{"async ", "unsafe ", "const "} {
			if strings.HasPrefix(rest, prefix) {
				rest = strings.TrimSpace(strings.TrimPrefix(rest, prefix))
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	if strings.HasPrefix(rest, "extern ") || !strings.HasPrefix(rest, "fn ") {
		return "", 0, false
	}
	rest = strings.TrimSpace(strings.TrimPrefix(rest, "fn "))
	if rest == "" || !rustToolchainIsIdentifierStart(rest[0]) {
		return "", 0, false
	}
	end := 1
	for end < len(rest) && rustToolchainIsIdentifierChar(rest[end]) {
		end++
	}
	name := rest[:end]
	afterName := strings.TrimSpace(rest[end:])
	if name == "main" || strings.HasPrefix(afterName, "<") {
		return "", 0, false
	}
	return name, strings.Index(line, "fn "+name) + 4, true
}

func rustToolchainArtifactFunctionEligible(data []byte, line int, attrs string) bool {
	attrText := strings.ToLower(attrs)
	if strings.Contains(attrText, "#[test") ||
		strings.Contains(attrText, "#[bench") ||
		strings.Contains(attrText, "#[cfg") ||
		strings.Contains(attrText, "#[inline") ||
		strings.Contains(attrText, "#[no_mangle]") ||
		strings.Contains(attrText, "#[export_name") ||
		strings.Contains(attrText, "#[used]") ||
		strings.Contains(attrText, "#[proc_macro") {
		return false
	}
	return !rustToolchainLineLooksPublic(data, line) &&
		!strings.Contains(rustToolchainSourceLine(data, line), `extern "C"`) &&
		!rustToolchainLineIsCfgTestOnly(data, line) &&
		!rustToolchainLineInTraitImpl(data, line)
}

func rustToolchainLegacyMangledPrefix(parts []string) string {
	var b strings.Builder
	b.WriteString("_ZN")
	for _, part := range parts {
		if part == "" {
			continue
		}
		_, _ = fmt.Fprintf(&b, "%d%s", len(part), part)
	}
	return b.String()
}

func rustToolchainLineContainsSymbol(line string, symbol string) bool {
	offset := 0
	for {
		idx := strings.Index(line[offset:], symbol)
		if idx < 0 {
			return false
		}
		start := offset + idx
		if start > 0 && rustToolchainIsIdentifierChar(line[start-1]) {
			offset = start + len(symbol)
			continue
		}
		end := start + len(symbol)
		if end < len(line) && rustToolchainIsIdentifierChar(line[end]) {
			offset = end
			continue
		}
		return true
	}
}

func rustToolchainIsIdentifierStart(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_'
}

func rustToolchainIsIdentifierChar(b byte) bool {
	return rustToolchainIsIdentifierStart(b) || (b >= '0' && b <= '9')
}

func rustToolchainCommandEnv(tmpDir string) []string {
	env := make([]string, 0, len(os.Environ())+2)
	var rustFlags string
	for _, item := range os.Environ() {
		switch {
		case strings.HasPrefix(item, "CARGO_TARGET_DIR="):
			continue
		case strings.HasPrefix(item, "RUSTFLAGS="):
			rustFlags = strings.TrimPrefix(item, "RUSTFLAGS=")
			continue
		default:
			env = append(env, item)
		}
	}
	flags := strings.TrimSpace(rustFlags + " -Ddead_code -Dunreachable_code")
	env = append(env,
		"CARGO_TARGET_DIR="+filepath.Join(tmpDir, "target"),
		"RUSTFLAGS="+flags,
	)
	return env
}

func runRustToolchainCommand(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // fixed Cargo invocation; config values are validated args, not commands
	cmd.Dir = dir
	cmd.Env = env
	var buf bytes.Buffer
	limited := newRustToolchainLimitedWriter(&buf, rustToolchainCommandOutputLimit)
	cmd.Stdout = limited
	cmd.Stderr = limited
	err := cmd.Run()
	if limited.Truncated() {
		return "", fmt.Errorf("%s output exceeded %d bytes", name, rustToolchainCommandOutputLimit)
	}
	return buf.String(), err
}

func parseCargoDeadCodeDiagnostics(output string, targetDir string) []rustToolchainIssue {
	seen := map[string]struct{}{}
	issues := make([]rustToolchainIssue, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		var msg cargoCompilerMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil || msg.Reason != "compiler-message" || msg.Message.Code == nil {
			continue
		}
		code := msg.Message.Code.Code
		if code != "dead_code" && code != "unreachable_code" {
			continue
		}
		span, ok := primaryCargoSpan(msg)
		if !ok {
			continue
		}
		rel, ok := rustToolchainRelativePath(targetDir, span.FileName)
		if !ok {
			continue
		}
		key := fmt.Sprintf("%s:%d:%d:%s", rel, span.LineStart, span.ColumnStart, code)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		issues = append(issues, rustToolchainIssue{
			path:    rel,
			line:    span.LineStart,
			column:  span.ColumnStart,
			code:    code,
			message: msg.Message.Message,
		})
	}
	return issues
}

func primaryCargoSpan(msg cargoCompilerMessage) (struct {
	FileName    string `json:"file_name"`
	LineStart   int    `json:"line_start"`
	ColumnStart int    `json:"column_start"`
	IsPrimary   bool   `json:"is_primary"`
}, bool) {
	for _, span := range msg.Message.Spans {
		if span.IsPrimary && span.FileName != "" && span.LineStart > 0 {
			return span, true
		}
	}
	return struct {
		FileName    string `json:"file_name"`
		LineStart   int    `json:"line_start"`
		ColumnStart int    `json:"column_start"`
		IsPrimary   bool   `json:"is_primary"`
	}{}, false
}

func rustToolchainRelativePath(targetDir string, fileName string) (string, bool) {
	if strings.HasPrefix(fileName, "<") {
		return "", false
	}
	clean := filepath.Clean(filepath.FromSlash(fileName))
	if !filepath.IsAbs(clean) {
		return filepath.ToSlash(clean), true
	}
	rel, err := filepath.Rel(targetDir, clean)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func rustToolchainIssueIgnored(env support.Context, target core.TargetConfig, cfg core.QualityDeadCodeConfig, issue rustToolchainIssue) bool {
	if !strings.HasSuffix(issue.path, ".rs") {
		return true
	}
	if rustToolchainArtifactPathIgnored(issue.path, cfg) {
		return true
	}
	data, ok := rustToolchainReadTargetFile(env, target, issue.path)
	if !ok {
		return false
	}
	if rustToolchainGeneratedFile(data) || rustToolchainProcMacroFile(data) || rustToolchainPluginRegistryFile(data) {
		return true
	}
	return rustToolchainLineLooksPublic(data, issue.line) ||
		rustToolchainLineHasRuntimeExport(data, issue.line) ||
		rustToolchainLineIsCfgTestOnly(data, issue.line) ||
		rustToolchainLineInTraitImpl(data, issue.line)
}

func rustToolchainReadTargetFile(env support.Context, target core.TargetConfig, rel string) ([]byte, bool) {
	if env.ReadTargetFile != nil {
		data, err := env.ReadTargetFile(target, rel)
		if err == nil {
			return data, true
		}
	}
	var found []byte
	env.VisitTargetFiles(target, func(path string) bool { return filepath.ToSlash(path) == filepath.ToSlash(rel) }, func(_ string, data []byte) {
		found = data
	})
	return found, found != nil
}

func rustToolchainGeneratedFile(data []byte) bool {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "generated") && strings.Contains(strings.ToLower(trimmed), "do not edit") {
			return true
		}
		if !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "/*") && !strings.HasPrefix(trimmed, "*") {
			return false
		}
	}
	return false
}

func rustToolchainProcMacroFile(data []byte) bool {
	source := string(data)
	return strings.Contains(source, "extern crate proc_macro") ||
		strings.Contains(source, "use proc_macro::") ||
		strings.Contains(source, "#[proc_macro") ||
		strings.Contains(source, "#[proc_macro_attribute") ||
		strings.Contains(source, "#[proc_macro_derive")
}

func rustToolchainPluginRegistryFile(data []byte) bool {
	source := string(data)
	return strings.Contains(source, "inventory::submit!") ||
		strings.Contains(source, "linkme::distributed_slice") ||
		strings.Contains(source, "#[distributed_slice]") ||
		strings.Contains(source, "ctor::ctor")
}

func rustToolchainLineLooksPublic(data []byte, line int) bool {
	current := rustToolchainSourceLine(data, line)
	return strings.Contains(current, "pub fn ") ||
		strings.Contains(current, "pub(crate) fn ") ||
		strings.Contains(current, "pub(super) fn ") ||
		strings.Contains(current, "pub struct ") ||
		strings.Contains(current, "pub enum ") ||
		strings.Contains(current, "pub trait ") ||
		strings.Contains(current, "pub type ") ||
		strings.Contains(current, "pub const ") ||
		strings.Contains(current, "pub static ")
}

func rustToolchainLineHasRuntimeExport(data []byte, line int) bool {
	window := rustToolchainSourceWindow(data, line, 4)
	return strings.Contains(window, "#[no_mangle]") ||
		strings.Contains(window, "#[export_name") ||
		strings.Contains(window, "#[used]") ||
		strings.Contains(window, "extern \"C\"")
}

func rustToolchainLineIsCfgTestOnly(data []byte, line int) bool {
	window := rustToolchainSourceWindow(data, line, 6)
	return strings.Contains(window, "#[cfg(test)]") || strings.Contains(window, "cfg_attr(test")
}

func rustToolchainLineInTraitImpl(data []byte, line int) bool {
	lines := strings.Split(string(data), "\n")
	if line <= 0 || line > len(lines) {
		return false
	}
	depth := 0
	implDepths := make([]int, 0)
	for idx, rawLine := range lines {
		if idx+1 == line {
			current := strings.TrimSpace(rawLine)
			return strings.Contains(current, "fn ") && len(implDepths) > 0
		}
		trimmed := strings.TrimSpace(rawLine)
		if strings.HasPrefix(trimmed, "impl ") && strings.Contains(trimmed, " for ") && strings.Contains(trimmed, "{") {
			implDepths = append(implDepths, depth+strings.Count(rawLine, "{"))
		}
		depth += strings.Count(rawLine, "{")
		depth -= strings.Count(rawLine, "}")
		if depth < 0 {
			depth = 0
		}
		for len(implDepths) > 0 && depth < implDepths[len(implDepths)-1] {
			implDepths = implDepths[:len(implDepths)-1]
		}
	}
	return false
}

func rustToolchainSourceLine(data []byte, line int) string {
	lines := strings.Split(string(data), "\n")
	if line <= 0 || line > len(lines) {
		return ""
	}
	return strings.TrimSpace(lines[line-1])
}

func rustToolchainSourceWindow(data []byte, line int, before int) string {
	lines := strings.Split(string(data), "\n")
	if line <= 0 || line > len(lines) {
		return ""
	}
	start := line - before - 1
	if start < 0 {
		start = 0
	}
	return strings.Join(lines[start:line], "\n")
}

func firstNonEmptyLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return strings.TrimSpace(output)
}

type rustToolchainLimitedWriter struct {
	buf       *bytes.Buffer
	limit     int
	truncated bool
}

func newRustToolchainLimitedWriter(buf *bytes.Buffer, limit int) *rustToolchainLimitedWriter {
	return &rustToolchainLimitedWriter{buf: buf, limit: limit}
}

func (w *rustToolchainLimitedWriter) Write(p []byte) (int, error) {
	if w.truncated {
		return len(p), nil
	}
	remaining := w.limit - w.buf.Len()
	if remaining <= 0 {
		w.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = w.buf.Write(p[:remaining])
		w.truncated = true
		return len(p), nil
	}
	_, _ = w.buf.Write(p)
	return len(p), nil
}

func (w *rustToolchainLimitedWriter) Truncated() bool {
	return w.truncated
}
