package codeguard_test

import (
	"strings"
	"testing"

	"github.com/devr-tools/codeguard/pkg/codeguard"
)

func TestValidateQualityDeadCodeConfig(t *testing.T) {
	tests := []struct {
		name     string
		deadCode codeguard.QualityDeadCodeConfig
		want     string
	}{
		{
			name:     "invalid mode",
			deadCode: codeguard.QualityDeadCodeConfig{Mode: "compiler"},
			want:     "dead_code.mode",
		},
		{
			name:     "invalid level",
			deadCode: codeguard.QualityDeadCodeConfig{Level: "error"},
			want:     "dead_code.level",
		},
		{
			name: "blank package pattern",
			deadCode: codeguard.QualityDeadCodeConfig{Go: codeguard.GoDeadCodeToolchainConfig{
				Packages: []string{" "},
			}},
			want: "dead_code.go.packages[0]",
		},
		{
			name: "flag-shaped entrypoint",
			deadCode: codeguard.QualityDeadCodeConfig{Go: codeguard.GoDeadCodeToolchainConfig{
				Entrypoints: []string{"-race"},
			}},
			want: "dead_code.go.entrypoints[0]",
		},
		{
			name: "escaping ignore path",
			deadCode: codeguard.QualityDeadCodeConfig{Go: codeguard.GoDeadCodeToolchainConfig{
				IgnorePaths: []string{"../generated/**"},
			}},
			want: "dead_code.go.ignore_paths[0]",
		},
		{
			name: "absolute rust crate",
			deadCode: codeguard.QualityDeadCodeConfig{Rust: codeguard.RustDeadCodeConfig{
				Crates: []string{"/tmp/crate"},
			}},
			want: "dead_code.rust.crates[0]",
		},
		{
			name: "flag-shaped rust package",
			deadCode: codeguard.QualityDeadCodeConfig{Rust: codeguard.RustDeadCodeConfig{
				Packages: []string{"--workspace"},
			}},
			want: "dead_code.rust.packages[0]",
		},
		{
			name: "escaping rust report",
			deadCode: codeguard.QualityDeadCodeConfig{Rust: codeguard.RustDeadCodeConfig{
				Reports: []string{"../target/rust-symbols.txt"},
			}},
			want: "dead_code.rust.reports[0]",
		},
		{
			name: "escaping c++ compile database",
			deadCode: codeguard.QualityDeadCodeConfig{CPP: codeguard.CPPDeadCodeConfig{
				CompileCommands: "../compile_commands.json",
			}},
			want: "dead_code.cpp.compile_commands",
		},
		{
			name: "blank c++ report",
			deadCode: codeguard.QualityDeadCodeConfig{CPP: codeguard.CPPDeadCodeConfig{
				Reports: []string{""},
			}},
			want: "dead_code.cpp.reports[0]",
		},
		{
			name: "blank python report",
			deadCode: codeguard.QualityDeadCodeConfig{Python: codeguard.PythonDeadCodeConfig{
				Reports: []string{""},
			}},
			want: "dead_code.python.reports[0]",
		},
		{
			name: "flag-shaped typescript project",
			deadCode: codeguard.QualityDeadCodeConfig{TypeScript: codeguard.ScriptDeadCodeConfig{
				Projects: []string{"--project"},
			}},
			want: "dead_code.typescript.projects[0]",
		},
		{
			name: "escaping javascript report",
			deadCode: codeguard.QualityDeadCodeConfig{JavaScript: codeguard.ScriptDeadCodeConfig{
				Reports: []string{"../meta.json"},
			}},
			want: "dead_code.javascript.reports[0]",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := codeguard.ExampleConfig()
			cfg.Checks.QualityRules.DeadCode = tt.deadCode
			err := codeguard.ValidateConfig(cfg)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("ValidateConfig error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestValidateQualityDeadCodeConfigAcceptsToolchainGoSettings(t *testing.T) {
	enabled := true
	linker := true
	cfg := codeguard.ExampleConfig()
	cfg.Checks.QualityRules.DeadCode = codeguard.QualityDeadCodeConfig{
		Enabled: &enabled,
		Mode:    "toolchain",
		Level:   "warn",
		Go: codeguard.GoDeadCodeToolchainConfig{
			Packages:    []string{"./..."},
			Entrypoints: []string{"./cmd/app"},
			IgnorePaths: []string{"internal/generated/**"},
			Linker:      &linker,
		},
		Rust: codeguard.RustDeadCodeConfig{
			Crates:      []string{"crates/app"},
			Packages:    []string{"app"},
			Entrypoints: []string{"src/main.rs"},
			Reports:     []string{".codeguard/rust-symbols.txt"},
			IgnorePaths: []string{"target/**"},
		},
		CPP: codeguard.CPPDeadCodeConfig{
			CompileCommands: "compile_commands.json",
			Entrypoints:     []string{"src/main.cpp"},
			Reports:         []string{"build/app.map", "build/nm.txt"},
			IgnorePaths:     []string{"build/**"},
		},
		Python: codeguard.PythonDeadCodeConfig{
			Modules:     []string{"src/app"},
			Entrypoints: []string{"src/app/__main__.py"},
			Reports:     []string{".codeguard/vulture.json"},
			IgnorePaths: []string{"migrations/**"},
		},
		TypeScript: codeguard.ScriptDeadCodeConfig{
			Projects:    []string{"tsconfig.json"},
			Entrypoints: []string{"src/main.ts"},
			Reports:     []string{"dist/meta.json"},
			IgnorePaths: []string{"src/generated/**"},
		},
		JavaScript: codeguard.ScriptDeadCodeConfig{
			Projects:    []string{"jsconfig.json"},
			Entrypoints: []string{"src/main.js"},
			Reports:     []string{"dist/js-meta.json"},
			IgnorePaths: []string{"generated/**"},
		},
	}
	if err := codeguard.ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
}
