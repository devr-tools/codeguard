package cpptooling

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
	"github.com/devr-tools/codeguard/internal/codeguard/cpp/compdb"
	runnersupport "github.com/devr-tools/codeguard/internal/codeguard/runner/support"
	"github.com/devr-tools/codeguard/internal/codeguard/trust"
)

const (
	defaultClangFormat = "clang-format"
	defaultCompiler    = "clang++"
	maxToolOutputBytes = 8 << 20
)

var ErrToolUnavailable = errors.New("c++ tool is unavailable")

type Issue struct {
	Path    string
	Message string
}

func CheckFormat(ctx context.Context, root string, cfg core.CPPToolingConfig, files []string) ([]Issue, error) {
	command, err := resolveCommand(root, cfg.ClangFormatCommand, defaultClangFormat, "quality_rules.cpp_tooling.clang_format_command")
	if err != nil {
		return nil, err
	}
	issues := make([]Issue, 0)
	for _, file := range files {
		if !safeRelative(file) {
			continue
		}
		output, runErr := runnersupport.RunLimitedCommand(ctx, root, maxToolOutputBytes, command, "--dry-run", "--Werror", "--", filepath.FromSlash(file))
		if runErr != nil {
			var exitErr *exec.ExitError
			if !errors.As(runErr, &exitErr) {
				return issues, fmt.Errorf("clang-format integration failed for %s: %w", filepath.ToSlash(file), runErr)
			}
			issues = append(issues, Issue{Path: filepath.ToSlash(file), Message: toolMessage("clang-format validation failed", output, runErr)})
		}
	}
	return issues, nil
}

func CheckSyntax(ctx context.Context, root string, cfg core.CPPToolingConfig) ([]Issue, error) {
	command, err := resolveCommand(root, cfg.CompilerCommand, defaultCompiler, "quality_rules.cpp_tooling.compiler_command")
	if err != nil {
		return nil, err
	}
	db, err := compdb.Load(root, cfg.CompileCommands)
	if err != nil {
		return nil, err
	}
	if len(db.Entries) == 0 {
		return nil, fmt.Errorf("compile_commands.json contains no target-local compilation entries")
	}
	issues := make([]Issue, 0)
	checked := 0
	seen := make(map[string]bool)
	for _, entry := range db.Entries {
		if !isCPPSource(entry.RelativeFile) || seen[entry.RelativeFile] {
			continue
		}
		seen[entry.RelativeFile] = true
		checked++
		args := safeSyntaxArgs(entry)
		output, runErr := runnersupport.RunLimitedCommand(ctx, root, maxToolOutputBytes, command, args...)
		if runErr != nil {
			var exitErr *exec.ExitError
			if !errors.As(runErr, &exitErr) {
				return issues, fmt.Errorf("c++ compiler integration failed for %s: %w", entry.RelativeFile, runErr)
			}
			issues = append(issues, Issue{Path: entry.RelativeFile, Message: toolMessage("C++ compiler syntax validation failed", output, runErr)})
		}
	}
	if checked == 0 {
		return nil, fmt.Errorf("compile_commands.json contains no target-local C++ source entries")
	}
	return issues, nil
}

func isCPPSource(path string) bool {
	ext := filepath.Ext(path)
	if ext == ".C" {
		return true
	}
	switch strings.ToLower(ext) {
	case ".cc", ".cp", ".cpp", ".cxx", ".c++", ".ixx", ".cppm", ".cxxm", ".ccm", ".c++m", ".mpp", ".mxx", ".ii":
		return true
	default:
		return false
	}
}

func safeSyntaxArgs(entry compdb.Entry) []string {
	args := []string{"-fsyntax-only"}
	if entry.Standard != "" {
		args = append(args, "-std="+entry.Standard)
	}
	for _, include := range entry.IncludeDirs {
		args = append(args, "-I"+include)
	}
	for _, define := range entry.Defines {
		args = append(args, "-D"+define)
	}
	for _, undefine := range entry.Undefines {
		args = append(args, "-U"+undefine)
	}
	// The compiler executable and all other flags in compile_commands.json are
	// intentionally discarded. In particular, plugins, response files, output
	// paths, wrappers, and driver escape hatches can never reach the subprocess.
	return append(args, "--", entry.File)
}

func resolveCommand(root, configured, builtIn, field string) (string, error) {
	command := strings.TrimSpace(configured)
	if command == "" {
		command = builtIn
	}
	if command != builtIn {
		if err := trust.GuardConfigCommand(field, command); err != nil {
			return "", err
		}
		if strings.ContainsRune(command, filepath.Separator) && !filepath.IsAbs(command) {
			command = filepath.Join(root, command)
		}
	}
	resolved, err := exec.LookPath(command)
	if err != nil {
		return "", fmt.Errorf("%w: %s was not found on PATH", ErrToolUnavailable, command)
	}
	return resolved, nil
}

func safeRelative(path string) bool {
	path = filepath.Clean(filepath.FromSlash(path))
	return path != "." && !filepath.IsAbs(path) && path != ".." && !strings.HasPrefix(path, ".."+string(filepath.Separator))
}

func toolMessage(prefix, output string, err error) string {
	detail := strings.TrimSpace(output)
	if detail == "" {
		detail = err.Error()
	}
	if len(detail) > 2000 {
		detail = detail[:2000] + "..."
	}
	return prefix + ": " + detail
}
